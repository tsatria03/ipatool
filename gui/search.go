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
// store is on for Global search, whose results come with prices and sizes. The
// purchase history (My apps) has neither: its Price column is left blank
// instead of claiming every app is free, and it has no Size column.
type appModel struct {
	walk.TableModelBase
	apps  []App
	store bool
}

type appColumn struct {
	title string
	width int
	value func(m *appModel, app App) string
}

var (
	nameColumn    = appColumn{"Name", 220, func(_ *appModel, a App) string { return a.Name }}
	bundleColumn  = appColumn{"Bundle ID", 210, func(_ *appModel, a App) string { return a.BundleID }}
	versionColumn = appColumn{"Version", 80, func(_ *appModel, a App) string { return a.Version }}
	priceColumn   = appColumn{"Price", 60, func(m *appModel, a App) string {
		switch {
		case !m.store:
			return ""
		case a.Price == 0:
			return "Free"
		}
		return strconv.FormatFloat(a.Price, 'f', 2, 64)
	}}
	sizeColumn      = appColumn{"Size", 70, func(_ *appModel, a App) string { return formatSize(a.Size) }}
	platformsColumn = appColumn{"Platforms", 130, func(_ *appModel, a App) string { return strings.Join(a.Platforms, ", ") }}
	idColumn        = appColumn{"App ID", 100, func(_ *appModel, a App) string { return strconv.FormatInt(a.ID, 10) }}
)

// columns lists the model's columns in order; Value and the TableView both use it.
func (m *appModel) columns() []appColumn {
	if m.store {
		return []appColumn{nameColumn, bundleColumn, versionColumn, priceColumn, sizeColumn, platformsColumn, idColumn}
	}
	return []appColumn{nameColumn, bundleColumn, versionColumn, priceColumn, platformsColumn, idColumn}
}

func (m *appModel) RowCount() int { return len(m.apps) }

func (m *appModel) Value(row, col int) interface{} {
	if cols := m.columns(); col < len(cols) {
		return cols[col].value(m, m.apps[row])
	}
	return ""
}

func appColumns(m *appModel) []TableViewColumn {
	var out []TableViewColumn
	for _, c := range m.columns() {
		out = append(out, TableViewColumn{Title: c.title, Width: c.width})
	}
	return out
}

// formatSize shows a size in bytes the way the App Store does, in decimal units:
// "850 KB", "393 MB", "1.2 GB". Unknown sizes (0) are blank.
func formatSize(bytes int64) string {
	switch {
	case bytes <= 0:
		return ""
	case bytes < 1e6:
		return fmt.Sprintf("%d KB", max(1, (bytes+500)/1e3))
	case bytes < 1e9:
		return fmt.Sprintf("%d MB", (bytes+5e5)/1e6)
	}
	return fmt.Sprintf("%.1f GB", float64(bytes)/1e9)
}

func (g *gui) searchTab() TabPage {
	s := &g.search
	s.model = &appModel{store: true}

	return TabPage{
		Title:  "Global search",
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
			Label{Text: "Global search resul&ts:"},
			TableView{AssignTo: &s.table, Model: s.model, Columns: appColumns(s.model),
				OnItemActivated: func() { g.sendToDownload(s.table, s.model, s.platform.Text()) }},
			g.listButtons(&s.table, s.model, func() string { return s.platform.Text() }, nil, nil),
		},
	}
}

// listButtons returns the "Download selected" and "Copy bundle ID" row used under
// both app lists, followed by any extra buttons. download and copy optionally
// receive the buttons (may be nil).
func (g *gui) listButtons(table **walk.TableView, model *appModel, platform func() string,
	download, copy **walk.PushButton, extra ...Widget) Composite {
	buttons := []Widget{
		PushButton{AssignTo: download, Text: "&Download selected", OnClicked: func() { g.sendToDownload(*table, model, platform()) }},
		PushButton{AssignTo: copy, Text: "&Copy bundle ID", OnClicked: func() { g.copyBundleID(*table, model) }},
	}
	return Composite{Layout: HBox{MarginsZero: true}, Children: append(append(buttons, extra...), HSpacer{})}
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
func (g *gui) fillList(table *walk.TableView, model *appModel, apps []App, err error, title string) {
	if err != nil {
		g.error(title, errorText(err))
		return
	}
	model.apps = apps
	model.PublishRowsReset()
	g.setStatus(fmt.Sprintf("%d apps found.", len(model.apps)))
	if len(model.apps) == 0 {
		g.info("No results", "No apps found.")
		return
	}
	_ = table.SetCurrentIndex(0)
	_ = table.SetFocus()
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
	} else if app.ID != 0 {
		if d.knownIDs == nil {
			d.knownIDs = map[string]int64{}
		}
		d.knownIDs[target] = app.ID
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
