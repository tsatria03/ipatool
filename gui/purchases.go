package main

import (
	"context"
	"fmt"

	"github.com/tailscale/walk"
	. "github.com/tailscale/walk/declarative"
)

const (
	allPlatforms = "all platforms"
	appsPerPage  = 100
)

// The My apps page fetches the whole purchase history once per session (about
// 12 seconds) and then searches, filters and pages through it locally, which
// is instant; see myapps.go. The first time focus enters the page's controls,
// the list loads by itself.
type purchasesPage struct {
	load, previous, next, find *walk.PushButton
	download, copy             *walk.PushButton
	filter, sort               *walk.ComboBox
	search, pageInfo           *walk.LineEdit
	table                      *walk.TableView
	model                      *appModel

	all        []App // the whole purchase history, newest first
	loaded     bool  // all holds a fetched list
	loading    bool  // a fetch from Apple is running
	autoLoaded bool  // the first-visit load has been tried this session
	page       int
}

func (g *gui) purchasesTab() TabPage {
	p := &g.purchases
	p.model = &appModel{} // no prices: the purchase history doesn't include them
	p.page = 1

	return TabPage{
		Title:  "My apps",
		Layout: VBox{},
		Children: []Widget{
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				PushButton{AssignTo: &p.load, Text: "&Load", OnClicked: func() { g.fetchMyApps(false, nil) }},
				PushButton{AssignTo: &p.previous, Text: "&Previous page", OnClicked: func() { g.turnMyAppsPage(-1) }},
				PushButton{AssignTo: &p.next, Text: "&Next page", OnClicked: func() { g.turnMyAppsPage(+1) }},
				Label{Text: "Pla&tform filter:"},
				ComboBox{AssignTo: &p.filter, Model: append([]string{allPlatforms}, platforms...), CurrentIndex: 0,
					Accessibility: accessible("Pla&tform filter:"), OnCurrentIndexChanged: g.myAppsViewChanged},
				Label{Text: "Pa&ge:"},
				LineEdit{AssignTo: &p.pageInfo, ReadOnly: true, Text: "Not loaded", Accessibility: accessible("Pa&ge:")},
			}},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				Label{Text: "&Search my apps:"},
				LineEdit{AssignTo: &p.search, Accessibility: accessible("&Search my apps:"),
					OnKeyDown: func(key walk.Key) {
						if key == walk.KeyReturn {
							g.searchMyApps()
						}
					}},
				PushButton{AssignTo: &p.find, Text: "Sea&rch", OnClicked: g.searchMyApps},
				Label{Text: "Sort &by:"},
				ComboBox{AssignTo: &p.sort, Model: sortChoices, CurrentIndex: 0,
					Accessibility: accessible("Sort &by:"), OnCurrentIndexChanged: g.myAppsViewChanged},
			}},
			Label{Text: "Apps you o&wn:"},
			TableView{AssignTo: &p.table, Model: p.model, Columns: appColumns(p.model),
				OnItemActivated: func() { g.sendToDownload(p.table, p.model, g.purchaseFilter()) }},
			g.listButtons(&p.table, p.model, g.purchaseFilter, &p.download, &p.copy),
		},
	}
}

// setupMyApps starts the first-visit load when focus first enters one of the
// page's controls. Only focus gains count, so arrowing past the tab, Ctrl+4 and
// switching to another program never start it.
func (g *gui) setupMyApps() {
	p := &g.purchases
	widgets := []walk.Widget{p.load, p.previous, p.next, p.filter, p.pageInfo, p.search, p.find, p.sort, p.table, p.download, p.copy}
	for _, w := range widgets {
		w := w
		w.FocusedChanged().Attach(func() {
			if w.Focused() && !p.autoLoaded && !p.loaded && !p.loading {
				p.autoLoaded = true
				g.fetchMyApps(true, nil)
			}
		})
	}
}

func (g *gui) purchaseFilter() string {
	if f := g.purchases.filter.Text(); f != allPlatforms {
		return f
	}
	return ""
}

// fetchMyApps gets the whole purchase history from Apple. Automatic loads keep
// focus where it is and never show the Busy box; then runs after a successful load.
func (g *gui) fetchMyApps(auto bool, then func()) {
	p := &g.purchases
	if p.loading {
		return // e.g. Alt+L focused Load, which started the first-visit load, then clicked it
	}
	if g.busy {
		if auto {
			g.setStatus("My apps will load when you press Load (Alt+L).")
			p.autoLoaded = false // try again on the next visit
			return
		}
		g.showBusy()
		return
	}
	g.runTask("load all my apps", "Loading all your apps from Apple.",
		func(ctx context.Context, b *backend) (any, error) { return b.ownedAppsAll() },
		func(result any, err error) {
			if err != nil {
				if auto {
					_ = p.pageInfo.SetText("Not loaded: " + errorText(err))
					g.setStatus("Could not load your apps: " + errorText(err))
				} else {
					g.error("Could not load your apps", errorText(err))
				}
				return
			}
			p.all, p.loaded, p.page = result.([]App), true, 1
			g.showMyApps(!auto)
			g.setStatus(fmt.Sprintf("%d apps loaded.", len(p.all)))
			if then != nil {
				then()
			}
		})
	if g.busy { // the task started
		p.loading = true
		g.afterTask = func() { p.loading = false } // also after Escape
	}
}

// showMyApps fills the list with the current page of the filtered, searched and
// sorted apps and updates the Page field. focusList moves focus to the first app.
func (g *gui) showMyApps(focusList bool) {
	p := &g.purchases
	query := p.search.Text()
	view := filterApps(p.all, g.purchaseFilter(), query) // a new slice, safe to sort
	sortApps(view, p.sort.CurrentIndex())
	apps, current, pages := pageOf(view, p.page, appsPerPage)
	p.page = current
	p.model.apps = apps
	p.model.PublishRowsReset()
	narrowed := g.purchaseFilter() != "" || query != ""
	_ = p.pageInfo.SetText(pageInfoText(current, pages, len(view), len(p.all), narrowed))
	if len(apps) > 0 {
		_ = p.table.SetCurrentIndex(0)
		if focusList {
			_ = p.table.SetFocus()
		}
	}
}

func (g *gui) turnMyAppsPage(step int) {
	p := &g.purchases
	if !p.loaded {
		g.fetchMyApps(false, nil)
		return
	}
	p.page += step
	g.showMyApps(true)
	g.setStatus(p.pageInfo.Text())
}

// myAppsViewChanged applies a new platform filter or sort order instantly; focus
// stays on the combo box so arrowing through the choices keeps working.
func (g *gui) myAppsViewChanged() {
	p := &g.purchases
	if !p.loaded {
		return // the first load uses whatever is selected
	}
	p.page = 1
	g.showMyApps(false)
	g.setStatus(p.pageInfo.Text())
}

// myAppsComboEnter is Enter in the platform filter or Sort by: go to the list.
func (g *gui) myAppsComboEnter() {
	p := &g.purchases
	if !p.loaded {
		g.fetchMyApps(false, nil)
		return
	}
	if len(p.model.apps) > 0 {
		_ = p.table.SetFocus()
	}
}

// searchMyApps is Enter in the "Search my apps" box, or the Search button.
func (g *gui) searchMyApps() {
	p := &g.purchases
	apply := func() {
		p.page = 1
		g.showMyApps(true)
		if len(p.model.apps) == 0 {
			g.info("No matches", fmt.Sprintf("None of your apps match %q.", p.search.Text()))
			_ = p.search.SetFocus()
			return
		}
		g.setStatus(p.pageInfo.Text())
	}
	if !p.loaded {
		g.fetchMyApps(false, apply)
		return
	}
	apply()
}
