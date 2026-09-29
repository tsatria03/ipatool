package main

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

func sampleApps(n int) []App {
	apps := make([]App, n)
	for i := range apps {
		apps[i] = App{ID: int64(i + 1), Name: fmt.Sprintf("App %d", i+1), BundleID: fmt.Sprintf("com.example.app%d", i+1),
			Platforms: []string{"iphone", "ipad"}}
	}
	return apps
}

func TestFilterApps(t *testing.T) {
	all := []App{
		{Name: "YouTube", BundleID: "com.google.ios.youtube", Platforms: []string{"iphone", "ipad"}},
		{Name: "YouTube Music", BundleID: "com.google.ios.youtubemusic", Platforms: []string{"iphone"}},
		{Name: "Enhancements For YouTube", BundleID: "com.thomasswebsites.YouTubeBuster", Platforms: []string{"macos"}},
		{Name: "Calculator", BundleID: "com.example.calc", Platforms: []string{"ipad"}},
	}
	tests := []struct {
		platform, query string
		want            int
	}{
		{"", "", 4},
		{"", "youtube", 3},       // name, any case
		{"", "  YOUTUBE ", 3},    // trimmed, case-insensitive
		{"", "youtubebuster", 1}, // bundle ID
		{"ipad", "", 2},          // platform only
		{"iphone", "youtube", 2}, // both
		{"macos", "youtube", 1},  // Mac app
		{"appletv", "", 0},       // nothing on that platform
		{"", "no such app", 0},   // no match
	}
	for _, tt := range tests {
		if got := filterApps(all, 0, tt.platform, tt.query); len(got) != tt.want {
			t.Errorf("filterApps(%q, %q) = %d apps, want %d", tt.platform, tt.query, len(got), tt.want)
		}
	}
}

func TestPageOf(t *testing.T) {
	apps := sampleApps(3554)
	tests := []struct{ page, wantLen, wantCurrent, wantPages int }{
		{1, 25, 1, 143},
		{2, 25, 2, 143},
		{143, 4, 143, 143}, // last page: 3554 - 142*25 = 4
		{500, 4, 143, 143}, // clamped to the last page
		{0, 25, 1, 143},    // clamped to the first page
	}
	for _, tt := range tests {
		got, current, pages := pageOf(apps, tt.page, 25)
		if len(got) != tt.wantLen || current != tt.wantCurrent || pages != tt.wantPages {
			t.Errorf("pageOf(page %d) = %d apps, page %d of %d; want %d apps, page %d of %d",
				tt.page, len(got), current, pages, tt.wantLen, tt.wantCurrent, tt.wantPages)
		}
	}
	// My apps' own page size: 3554 apps -> 36 pages of 100, the last with 54.
	if got, current, pages := pageOf(apps, 36, appsPerPage); len(got) != 54 || current != 36 || pages != 36 {
		t.Errorf("appsPerPage %d, last page: %d apps, page %d of %d; want 54, 36 of 36", appsPerPage, len(got), current, pages)
	}
	if got, _, _ := pageOf(apps, 2, 25); got[0].ID != 26 {
		t.Errorf("page 2 starts with app %d, want 26", got[0].ID)
	}
	if got, current, pages := pageOf(nil, 1, 25); len(got) != 0 || current != 1 || pages != 1 {
		t.Errorf("empty list: %d apps, page %d of %d; want 0, 1 of 1", len(got), current, pages)
	}
}

func TestSortApps(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2024, 1, d, 0, 0, 0, 0, time.UTC) }
	base := []App{
		{ID: 300, Name: "banana", BundleID: "com.b.app", PurchaseDate: day(2), Size: 50, Developer: "Zeta"},
		{ID: 100, Name: "Apple", BundleID: "org.a.app", PurchaseDate: day(3), Size: 200, Developer: "alpha"},
		{ID: 200, Name: "cherry", BundleID: "com.a.app", PurchaseDate: day(1), Size: 10, Developer: "Zeta"},
		{ID: 400, Name: "date", BundleID: "net.d.app", PurchaseDate: day(4)}, // left the store: no size or developer
	}
	want := map[int][]string{
		0:  {"date", "Apple", "banana", "cherry"}, // newest purchase first
		1:  {"cherry", "banana", "Apple", "date"}, // oldest purchase first
		2:  {"Apple", "banana", "cherry", "date"}, // name A-Z, ignoring capitals
		3:  {"date", "cherry", "banana", "Apple"}, // name Z-A
		4:  {"cherry", "banana", "date", "Apple"}, // bundle ID A-Z: com.a, com.b, net.d, org.a
		5:  {"Apple", "date", "banana", "cherry"}, // bundle ID Z-A
		6:  {"date", "banana", "cherry", "Apple"}, // newest to the App Store: ID 400, 300, 200, 100
		7:  {"Apple", "cherry", "banana", "date"}, // oldest to the App Store
		8:  {"Apple", "banana", "cherry", "date"}, // largest first; no size last
		9:  {"cherry", "banana", "Apple", "date"}, // smallest first; no size still last
		10: {"Apple", "banana", "cherry", "date"}, // developer A-Z (alpha, Zeta, Zeta: by name); none last
		11: {"banana", "cherry", "Apple", "date"}, // developer Z-A, same developer still by name A-Z; none last
	}
	if len(want) != len(sortChoices) {
		t.Fatalf("test covers %d choices, sortChoices has %d", len(want), len(sortChoices))
	}
	for choice, names := range want {
		apps := slices.Clone(base)
		sortApps(apps, choice)
		for i, app := range apps {
			if app.Name != names[i] {
				t.Errorf("%s: position %d is %q, want %q", sortChoices[choice], i+1, app.Name, names[i])
			}
		}
	}
	// Ties (same purchase date) fall back to bundle ID, so the order is fixed.
	tied := []App{{ID: 1, BundleID: "z"}, {ID: 2, BundleID: "a"}}
	sortApps(tied, 0)
	if tied[0].BundleID != "a" {
		t.Errorf("tie order: got %q first, want %q", tied[0].BundleID, "a")
	}
}

func TestFormatSize(t *testing.T) {
	tests := map[int64]string{
		0:             "",
		300:           "1 KB",
		850_000:       "850 KB",
		999_400:       "999 KB",
		1_000_000:     "1 MB",
		392695808:     "393 MB",
		999_400_000:   "999 MB",
		1_234_000_000: "1.2 GB",
		4_000_000_000: "4.0 GB",
	}
	for bytes, want := range tests {
		if got := formatSize(bytes); got != want {
			t.Errorf("formatSize(%d) = %q, want %q", bytes, got, want)
		}
	}
}

func TestAppColumns(t *testing.T) {
	titles := func(m *appModel) string {
		var out []string
		for _, c := range appColumns(m) {
			out = append(out, c.Title)
		}
		return strings.Join(out, ", ")
	}

	// Global search: Name, Version, Developer, Price, Size.
	store := &appModel{store: true, apps: []App{
		{Name: "YouTube", Version: "20.1", Developer: "Google", Price: 0, Size: 392695808},
		{Name: "Paid", Price: 1.99},
	}}
	if got := titles(store); got != "Name, Version, Developer, Price, Size" {
		t.Errorf("Global search columns = %s", got)
	}
	for col, want := range []string{"YouTube", "20.1", "Google", "Free", "393 MB"} {
		if got := store.Value(0, col); got != want {
			t.Errorf("Global search column %d = %q, want %q", col, got, want)
		}
	}
	if got := store.Value(1, 3); got != "1.99" {
		t.Errorf("Global search price = %q, want 1.99", got)
	}

	// My apps: Name, Version, Developer, Purchase date, Size.
	bought := time.Date(2026, 9, 5, 12, 0, 0, 0, time.Local)
	owned := &appModel{apps: []App{
		{Name: "Game-board", Version: "1.0.5", PurchaseDate: bought, Size: 217000000, Developer: "Muamel Aljanahi"},
		{Name: "Dusk", Version: "1.2", PurchaseDate: bought}, // left the store: no developer or size
	}}
	if got := titles(owned); got != "Name, Version, Developer, Purchase date, Size" {
		t.Errorf("My apps columns = %s", got)
	}
	for col, want := range []string{"Game-board", "1.0.5", "Muamel Aljanahi", "September 5, 2026", "217 MB"} {
		if got := owned.Value(0, col); got != want {
			t.Errorf("My apps column %d = %q, want %q", col, got, want)
		}
	}
	if got := owned.Value(1, 2).(string) + owned.Value(1, 4).(string); got != "" {
		t.Errorf("an app that left the store shows developer or size %q", got)
	}
}

func TestExportLines(t *testing.T) {
	apps := []App{
		{Name: "Game-board", BundleID: "net.muamal.gameboard", Version: "1.0.5", Platforms: []string{"iphone"}, ID: 6786885206},
		{Name: "No version", BundleID: "com.example.x", Platforms: []string{"iphone", "ipad"}, ID: 7},
	}
	want := "Game-board; Bundle ID: net.muamal.gameboard; Version: 1.0.5; Platforms: iphone; App ID: 6786885206\r\n" +
		"No version; Bundle ID: com.example.x; Platforms: iphone, ipad; App ID: 7\r\n"
	if got := exportLines(apps); got != want {
		t.Errorf("exportLines:\n got %q\nwant %q", got, want)
	}
}

func TestExportJSON(t *testing.T) {
	bought := time.Date(2026, 9, 20, 18, 42, 11, 0, time.UTC)
	apps := []App{
		{Name: "腾讯微博 & more", BundleID: "com.tencent.WeiBo", Version: "6.1.2", Platforms: []string{"iphone"}, ID: 373357386, PurchaseDate: bought},
		{Name: "Old", BundleID: "com.example.old", ID: 5},
	}
	info := exportInfo{exported: bought, total: 3554, search: " youtube ", platform: "", sortedBy: "Name, A to Z"}
	data, err := exportJSON(info, apps)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		ExportedFrom, Exported, Search, Platform, SortedBy string
		TotalApps, ExportedApps                            int
		Apps                                               []map[string]any
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, data)
	}
	if doc.TotalApps != 3554 || doc.ExportedApps != 2 || doc.Search != "youtube" || doc.Platform != "all platforms" ||
		doc.SortedBy != "Name, A to Z" || doc.Exported != "2026-09-20T18:42:11Z" || doc.ExportedFrom != "ipatool GUI, My apps" {
		t.Errorf("heading: %+v", doc)
	}
	first := doc.Apps[0]
	if first["name"] != "腾讯微博 & more" || first["bundleID"] != "com.tencent.WeiBo" || first["id"] != float64(373357386) ||
		first["purchaseDate"] != "2026-09-20T18:42:11Z" || first["version"] != "6.1.2" {
		t.Errorf("first app: %v", first)
	}
	if _, has := doc.Apps[1]["version"]; has {
		t.Errorf("empty version should be left out: %v", doc.Apps[1])
	}
	if platforms, _ := doc.Apps[1]["platforms"].([]any); platforms == nil {
		t.Errorf("platforms should be an empty list, not missing or null: %v", doc.Apps[1])
	}
	if !strings.Contains(string(data), "腾讯微博 & more") {
		t.Errorf("names should be written as-is, not escaped:\n%s", data)
	}
}

func TestPageInfoText(t *testing.T) {
	if got := pageInfoText(2, 143, 3554, 3554, false); got != "Page 2 of 143, 3554 apps" {
		t.Errorf("got %q", got)
	}
	if got := pageInfoText(1, 2, 30, 3554, true); got != "Page 1 of 2, 30 of 3554 apps match" {
		t.Errorf("got %q", got)
	}
}
