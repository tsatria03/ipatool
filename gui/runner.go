package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

var platforms = []string{"iphone", "ipad", "appletv", "visionos", "macos"}

const twoFAHint = "2FA code is required"

// Settings are what the GUI remembers between sessions, stored as JSON in
// %APPDATA%\ipatool-gui\settings.json.
type Settings struct {
	Exe        string `json:"exe"`
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

// findExe returns the newest ipatool*.exe next to this program, in the current
// folder, or in ../releases (for `go run` from the gui folder).
func findExe() string {
	var dirs []string
	if self, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Dir(self))
	}
	if wd, err := os.Getwd(); err == nil {
		dirs = append(dirs, wd, filepath.Join(wd, "..", "releases"))
	}

	versionOf := func(path string) []int {
		name := strings.SplitN(filepath.Base(path), "-windows", 2)[0]
		var parts []int
		for _, n := range regexp.MustCompile(`\d+`).FindAllString(name, -1) {
			v, _ := strconv.Atoi(n)
			parts = append(parts, v)
		}
		return parts
	}
	less := func(a, b []int) bool {
		for i := 0; i < len(a) && i < len(b); i++ {
			if a[i] != b[i] {
				return a[i] < b[i]
			}
		}
		return len(a) < len(b)
	}

	for _, dir := range dirs {
		matches, _ := filepath.Glob(filepath.Join(dir, "ipatool*.exe"))
		var tools []string
		for _, m := range matches {
			// Skip this GUI itself if it was built as ipatool-gui.exe.
			if !strings.Contains(strings.ToLower(filepath.Base(m)), "gui") {
				tools = append(tools, m)
			}
		}
		if len(tools) > 0 {
			sort.Slice(tools, func(i, j int) bool { return less(versionOf(tools[i]), versionOf(tools[j])) })
			abs, _ := filepath.Abs(tools[len(tools)-1])
			return abs
		}
	}
	return ""
}

// resolveExe decides which ipatool to run. It keeps a saved exe while it exists,
// otherwise looks for an exe again (e.g. after upgrading), and when there is none
// falls back to the ipatool source folder, which is run with `go run`.
// A saved source folder gives way to an exe as soon as one appears.
func resolveExe(saved string) string {
	if saved != "" {
		if info, err := os.Stat(saved); err == nil {
			if !info.IsDir() {
				return saved
			}
			if isSourceDir(saved) {
				if exe := findExe(); exe != "" {
					return exe
				}
				return saved
			}
		}
	}
	if exe := findExe(); exe != "" {
		return exe
	}
	return findSourceDir()
}

// isSourceDir reports whether dir is the ipatool repository (not this GUI's module).
func isSourceDir(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	return err == nil && bytes.Contains(data, []byte("module github.com/majd/ipatool"))
}

// findSourceDir looks for the ipatool repository in the current folder and the
// folders above it (the GUI normally runs from its gui subfolder), then next to
// this program.
func findSourceDir() string {
	var starts []string
	if wd, err := os.Getwd(); err == nil {
		starts = append(starts, wd)
	}
	if self, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(self))
	}
	for _, dir := range starts {
		for {
			if isSourceDir(dir) {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return ""
}

func maskArgs(args []string) []string {
	secret := map[string]bool{"-p": true, "--password": true, "--keychain-passphrase": true, "--auth-code": true}
	masked := make([]string, len(args))
	hideNext := false
	for i, arg := range args {
		if hideNext {
			masked[i] = "****"
		} else {
			masked[i] = arg
		}
		hideNext = secret[arg]
	}
	return masked
}

// friendlyError translates ipatool's raw error strings into plain advice.
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
		return "Apple didn't send a download for this app. Check that the platform is right, that you are " +
			"using ipatool 2.6.0 or newer, or try again later."
	case strings.Contains(lowered, "app not found"):
		return "Apple couldn't find that app. It may have been removed from the App Store in your country."
	}
	return strings.TrimSpace(text)
}

// Result is the outcome of one ipatool run in --format json mode.
type Result struct {
	Code   int
	Events []map[string]any
	Raw    string
	Output string
}

// Final is the last event that reports success or failure, which carries the command's data.
func (r Result) Final() map[string]any {
	for i := len(r.Events) - 1; i >= 0; i-- {
		if _, ok := r.Events[i]["success"]; ok {
			return r.Events[i]
		}
	}
	if len(r.Events) > 0 {
		return r.Events[len(r.Events)-1]
	}
	return map[string]any{}
}

func (r Result) OK() bool {
	success, ok := r.Final()["success"].(bool)
	return r.Code == 0 && (!ok || success)
}

func (r Result) Error() string {
	if msg := r.Str("error"); msg != "" {
		return friendlyError(msg)
	}
	if raw := strings.TrimSpace(r.Raw); raw != "" {
		return friendlyError(raw)
	}
	return fmt.Sprintf("Unknown error (exit code %d).", r.Code)
}

func (r Result) HasMessage(text string) bool {
	for _, event := range r.Events {
		if msg, _ := event["message"].(string); strings.Contains(msg, text) {
			return true
		}
	}
	return false
}

func (r Result) Str(key string) string {
	switch v := r.Final()[key].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	}
	return ""
}

func (r Result) Int(key string) int {
	v, _ := r.Final()[key].(float64)
	return int(v)
}

func (r Result) Strings(key string) []string {
	items, _ := r.Final()[key].([]any)
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, fmt.Sprint(item))
	}
	return out
}

// App is one app from search or list-purchases output.
type App struct {
	ID        int64
	BundleID  string
	Name      string
	Version   string
	Price     float64
	Platforms []string
}

func (r Result) Apps() []App {
	items, _ := r.Final()["apps"].([]any)
	apps := make([]App, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		app := App{}
		if v, ok := m["id"].(float64); ok {
			app.ID = int64(v)
		}
		app.BundleID, _ = m["bundleID"].(string)
		app.Name, _ = m["name"].(string)
		app.Version, _ = m["version"].(string)
		app.Price, _ = m["price"].(float64)
		if list, ok := m["platforms"].([]any); ok {
			for _, p := range list {
				app.Platforms = append(app.Platforms, fmt.Sprint(p))
			}
		}
		apps = append(apps, app)
	}
	return apps
}

// Runner starts ipatool without a console window and remembers the running
// process so it can be cancelled. exe is either ipatool.exe or the ipatool
// source folder, which is run with `go run`.
type Runner struct {
	mu      sync.Mutex
	process *os.Process
}

func (r *Runner) Run(exe string, args []string) Result {
	var cmd *exec.Cmd
	if isSourceDir(exe) {
		goExe, err := exec.LookPath("go")
		if err != nil {
			msg := "Go isn't installed (or isn't on the PATH), so ipatool can't be run from its source folder. " +
				"Install Go, or choose an ipatool exe on the Account page."
			return Result{Code: -1, Raw: msg, Output: msg}
		}
		cmd = exec.Command(goExe, append([]string{"run", "."}, args...)...)
		cmd.Dir = exe
	} else {
		cmd = exec.Command(exe, args...)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	if err := cmd.Start(); err != nil {
		return Result{Code: -1, Raw: err.Error(), Output: err.Error()}
	}
	r.mu.Lock()
	r.process = cmd.Process
	r.mu.Unlock()

	err := cmd.Wait()

	r.mu.Lock()
	r.process = nil
	r.mu.Unlock()

	code := 0
	if err != nil {
		code = -1
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		}
	}

	result := Result{Code: code, Output: stdout.String() + stderr.String()}
	var raw []string
	scanner := bufio.NewScanner(&stdout)
	scanner.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		var event map[string]any
		if json.Unmarshal([]byte(line), &event) == nil {
			result.Events = append(result.Events, event)
		} else if strings.TrimSpace(line) != "" {
			raw = append(raw, line)
		}
	}
	raw = append(raw, stderr.String())
	result.Raw = strings.Join(raw, "\n")
	return result
}

func (r *Runner) Cancel() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.process == nil {
		return false
	}
	// Stop the whole process tree: with `go run`, ipatool is a child of go.exe.
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(r.process.Pid))
	kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if kill.Run() != nil {
		_ = r.process.Kill()
	}
	return true
}
