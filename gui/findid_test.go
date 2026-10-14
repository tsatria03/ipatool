package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestAppSlug(t *testing.T) {
	tests := map[string]string{
		"Heathcote":                "heathcote",
		"Tony's Game!":             "tonys-game",
		"Tony’s Game":              "tonys-game",
		"Blinded Chef":             "blinded-chef",
		"  Battle Prime: Modern  ": "battle-prime-modern",
		"Ready to Roll - RPG Dice": "ready-to-roll-rpg-dice",
		"Café & Co.":               "café-co",
		"!!!":                      "",
	}
	for name, want := range tests {
		if got := appSlug(name); got != want {
			t.Errorf("appSlug(%q) = %q, want %q", name, got, want)
		}
	}
}

// Real answers from the Wayback Machine's address search (2026-10-14).
var fakeCDX = map[string]string{
	"apps.apple.com/us/app/heathcote": `20200330170919 https://apps.apple.com/us/app/heathcote-high-school-enews/id889683640 200
20190820173011 https://apps.apple.com/us/app/heathcote/id1050456637 200
20190718023801 https://apps.apple.com/us/app/heathcote/id1050456637?ign-mpt=uo%3D8 200
`,
	"itunes.apple.com/us/app/heathcote": `20160701042254 https://itunes.apple.com/us/app/heathcote/id1050456637?mt=8&uo=8&at=11l4LS 301
20160411051647 https://itunes.apple.com/us/app/heathcote/id1050456637?mt=8 200
`,
	"itunes.apple.com/us/app/sixthsense": `20150501143731 https://itunes.apple.com/us/app/sixthsense-all-new-3d-sound/id580417319?mt=8&ign-mpt=uo%3D4&at=11l4LS 301
20250816113227 https://itunes.apple.com/us/app/sixthsense-all-new-3d-sound/id580417319?mt=8 301
20210609061038 https://itunes.apple.com/us/app/sixthsense-all-new-3d.. 301
20110308051259 http://itunes.apple.com:80/us/app/sixthsense/id421794646? 200
`,
}

// Parts of real saved store pages: apps.apple.com (2019) and itunes.apple.com (2016, 2011).
const (
	pageNew = `<html><head><title>
&#x200E;Heathcote on the App&nbsp;Store</title>
<script type="application/ld+json">{"@context":"http://schema.org","@type":"SoftwareApplication","name":"Heathcote",
"author":{"@type":"Person","name":"Fabulae Obscurae, Inc.","url":"https://apps.apple.com/us/developer/fabulae-obscurae-inc/id1050456636"},
"offers":{"@type":"Offer","price":0,"priceCurrency":"USD"}}</script></head><body>
<h1 class="product-header__title app-header__title">Heathcote <span class="badge badge--product-title">4+</span></h1>
<h2 class="product-header__identity app-header__identity"> <a class="link" dir="auto" href="https://apps.apple.com/us/developer/fabulae-obscurae-inc/id1050456636">Fabulae Obscurae, Inc.</a></h2>
<li class="inline-list__item inline-list__item--bulleted app-header__list__item--price">Free</li></body></html>`
	pageOld = `<html><head><title>Heathcote on the App Store</title></head><body>
<div class="left"> <h1 itemprop="name">Heathcote</h1> <h2>By Fabulae Obscurae, Inc.</h2></div>
<div class="price">Free</div></body></html>`
	page2011 = `<html><head><title>SixthSense for iPhone, iPod touch, and iPad on the iTunes App Store</title></head>
<body><h2>By TechXperts</h2><div class="price">$0.99</div></body></html>`
)

// fakeArchive serves fakeCDX and the pages, failing the first busyTimes requests.
func fakeArchive(t *testing.T, busyTimes int32) (*httptest.Server, *int32) {
	t.Helper()
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&requests, 1)
		if n <= busyTimes {
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		if r.Header.Get("User-Agent") != archiveUserAgent {
			t.Errorf("User-Agent = %q", r.Header.Get("User-Agent"))
		}
		switch {
		case r.URL.Path == "/cdx/search/cdx":
			w.Write([]byte(fakeCDX[r.URL.Query().Get("url")]))
		case strings.Contains(r.URL.Path, "apps.apple.com/us/app/heathcote/id1050456637"):
			w.Write([]byte(pageNew))
		case strings.Contains(r.URL.Path, "id421794646"):
			w.Write([]byte(page2011))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	old := archiveBase
	archiveBase = server.URL
	t.Cleanup(func() { archiveBase = old })
	return server, &requests
}

func TestSearchArchive(t *testing.T) {
	server, requests := fakeArchive(t, 0)
	result, err := searchArchive(context.Background(), server.Client(), "Heathcote")
	if err != nil {
		t.Fatal(err)
	}
	if *requests != 3 {
		t.Errorf("made %d requests, want one per address form (3)", *requests)
	}
	if len(result.Hits) != 2 || result.Full {
		t.Fatalf("hits = %+v, full = %v", result.Hits, result.Full)
	}
	// The exact name comes first, with its newest address saved as a page.
	h := result.Hits[0]
	if h.ID != 1050456637 || h.Slug != "heathcote" || h.Timestamp != "20190820173011" ||
		h.URL != "https://apps.apple.com/us/app/heathcote/id1050456637" {
		t.Errorf("first hit = %+v", h)
	}
	if result.Hits[1].ID != 889683640 {
		t.Errorf("second hit = %+v", result.Hits[1])
	}

	if err := pageDetails(context.Background(), server.Client(), &h); err != nil {
		t.Fatal(err)
	}
	want := "Heathcote; Developer: Fabulae Obscurae, Inc.; Price: Free; App ID: 1050456637; saved 2019"
	if got := h.line(); got != want {
		t.Errorf("line = %q, want %q", got, want)
	}
}

func TestSearchArchiveSeveralApps(t *testing.T) {
	server, _ := fakeArchive(t, 0)
	result, err := searchArchive(context.Background(), server.Client(), "SixthSense")
	if err != nil {
		t.Fatal(err)
	}
	ids := map[int64]archiveHit{}
	for _, h := range result.Hits {
		ids[h.ID] = h
	}
	if len(ids) != 2 {
		t.Fatalf("hits = %+v, want two different apps", result.Hits)
	}
	// Only redirects were saved for 580417319: the newest one is used.
	if h := ids[580417319]; h.Timestamp != "20250816113227" || h.ok200 {
		t.Errorf("580417319 = %+v", h)
	}
	if h := ids[421794646]; !h.ok200 || h.Slug != "sixthsense" {
		t.Errorf("421794646 = %+v", h)
	}
	if result.Hits[0].ID != 421794646 {
		t.Errorf("the exact name should come first: %+v", result.Hits[0])
	}
}

func TestSearchArchiveBusy(t *testing.T) {
	server, requests := fakeArchive(t, 1) // the first request fails, the retry works
	result, err := searchArchive(context.Background(), server.Client(), "Heathcote")
	if err != nil || len(result.Hits) != 2 || *requests != 4 {
		t.Errorf("hits %d, requests %d, err %v", len(result.Hits), *requests, err)
	}
	server, _ = fakeArchive(t, 100) // always busy
	if _, err := searchArchive(context.Background(), server.Client(), "Heathcote"); !errors.Is(err, errArchiveBusy) {
		t.Errorf("err = %v, want errArchiveBusy", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := searchArchive(ctx, server.Client(), "Heathcote"); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled: err = %v", err)
	}
	if _, err := searchArchive(context.Background(), server.Client(), "!!"); !errors.Is(err, errNoName) {
		t.Errorf("no name: err = %v", err)
	}
}

func TestParseStorePage(t *testing.T) {
	tests := []struct {
		page, name, developer, price string
	}{
		{pageNew, "Heathcote", "Fabulae Obscurae, Inc.", "Free"},
		{pageOld, "Heathcote", "Fabulae Obscurae, Inc.", "Free"},
		{page2011, "SixthSense", "TechXperts", "$0.99"},
		{"<html>not a store page</html>", "", "", ""},
	}
	for _, tt := range tests {
		name, developer, price := parseStorePage(tt.page)
		if name != tt.name || developer != tt.developer || price != tt.price {
			t.Errorf("parseStorePage = %q, %q, %q; want %q, %q, %q", name, developer, price, tt.name, tt.developer, tt.price)
		}
	}
}

func TestArchiveHitLine(t *testing.T) {
	h := archiveHit{ID: 346157113, Slug: "virtual-balloon", Timestamp: "20120415184500"}
	if got, want := h.line(), "virtual-balloon; App ID: 346157113; checking the saved page; saved 2012"; got != want {
		t.Errorf("before checking: %q, want %q", got, want)
	}
	h.Checked = true
	if got, want := h.line(), "virtual-balloon; App ID: 346157113; details not available; saved 2012"; got != want {
		t.Errorf("nothing found: %q, want %q", got, want)
	}
}

func TestArchiveClientStaysOnArchive(t *testing.T) {
	var apple int32
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&apple, 1)
	}))
	defer elsewhere.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/inside" {
			w.Write([]byte(pageOld))
			return
		}
		if r.URL.Path == "/hop" {
			http.Redirect(w, r, "/inside", http.StatusFound)
			return
		}
		http.Redirect(w, r, elsewhere.URL+"/us/app/x/id1", http.StatusMovedPermanently)
	}))
	defer server.Close()
	old := archiveBase
	archiveBase = server.URL
	defer func() { archiveBase = old }()

	client := newArchiveClient()
	if body, err := archiveGet(context.Background(), client, server.URL+"/hop"); err != nil || !strings.Contains(body, "Heathcote") {
		t.Errorf("a redirect inside the archive should be followed: %v", err)
	}
	if _, err := archiveGet(context.Background(), client, server.URL+"/away"); err == nil {
		t.Error("a redirect away from the archive should fail")
	}
	if apple != 0 {
		t.Errorf("the other site was asked %d times", apple)
	}
}
