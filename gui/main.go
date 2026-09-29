// Command ipatool-gui is a screen-reader-friendly Windows front end for ipatool.
//
// It uses walk, which builds the window from native Windows controls that NVDA,
// JAWS and Narrator can read. It uses ipatool's engine (pkg/) directly, so it
// works on its own without ipatool.exe; see backend.go.
package main

import (
	"log"
	"strings"
	"time"

	"github.com/tailscale/walk"
	. "github.com/tailscale/walk/declarative"
	"github.com/tailscale/win"
)

var pageTitles = []string{"Account", "Global search", "Download", "My apps", "Log"}

const shortcutsHelp = `Keyboard shortcuts

Ctrl+1 to Ctrl+5: go to the Account, Global search, Download, My apps or Log page.
Ctrl+Tab / Ctrl+Shift+Tab: next / previous page.
Switching pages puts you on the page tabs; Left and Right arrows also switch pages there.
Tab / Shift+Tab: move between fields and buttons (Tab from the page tabs enters the page).
Alt + underlined letter: jump to a field or press a button.
Enter in the search box: search.
Enter in the Apple ID password box: log in.
Enter on an app in a results list: send it to the Download page.
App info (Alt+O), or Alt+Enter on an app in a list: show what the App Store lists
  about it (developer, size, rating, what's new, description and more) in a window you
  can read line by line; Escape closes it.
Letters in an app list: jump to the next app whose name starts with them; type
  several letters quickly to match more of the name (current page only).
Download (Alt+D on the Download page): starts the download and moves you to the
  Result field, which shows the progress.
My apps: your apps load the first time you move into the page. Search my apps (Alt+S),
  the availability filter (Alt+I: all apps, available, or no longer on the App Store),
  the platform filter (Alt+T) and Sort by (Alt+B) work instantly; Enter in any of them
  moves to the list. The Search button (Alt+R) does the same as Enter in Search my apps.
  Load (Alt+L) gets a fresh list from Apple.
  Copy all apps (Alt+A) copies every matching app (all pages, in the sorted order) as
  text, one line per app; Export to JSON (Alt+E) saves them as a JSON file.
Escape: cancel the task that is running.
F5: check which account you are signed in with.
F1: this help.`

type gui struct {
	app      *walk.Application
	mw       *walk.MainWindow
	tabs     *walk.TabWidget
	status   *walk.StatusBarItem
	settings Settings

	busy       bool
	busyStatus string
	busyStart  time.Time
	cancelled  bool   // the running task was cancelled with Escape
	cancelTask func() // cancels the running engine task, if any
	// busyProgress, when set, is called every second while busy; a non-empty
	// result (e.g. "45 percent") replaces the seconds counter in the status bar.
	busyProgress func() string
	// afterTask, when set right after starting a task, runs when that task ends,
	// including when it was cancelled.
	afterTask func()

	tabList win.HWND // the native tab strip inside the TabWidget

	account   accountPage
	search    searchPage
	download  downloadPage
	purchases purchasesPage
	logText   *walk.TextEdit

	// detailsCache holds the App info looked up this session, by app ID (nil:
	// not on the App Store), so pressing App info again asks Apple nothing.
	detailsCache map[int64]*storeDetails
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
	fixTableView(g.search.table, "Global search results", "Alt+T")
	fixTableView(g.purchases.table, "Apps you own", "Alt+W")
	for _, list := range []struct {
		tv    *walk.TableView
		model *appModel
	}{{g.search.table, g.search.model}, {g.purchases.table, g.purchases.model}} {
		model := list.model
		enableTypeAhead(list.tv, model.RowCount, func(row int) string { return model.apps[row].Name })
	}
	assignControlIDs(g.mw.Handle())
	g.setupTabs()
	g.setupMyApps()

	g.mw.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		g.persist()
		if g.cancelTask != nil {
			g.cancelTask()
		}
	})
	g.setStatus("Ready. Press F1 for keyboard shortcuts.")
	g.goToPage(0)
	if g.account.passphrase.Text() != "" {
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

// ----- tasks and settings --------------------------------------------------------

// cancel stops the running task (Escape); its result is then discarded.
func (g *gui) cancel() {
	if !g.busy || g.cancelTask == nil {
		return
	}
	g.cancelTask()
	g.cancelled = true
	g.setStatus("Cancelling...")
}

func (g *gui) persist() {
	s := Settings{
		Email:  strings.TrimSpace(g.account.email.Text()),
		Output: strings.TrimSpace(g.download.output.Text()),
	}
	if g.account.remember.Checked() {
		s.Passphrase = g.account.passphrase.Text()
	}
	saveSettings(s)
}
