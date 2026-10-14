package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Find App ID looks for an app's App ID in the Wayback Machine's saved App
// Store addresses, which contain the app's name and ID, such as
// apps.apple.com/us/app/heathcote/id1050456637. It's for apps that left the App
// Store and aren't in My apps. Only the Internet Archive is asked; never Apple.

// archiveBase is the Wayback Machine; tests point it at a fake server.
var archiveBase = "https://web.archive.org"

// archiveStores are the App Store address forms searched, newest first.
var archiveStores = []string{"apps.apple.com/us/app/", "itunes.apple.com/us/app/", "itunes.apple.com/app/"}

const (
	archiveRowLimit  = 500 // addresses per search; reaching it means the name is too short
	archiveUserAgent = "ipatool-gui (Find App ID)"
)

var (
	errArchiveBusy = errors.New("the Internet Archive isn't answering")
	errNoName      = errors.New("no letters or digits in the name")
)

// archiveHit is one App ID found in the saved addresses.
type archiveHit struct {
	ID        int64
	Slug      string // the name part of the address, like "heathcote"
	URL       string // a saved address for it, best first
	Timestamp string // when that address was saved, as the Wayback Machine writes it
	ok200     bool   // that address was saved as a page (not a redirect)

	// Filled in by pageDetails.
	Name, Developer, Price string
	Checked                bool // the saved page was read (Name may still be empty)
}

// archiveResult is what a search found. Full is true when an address list hit
// archiveRowLimit, so some matches may be missing.
type archiveResult struct {
	Hits []archiveHit
	Full bool
}

// appSlug turns an app's name into the name part of its App Store address:
// lowercase letters and digits, joined by dashes, with apostrophes dropped
// ("Tony's Game!" -> "tonys-game"). Letters with accents are kept, as Apple does.
func appSlug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r == '\'' || r == '’':
			continue
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
		default:
			dash = true
		}
	}
	return b.String()
}

var (
	appIDInURL = regexp.MustCompile(`/id(\d{6,12})`)
	// slugInURL takes the name from addresses like
	// http://itunes.apple.com:80/us/app/sixthsense/id421794646 (port and country vary).
	slugInURL = regexp.MustCompile(`/app/([^/?]+)/id\d`)
)

// searchArchive finds App IDs for name in the saved App Store addresses.
func searchArchive(ctx context.Context, client *http.Client, name string) (archiveResult, error) {
	slug := appSlug(name)
	if slug == "" {
		return archiveResult{}, errNoName
	}
	hits := map[int64]*archiveHit{}
	var result archiveResult
	for _, store := range archiveStores {
		rows, err := archiveRows(ctx, client, store+slug)
		if err != nil {
			return archiveResult{}, err
		}
		if len(rows) >= archiveRowLimit {
			result.Full = true
		}
		for _, row := range rows {
			addHit(hits, row)
		}
	}
	for _, h := range hits {
		result.Hits = append(result.Hits, *h)
	}
	// Exact name matches first, then the most recently saved.
	sort.SliceStable(result.Hits, func(i, j int) bool {
		a, b := result.Hits[i], result.Hits[j]
		if (a.Slug == slug) != (b.Slug == slug) {
			return a.Slug == slug
		}
		return a.Timestamp > b.Timestamp
	})
	return result, nil
}

// newArchiveClient returns the HTTP client for Find App ID. It follows
// redirects only within the Wayback Machine: a saved redirect could otherwise
// lead to Apple's live store, and Find App ID never asks Apple.
func newArchiveClient() *http.Client {
	return &http.Client{
		Timeout: 60 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			base, err := url.Parse(archiveBase)
			if err != nil || req.URL.Host != base.Host {
				return errLeftArchive
			}
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			return nil
		},
	}
}

var errLeftArchive = errors.New("a saved page pointed away from the Internet Archive")

// archiveRow is one line of the Wayback Machine's address list.
type archiveRow struct {
	timestamp, original, status string
}

// addHit adds one saved address to hits, keeping for each ID the best address
// to read later: one saved with status 200, then the newest.
func addHit(hits map[int64]*archiveHit, row archiveRow) {
	m := appIDInURL.FindStringSubmatch(row.original)
	if m == nil {
		return
	}
	id, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return
	}
	slug := ""
	if s := slugInURL.FindStringSubmatch(row.original); s != nil {
		slug = s[1]
	}
	row200 := row.status == "200"
	h, ok := hits[id]
	if !ok {
		h = &archiveHit{ID: id, Slug: slug}
		hits[id] = h
	}
	if !ok || (row200 && !h.ok200) || (row200 == h.ok200 && row.timestamp > h.Timestamp) {
		h.URL, h.Timestamp, h.ok200 = row.original, row.timestamp, row200
	}
	if h.Slug == "" {
		h.Slug = slug
	}
}

// archiveRows asks the Wayback Machine for saved addresses starting with prefix.
func archiveRows(ctx context.Context, client *http.Client, prefix string) ([]archiveRow, error) {
	u := fmt.Sprintf("%s/cdx/search/cdx?url=%s&matchType=prefix&fl=timestamp,original,statuscode&collapse=urlkey&limit=%d",
		archiveBase, url.QueryEscape(prefix), archiveRowLimit)
	body, err := archiveGet(ctx, client, u)
	if err != nil {
		return nil, err
	}
	var rows []archiveRow
	scanner := bufio.NewScanner(strings.NewReader(body))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 3 {
			rows = append(rows, archiveRow{fields[0], fields[1], fields[2]})
		}
	}
	return rows, nil
}

// archiveGet fetches u, trying once more when the Wayback Machine is busy.
func archiveGet(ctx context.Context, client *http.Client, u string) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(2 * time.Second):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", archiveUserAgent)
		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			lastErr = err
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		switch {
		case err != nil:
			lastErr = err
		case resp.StatusCode == http.StatusOK:
			return string(body), nil
		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
		default:
			return "", fmt.Errorf("the Internet Archive answered HTTP %d", resp.StatusCode)
		}
	}
	return "", fmt.Errorf("%w (%v)", errArchiveBusy, lastErr)
}

// pageDetails reads the saved store page of h and fills in its name, developer
// and price, as far as the page shows them.
func pageDetails(ctx context.Context, client *http.Client, h *archiveHit) error {
	page, err := archiveGet(ctx, client, fmt.Sprintf("%s/web/%sid_/%s", archiveBase, h.Timestamp, h.URL))
	if err != nil {
		return err
	}
	h.Name, h.Developer, h.Price = parseStorePage(page)
	h.Checked = true
	return nil
}

var (
	pageTitle     = regexp.MustCompile(`(?is)<title>(.*?)</title>`)
	pageH1        = regexp.MustCompile(`(?is)<h1[^>]*itemprop="name"[^>]*>(.*?)</h1>`)
	pageAuthor    = regexp.MustCompile(`"author":\{[^}]*?"name":"((?:[^"\\]|\\.)*)"`)
	pageBy        = regexp.MustCompile(`(?is)<h2>\s*By\s+(.*?)</h2>`)
	pageIdentity  = regexp.MustCompile(`(?is)app-header__identity"[^>]*>\s*<a[^>]*>(.*?)</a>`)
	pagePriceItem = regexp.MustCompile(`(?is)app-header__list__item--price"[^>]*>(.*?)<`)
	pagePriceDiv  = regexp.MustCompile(`(?is)class="price"[^>]*>(.*?)<`)
	titleSuffix   = regexp.MustCompile(`(?i)\s+(?:for [^<]*?\s+)?on the (?:iTunes )?App Store$`)
	spaces        = regexp.MustCompile(`\s+`)
	tags          = regexp.MustCompile(`<[^>]*>`)
)

// parseStorePage reads an App Store page as the Wayback Machine saved it: the
// current apps.apple.com pages (since 2019) and the older itunes.apple.com ones.
func parseStorePage(page string) (name, developer, price string) {
	clean := func(s string) string {
		s = html.UnescapeString(tags.ReplaceAllString(s, " "))
		s = strings.ReplaceAll(s, "\u00a0", " ") // Apple writes "App Store" with a no-break space
		s = strings.Trim(s, "‎‏ ")
		return strings.TrimSpace(spaces.ReplaceAllString(s, " "))
	}
	first := func(res ...*regexp.Regexp) string {
		for _, re := range res {
			if m := re.FindStringSubmatch(page); m != nil {
				if v := clean(m[1]); v != "" {
					return v
				}
			}
		}
		return ""
	}
	name = first(pageH1)
	if name == "" {
		if m := pageTitle.FindStringSubmatch(page); m != nil {
			name = clean(titleSuffix.ReplaceAllString(clean(m[1]), ""))
		}
	}
	developer = first(pageAuthor, pageBy, pageIdentity)
	developer = strings.ReplaceAll(developer, `\"`, `"`)
	price = first(pagePriceItem, pagePriceDiv)
	return name, developer, price
}

// savedYear is the year an address was saved, from its timestamp.
func (h archiveHit) savedYear() string {
	if len(h.Timestamp) >= 4 {
		return h.Timestamp[:4]
	}
	return ""
}

// line is how a result reads in the list.
func (h archiveHit) line() string {
	name := h.Name
	if name == "" {
		name = h.Slug
	}
	parts := []string{name}
	if h.Developer != "" {
		parts = append(parts, "Developer: "+h.Developer)
	}
	if h.Price != "" {
		parts = append(parts, "Price: "+h.Price)
	}
	parts = append(parts, fmt.Sprintf("App ID: %d", h.ID))
	switch {
	case !h.Checked && h.Name == "":
		parts = append(parts, "checking the saved page")
	case h.Checked && h.Name == "" && h.Developer == "":
		parts = append(parts, "details not available")
	}
	if y := h.savedYear(); y != "" {
		parts = append(parts, "saved "+y)
	}
	return strings.Join(parts, "; ")
}
