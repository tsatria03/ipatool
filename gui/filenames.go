package main

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// Downloads are named like iTunes 12.6.5.3 named them: "App Name Version.ipa"
// (for example "Dice Only 1.9.ipa"), instead of ipatool's
// "bundle.id_appID_version.ipa".

// maxNameLength keeps file names well within Windows' limits (255 characters
// per name, and paths are often limited to 260 in total).
const maxNameLength = 150

// windowsReserved are names Windows doesn't allow for files, with any extension.
var windowsReserved = map[string]bool{"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true}

// itunesFileName returns "Name Version.ext" made safe for Windows, "Name.ext"
// when the version is unknown, or "" when the name is unknown.
func itunesFileName(name, version, ext string) string {
	name = shorten(safeFileText(name), maxNameLength)
	if name == "" {
		return ""
	}
	if windowsReserved[strings.ToUpper(name)] {
		name += " app"
	}
	if version = safeFileText(version); version != "" {
		name += " " + version
	}
	return name + "." + strings.TrimPrefix(ext, ".")
}

// safeFileText replaces the characters Windows forbids in file names and tidies
// the spacing. "Battle Prime: Modern War" becomes "Battle Prime - Modern War".
func safeFileText(s string) string {
	s = strings.NewReplacer(
		": ", " - ", ":", "-",
		"/", "-", `\`, "-", "|", "-",
		`"`, "'", "<", "(", ">", ")",
		"?", "", "*", "",
	).Replace(s)
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimRight(s, ". ") // Windows drops trailing dots and spaces
}

// shorten cuts s to at most max characters, at a word break when possible.
func shorten(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	cut := string(runes[:max])
	if i := strings.LastIndex(cut, " "); i > max/2 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " -,;.")
}

// renameLikeITunes renames a finished download to its iTunes-style name in the
// same folder, replacing a file that already has that name, and returns the new
// path. If the name is unknown, the file keeps its name.
func renameLikeITunes(path, name, version string) (string, error) {
	file := itunesFileName(name, version, filepath.Ext(path))
	if file == "" {
		return path, nil
	}
	target := filepath.Join(filepath.Dir(path), file)
	if strings.EqualFold(target, path) {
		return path, nil
	}
	// Windows won't rename over an existing file, so remove it first; the new
	// download is complete at this point, so nothing is lost if this fails.
	if _, err := os.Stat(target); err == nil {
		if err := os.Remove(target); err != nil {
			return path, err
		}
	}
	if err := os.Rename(path, target); err != nil {
		return path, err
	}
	return target, nil
}
