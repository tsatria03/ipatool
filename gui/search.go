package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/tailscale/walk"
	. "github.com/tailscale/walk/declarative"
)

var resultLimits = []string{"5", "10", "25", "50"}

type searchPage struct {
	term     *walk.LineEdit
	platform *walk.ComboBox
	limit    *walk.ComboBox
	table    *walk.TableView
	model    *appModel
}

// appModel feeds a list of apps to a TableView (a native Windows list view).
type appModel struct {
	walk.TableModelBase
	apps []App
}

func (m *appModel) RowCount() int { return len(m.apps) }

func (m *appModel) Value(row, col int) interface{} {
	app := m.apps[row]
	switch col {
	case 0:
		return app.Name
	case 1:
		return app.BundleID
	case 2:
		return app.Version
	case 3:
		if app.Price == 0 {
			return "Free"
		}
		return strconv.FormatFloat(app.Price, 'f', 2, 64)
	case 4:
		return strings.Join(app.Platforms, ", ")
	case 5:
		return strconv.FormatInt(app.ID, 10)
	}
	return ""
}

func appColumns() []TableViewColumn {
	return []TableViewColumn{
		{Title: "Name", Width: 220},
		{Title: "Bundle ID", Width: 210},
		{Title: "Version", Width: 80},
		{Title: "Price", Width: 60},
		{Title: "Platforms", Width: 130},
		{Title: "App ID", Width: 100},
	}
}

func (g *gui) searchTab() TabPage {
	s := &g.search
	s.model = &appModel{}

	return TabPage{
		Title:  "Search",
		Layout: VBox{},
		Children: []Widget{
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				Label{Text: "Search &for:"},
				LineEdit{AssignTo: &s.term, Accessibility: accessible("Search &for:"),
					OnKeyDown: func(key walk.Key) {
						if key == walk.KeyReturn {
							g.runSearch()
						}
					}},
				Label{Text: "&Platform:"},
				ComboBox{AssignTo: &s.platform, Model: platforms, CurrentIndex: 0, Accessibility: accessible("&Platform:")},
				Label{Text: "&Number of results:"},
				ComboBox{AssignTo: &s.limit, Model: resultLimits, CurrentIndex: 1,
					Accessibility: accessible("&Number of results:")},
				PushButton{Text: "&Search", OnClicked: g.runSearch},
			}},
			Label{Text: "Search resul&ts:"},
			TableView{AssignTo: &s.table, Model: s.model, Columns: appColumns(),
				OnItemActivated: func() { g.sendToDownload(s.table, s.model, s.platform.Text()) }},
			g.listButtons(&s.table, s.model, func() string { return s.platform.Text() }, nil, nil),
		},
	}
}

// listButtons returns the "Download selected" and "Copy bundle ID" row used under
// both app lists. download and copy optionally receive the buttons (may be nil).
func (g *gui) listButtons(table **walk.TableView, model *appModel, platform func() string,
	download, copy **walk.PushButton) Composite {
	return Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
		PushButton{AssignTo: download, Text: "&Download selected", OnClicked: func() { g.sendToDownload(*table, model, platform()) }},
		PushButton{AssignTo: copy, Text: "&Copy bundle ID", OnClicked: func() { g.copyBundleID(*table, model) }},
		HSpacer{},
	}}
}

func (g *gui) runSearch() {
	s := &g.search
	term := strings.TrimSpace(s.term.Text())
	if term == "" {
		_ = s.term.SetFocus()
		return
	}
	limit, _ := strconv.ParseInt(s.limit.Text(), 10, 64)
	platform := s.platform.Text()
	g.runTask(fmt.Sprintf("search %q (%s, up to %d)", term, platform, limit), "Searching.",
		func(ctx context.Context, b *backend) (any, error) { return b.search(term, limit, platform) },
		func(result any, err error) {
			apps, _ := result.([]App)
			g.fillList(s.table, s.model, apps, err, "Search failed")
		})
}

// fillList shows apps (or the error) and moves focus to the first app so it is read aloud.
func (g *gui) fillList(table *walk.TableView, model *appModel, apps []App, err error, title string) bool {
	if err != nil {
		g.error(title, errorText(err))
		return false
	}
	model.apps = apps
	model.PublishRowsReset()
	g.setStatus(fmt.Sprintf("%d apps found.", len(model.apps)))
	if len(model.apps) == 0 {
		g.info("No results", "No apps found.")
		return true
	}
	_ = table.SetCurrentIndex(0)
	_ = table.SetFocus()
	return true
}

func selectedApp(table *walk.TableView, model *appModel) (App, bool) {
	i := table.CurrentIndex()
	if i < 0 || i >= len(model.apps) {
		return App{}, false
	}
	return model.apps[i], true
}

func (g *gui) sendToDownload(table *walk.TableView, model *appModel, platform string) {
	app, ok := selectedApp(table, model)
	if !ok {
		g.info("Nothing selected", "Select an app in the list first.")
		return
	}
	// Prefer the chosen platform, but only if the app actually supports it.
	if len(app.Platforms) > 0 && !contains(app.Platforms, platform) {
		platform = app.Platforms[0]
	}
	if !contains(platforms, platform) {
		platform = "iphone"
	}
	d := &g.download
	target := app.BundleID
	if target == "" {
		target = strconv.FormatInt(app.ID, 10)
	}
	_ = d.app.SetText(target)
	_ = d.platform.SetCurrentIndex(indexOf(platforms, platform))
	_ = d.version.SetText("")
	_ = d.result.SetText("Ready to download " + app.Name + ".")
	g.focusWidget(2, d.app)
}

func (g *gui) copyBundleID(table *walk.TableView, model *appModel) {
	if app, ok := selectedApp(table, model); ok {
		_ = walk.Clipboard().SetText(app.BundleID)
		g.setStatus("Copied " + app.BundleID)
	}
}

func contains(list []string, value string) bool { return indexOf(list, value) >= 0 }

func indexOf(list []string, value string) int {
	for i, item := range list {
		if item == value {
			return i
		}
	}
	return -1
}
