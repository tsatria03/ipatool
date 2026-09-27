package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
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

// App is one app from a search or the owned-apps list. Search results have a
// price and a size; the purchase history has a purchase date but neither.
type App struct {
	ID           int64
	BundleID     string
	Name         string
	Version      string
	Price        float64
	PurchaseDate time.Time
	Platforms    []string
	Size         int64 // bytes, as listed by Apple; 0 when unknown (My apps)
}
