package main

import "testing"

func TestFindByPrefix(t *testing.T) {
	names := []string{"Tango", "Evernote", "TapTapSee", "tides", "YouTube", "腾讯微博", "Tap Tap Revenge"}
	at := func(row int) string { return names[row] }
	tests := []struct {
		prefix string
		start  int
		wrap   bool
		want   int
	}{
		{"t", 0, true, 0},     // first match, including the start row
		{"t", 1, true, 2},     // the same letter again moves on (the list starts after the current row)
		{"T", 3, true, 3},     // capitals ignored ("tides")
		{"t", 4, true, 6},     // later rows
		{"t", 7, true, 0},     // past the end: back to the top
		{"tap", 0, true, 2},   // several quick letters form a prefix
		{"tap t", 3, true, 6}, // spaces are part of the prefix
		{"腾", 0, true, 5},     // any language
		{"q", 0, true, -1},    // no match
		{"e", 2, false, -1},   // no wrap: only rows after start
		{"", 0, true, -1},     // nothing typed
	}
	for _, tt := range tests {
		if got := findByPrefix(len(names), at, tt.prefix, tt.start, tt.wrap); got != tt.want {
			t.Errorf("findByPrefix(%q, start %d, wrap %v) = %d, want %d", tt.prefix, tt.start, tt.wrap, got, tt.want)
		}
	}
	if got := findByPrefix(0, at, "t", 0, true); got != -1 {
		t.Errorf("empty list: %d, want -1", got)
	}
}
