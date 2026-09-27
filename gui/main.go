// Command ipatool-gui is a screen-reader-friendly Windows front end for ipatool.
//
// It uses walk, which builds the window from native Windows controls that NVDA,
// JAWS and Narrator can read. Every action runs the ipatool command line tool
// (an exe, or the source folder via go run) in non-interactive JSON mode.
package main

import (
	"log"
	"strings"
	"time"

	"github.com/tailscale/walk"
	. "github.com/tailscale/walk/declarative"
	"github.com/tailscale/win"
)

var pageTitles = []string{"Account", "Search", "Download", "My apps", "Log"}

const shortcutsHelp = `Keyboard shortcuts

Ctrl+1 to Ctrl+5: go to the Account, Search, Download, My apps or Log page.
Ctrl+Tab / Ctrl+Shift+Tab: next / previous page.
Switching pages puts you on the page tabs; Left and Right arrows also switch pages there.
Tab / Shift+Tab: move between fields and buttons (Tab from the page tabs enters the page).
Alt + underlined letter: jump to a field or press a button.
Enter in the search box: search.
Enter in the Apple ID password box: log in.
Enter on an app in a results list: send it to the Download page.
Escape: cancel the task that is running.
F5: check which account you are signed in with.
F1: this help.`

type gui struct {
	app      *walk.Application
	mw       *walk.MainWindow
	tabs     *walk.TabWidget
	status   *walk.StatusBarItem
	settings Settings
	runner   Runner

	busy       bool
	busyStatus string
	busyStart  time.Time
	cancelled  bool   // the running task was cancelled with Escape
	cancelTask func() // cancels the running engine task, if any
	// busyProgress, when set, is called every second while busy; a non-empty
	// result (e.g. "45 percent") replaces the seconds counter in the status bar.
	busyProgress func() string

	tabList win.HWND // the native tab strip inside the TabWidget

	account   accountPage
	search    searchPage
	download  downloadPage
	purchases purchasesPage
	logText   *walk.TextEdit
}

func main() {
	app, err := walk.InitApp()
	if err != nil {
		log.Fatal(err)
	}
	g := &gui{app: app, settings: loadSettings()}

	if err := g.window().Create(); err != nil {
		log.Fatal(err)
	}
	// walk always creates a toolbar; this app has none, and an empty one would be a stray Tab stop.
	g.mw.ToolBar().SetVisible(false)
	fixTableView(g.search.table, "Search results", "Alt+T")
	fixTableView(g.purchases.table, "Apps you own", "Alt+W")
	assignControlIDs(g.mw.Handle())
	g.setupTabs()

	g.mw.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		g.persist()
		g.runner.Cancel()
	})
	g.setStatus("Ready. Press F1 for keyboard shortcuts.")
	g.goToPage(0)
	if g.account.passphrase.Text() != "" && g.account.exe.Text() != "" {
		g.checkAccount(false)
	}
	app.Run()
}

func (g *gui) window() MainWindow {
	viewItems := make([]MenuItem, len(pageTitles))
	for i, title := range pageTitles {
		i := i
		viewItems[i] = Action{
			Text:        title + " page",
			Shortcut:    Shortcut{Modifiers: walk.ModControl, Key: walk.Key1 + walk.Key(i)},
			OnTriggered: func() { g.goToPage(i) },
		}
	}

	return MainWindow{
		AssignTo: &g.mw,
		Title:    "ipatool GUI",
		Size:     Size{Width: 900, Height: 620},
		MinSize:  Size{Width: 640, Height: 440},
		Layout:   VBox{},
		MenuItems: []MenuItem{
			Menu{Text: "&File", Items: []MenuItem{
				Action{Text: "E&xit", OnTriggered: func() { g.mw.Close() }},
			}},
			Menu{Text: "&View", Items: viewItems},
			Menu{Text: "&Task", Items: []MenuItem{
				Action{Text: "&Check account", Shortcut: Shortcut{Key: walk.KeyF5}, OnTriggered: func() { g.checkAccount(true) }},
				Action{Text: "C&ancel current task", Shortcut: Shortcut{Key: walk.KeyEscape}, OnTriggered: g.cancel},
			}},
			Menu{Text: "&Help", Items: []MenuItem{
				Action{Text: "&Keyboard shortcuts", Shortcut: Shortcut{Key: walk.KeyF1},
					OnTriggered: func() { g.info("Keyboard shortcuts", shortcutsHelp) }},
			}},
		},
		StatusBarItems: []StatusBarItem{{AssignTo: &g.status, Width: 600}},
		Children: []Widget{
			TabWidget{
				AssignTo: &g.tabs,
				Pages: []TabPage{
					g.accountTab(),
					g.searchTab(),
					g.downloadTab(),
					g.purchasesTab(),
					g.logTab(),
				},
			},
		},
	}
}

func (g *gui) logTab() TabPage {
	return TabPage{
		Title:  "Log",
		Layout: VBox{},
		Children: []Widget{
			Label{Text: "Command &log:"},
			TextEdit{AssignTo: &g.logText, ReadOnly: true, VScroll: true,
				Accessibility: Accessibility{Name: "Command log", Accelerator: "Alt+L"}},
		},
	}
}

// labeled returns a label and control for a 3-column Grid, plus an optional third
// widget. walk nests labels inside a wrapper window, so screen readers can't link
// them to the next control; each control therefore gets its own accessible name.
func labeled(label string, control Widget, extra Widget) []Widget {
	if extra == nil {
		extra = HSpacer{}
	}
	return []Widget{Label{Text: label}, control, extra}
}

// accessible builds a control's accessible name and Alt shortcut from its label text.
func accessible(label string) Accessibility {
	a := Accessibility{Name: strings.TrimSuffix(strings.ReplaceAll(label, "&", ""), ":")}
	if i := strings.Index(label, "&"); i >= 0 && i+1 < len(label) {
		a.Accelerator = "Alt+" + strings.ToUpper(label[i+1:i+2])
	}
	return a
}

// ----- navigation and messages ------------------------------------------------

func (g *gui) setStatus(text string) {
	_ = g.status.SetText(text)
}

func (g *gui) info(title, message string) {
	walk.MsgBox(g.mw, title, message, walk.MsgBoxOK|walk.MsgBoxIconInformation)
}

func (g *gui) error(title, message string) {
	walk.MsgBox(g.mw, title, message, walk.MsgBoxOK|walk.MsgBoxIconError)
}

func (g *gui) log(text string) {
	g.logText.AppendText(strings.ReplaceAll(strings.TrimRight(text, "\n"), "\n", "\r\n") + "\r\n")
}

// ----- running ipatool ----------------------------------------------------------

// run starts ipatool in the background and calls onDone on the UI thread when it finishes.
func (g *gui) run(args []string, status string, needsPassphrase bool, onDone func(Result)) {
	if g.busy {
		walk.MsgBox(g.mw, "Busy", "Please wait for the current task to finish, or press Escape to cancel it.",
			walk.MsgBoxOK|walk.MsgBoxIconWarning)
		return
	}
	exe := strings.TrimSpace(g.account.exe.Text())
	if exe == "" || resolveExe(exe) != exe {
		g.error("ipatool not found", "No ipatool exe or ipatool source folder was found. In the \"ipatool program\" "+
			"field on the Account page, enter the path of ipatool.exe or of the ipatool source folder.")
		g.focusWidget(0, g.account.exe)
		return
	}
	passphrase := g.account.passphrase.Text()
	if needsPassphrase && passphrase == "" {
		g.error("Passphrase needed", "Enter a keychain passphrase on the Account page.")
		g.focusWidget(0, g.account.passphrase)
		return
	}

	full := append(append([]string{}, args...), "--format", "json", "--non-interactive")
	if passphrase != "" {
		full = append(full, "--keychain-passphrase", passphrase)
	}

	if isSourceDir(exe) {
		status += " (running ipatool from source; after a code change it recompiles first)"
	}
	g.log("> ipatool " + strings.Join(maskArgs(args), " "))
	stopBusy := g.startBusy(status)

	go func() {
		result := g.runner.Run(exe, full)
		g.app.Synchronize(func() {
			stopBusy()
			out := strings.TrimSpace(result.Output)
			if out == "" {
				out = "(no output)"
			}
			g.log(out)
			if g.cancelled {
				// A cancelled task is not a failure: skip its result and error messages.
				g.cancelled = false
				g.log("(cancelled)")
				g.setStatus("Cancelled.")
				return
			}
			if result.OK() {
				g.setStatus("Done.")
			} else {
				g.setStatus("Failed.")
			}
			onDone(result)
		})
	}()
}

func (g *gui) cancel() {
	if !g.busy {
		return
	}
	if g.cancelTask != nil {
		g.cancelTask()
	} else if !g.runner.Cancel() {
		return
	}
	g.cancelled = true
	g.setStatus("Cancelling...")
}

func (g *gui) persist() {
	s := Settings{
		Exe:    strings.TrimSpace(g.account.exe.Text()),
		Email:  strings.TrimSpace(g.account.email.Text()),
		Output: strings.TrimSpace(g.download.output.Text()),
	}
	if g.account.remember.Checked() {
		s.Passphrase = g.account.passphrase.Text()
	}
	saveSettings(s)
}
