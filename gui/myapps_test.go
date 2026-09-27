package main

import (
	"fmt"
	"testing"
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
		if got := filterApps(all, tt.platform, tt.query); len(got) != tt.want {
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
	if got, _, _ := pageOf(apps, 2, 25); got[0].ID != 26 {
		t.Errorf("page 2 starts with app %d, want 26", got[0].ID)
	}
	if got, current, pages := pageOf(nil, 1, 25); len(got) != 0 || current != 1 || pages != 1 {
		t.Errorf("empty list: %d apps, page %d of %d; want 0, 1 of 1", len(got), current, pages)
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
