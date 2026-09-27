package main

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// My apps keeps the whole purchase history in memory for the session (fetching
// it takes about 12 seconds, because Apple always sends the full history), and
// searches, filters and pages through it locally, which is instant.

// filterApps returns the apps available on platform ("" for all platforms)
// whose name or bundle ID contains query (case-insensitive; "" matches all).
func filterApps(all []App, platform, query string) []App {
	query = strings.ToLower(strings.TrimSpace(query))
	var out []App
	for _, app := range all {
		if platform != "" && !contains(app.Platforms, platform) {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(app.Name), query) &&
			!strings.Contains(strings.ToLower(app.BundleID), query) {
			continue
		}
		out = append(out, app)
	}
	return out
}

// sortChoices are the "Sort by" options, in the order they appear; index 0 is
// the default (the order the engine returns).
var sortChoices = []string{
	"Newest purchase first",
	"Oldest purchase first",
	"Name, A to Z",
	"Name, Z to A",
	"Bundle ID, A to Z",
	"Bundle ID, Z to A",
	"Newest to the App Store first",
	"Oldest to the App Store first",
}

// sortApps sorts apps in place by the given sortChoices index. App IDs grow as
// apps are added to the App Store, so sorting by ID approximates App Store age.
// Ties fall back to bundle ID and then ID, so the order never shuffles.
func sortApps(apps []App, choice int) {
	lower := strings.ToLower
	tie := func(a, b App) int {
		if c := strings.Compare(lower(a.BundleID), lower(b.BundleID)); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	}
	var compare func(a, b App) int
	switch choice {
	case 1:
		compare = func(a, b App) int { return a.PurchaseDate.Compare(b.PurchaseDate) }
	case 2:
		compare = func(a, b App) int { return strings.Compare(lower(a.Name), lower(b.Name)) }
	case 3:
		compare = func(a, b App) int { return strings.Compare(lower(b.Name), lower(a.Name)) }
	case 4:
		compare = func(a, b App) int { return strings.Compare(lower(a.BundleID), lower(b.BundleID)) }
	case 5:
		compare = func(a, b App) int { return strings.Compare(lower(b.BundleID), lower(a.BundleID)) }
	case 6:
		compare = func(a, b App) int { return cmp.Compare(b.ID, a.ID) }
	case 7:
		compare = func(a, b App) int { return cmp.Compare(a.ID, b.ID) }
	default:
		compare = func(a, b App) int { return b.PurchaseDate.Compare(a.PurchaseDate) }
	}
	slices.SortStableFunc(apps, func(a, b App) int {
		if c := compare(a, b); c != 0 {
			return c
		}
		return tie(a, b)
	})
}

// pageOf returns one page of apps, with the page number clamped to the valid
// range and the total number of pages (at least 1).
func pageOf(apps []App, page, perPage int) (pageApps []App, current, pages int) {
	pages = max(1, (len(apps)+perPage-1)/perPage)
	current = min(max(1, page), pages)
	start := (current - 1) * perPage
	end := min(start+perPage, len(apps))
	if start >= len(apps) {
		return nil, current, pages
	}
	return apps[start:end], current, pages
}

// pageInfoText describes the current page for the Page field, for example
// "Page 2 of 143, 3554 apps" or "Page 1 of 2, 30 of 3554 apps match".
func pageInfoText(current, pages, matching, total int, narrowed bool) string {
	if narrowed {
		return fmt.Sprintf("Page %d of %d, %d of %d apps match", current, pages, matching, total)
	}
	return fmt.Sprintf("Page %d of %d, %d apps", current, pages, total)
}
