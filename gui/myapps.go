package main

import (
	"fmt"
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
