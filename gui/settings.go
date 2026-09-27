package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

var platforms = []string{"iphone", "ipad", "appletv", "visionos", "macos"}

// Settings are what the GUI remembers between sessions, stored as JSON in
// %APPDATA%\ipatool-gui\settings.json.
type Settings struct {
	Email      string `json:"email"`
	Output     string `json:"output"`
	Passphrase string `json:"passphrase"`
}

func settingsPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "ipatool-gui", "settings.json")
}

func loadSettings() Settings {
	var s Settings
	if data, err := os.ReadFile(settingsPath()); err == nil {
		_ = json.Unmarshal(data, &s)
	}
	return s
}

func saveSettings(s Settings) {
	path := settingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	if data, err := json.MarshalIndent(s, "", "  "); err == nil {
		_ = os.WriteFile(path, data, 0o600)
	}
}

// friendlyError translates ipatool's error messages into plain advice.
func friendlyError(text string) string {
	lowered := strings.ToLower(strings.TrimSpace(text))
	switch {
	case strings.Contains(lowered, "could not be found in the keyring"):
		return "You are not signed in. Log in on the Account page first (Ctrl+1)."
	case strings.Contains(lowered, "integrity check failed"):
		return "The keychain passphrase is wrong. Use the same one you chose when you first logged in."
	case strings.Contains(lowered, "keychain passphrase is required"):
		return "Enter a keychain passphrase on the Account page."
	case strings.Contains(lowered, "license is required"):
		return "Your account doesn't own this app. Check \"Get a free license if needed\" (free apps only)."
	case strings.Contains(lowered, "password token is expired"):
		return "Your session expired. Log in again on the Account page."
	case lowered == "invalid response":
		return "Apple didn't send a download for this app. Check that the platform is right, or try again later."
	case strings.Contains(lowered, "app not found"):
		return "Apple couldn't find that app. It may have been removed from the App Store in your country."
	}
	return strings.TrimSpace(text)
}

// App is one app from a search or the owned-apps list.
type App struct {
	ID        int64
	BundleID  string
	Name      string
	Version   string
	Price     float64
	Platforms []string
}
