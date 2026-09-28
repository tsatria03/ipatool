package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestItunesFileName(t *testing.T) {
	tests := []struct{ name, version, ext, want string }{
		{"Dice Only", "1.9", ".ipa", "Dice Only 1.9.ipa"},
		{"Loopy HD", "1.6.33", "ipa", "Loopy HD 1.6.33.ipa"},
		{"Pages", "14.2", ".pkg", "Pages 14.2.pkg"},
		{"Battle Prime: Modern War Game", "1.0", ".ipa", "Battle Prime - Modern War Game 1.0.ipa"},
		{"!iM: iKlavka, classic", "2.0", ".ipa", "!iM - iKlavka, classic 2.0.ipa"},
		{`AC/DC "Live" <HD>?*|`, "1", ".ipa", "AC-DC 'Live' (HD)- 1.ipa"},
		{"  Spaced   out \t name  ", "3", ".ipa", "Spaced out name 3.ipa"},
		{"Ends with dots...", "2.1", ".ipa", "Ends with dots 2.1.ipa"},
		{"No version", "", ".ipa", "No version.ipa"},
		{"CON", "", ".ipa", "CON app.ipa"},
		{"腾讯微博", "6.1.2", ".ipa", "腾讯微博 6.1.2.ipa"},
		{"", "1.0", ".ipa", ""}, // unknown name: keep ipatool's
	}
	for _, tt := range tests {
		if got := itunesFileName(tt.name, tt.version, tt.ext); got != tt.want {
			t.Errorf("itunesFileName(%q, %q) = %q, want %q", tt.name, tt.version, got, tt.want)
		}
	}

	long := "101 Free Alerts - Change your text tone, new email alert, new voicemail alert and more " +
		"and even more words that keep going well past any sensible length for a file name on Windows today"
	got := itunesFileName(long, "1.0.0", ".ipa")
	name := strings.TrimSuffix(got, " 1.0.0.ipa")
	if utf8.RuneCountInString(name) > maxNameLength || strings.HasSuffix(name, " ") || !strings.HasPrefix(got, "101 Free Alerts") {
		t.Errorf("long name shortened badly: %q (%d characters)", got, utf8.RuneCountInString(name))
	}
	if !strings.HasSuffix(got, " 1.0.0.ipa") {
		t.Errorf("long name lost its version: %q", got)
	}
}

func TestRenameLikeITunes(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}

	// A fresh download gets the iTunes-style name.
	got, err := renameLikeITunes(write("com.dice.only_1_1.9.ipa", "new"), "Dice Only", "1.9")
	if err != nil || got != filepath.Join(dir, "Dice Only 1.9.ipa") {
		t.Fatalf("got %q, %v", got, err)
	}

	// Downloading it again replaces the existing file.
	got, err = renameLikeITunes(write("com.dice.only_1_1.9.ipa", "newer"), "Dice Only", "1.9")
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(got); string(data) != "newer" {
		t.Errorf("existing file not replaced: %q", data)
	}
	if _, err := os.Stat(filepath.Join(dir, "com.dice.only_1_1.9.ipa")); !os.IsNotExist(err) {
		t.Error("the ipatool-named file should be gone")
	}

	// Unknown name: the file keeps its name.
	keep := write("com.x_2_1.0.ipa", "x")
	if got, err := renameLikeITunes(keep, "", "1.0"); err != nil || got != keep {
		t.Errorf("unknown name: got %q, %v", got, err)
	}
}
