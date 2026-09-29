package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/tailscale/walk"
	. "github.com/tailscale/walk/declarative"
	"github.com/tailscale/win"
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
	download, copy, info       *walk.PushButton
	copyAll, export            *walk.PushButton
	availability, filter, sort *walk.ComboBox
	search, pageInfo           *walk.LineEdit
	table                      *walk.TableView
	model                      *appModel

	all        []App // the whole purchase history, newest first
	loaded     bool  // all holds a fetched list
	loading    bool  // a fetch from Apple is running
	autoLoaded bool  // the first-visit load has been tried this session
	page       int
	// availabilityErr is why the availability check failed (nil when it worked);
	// the apps then have unknown availability.
	availabilityErr error
	whenLoaded      []func() // actions waiting for the running load (Search, Copy all apps...)
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
				Label{Text: "Availab&ility filter:"},
				ComboBox{AssignTo: &p.availability, Model: availabilityChoices, CurrentIndex: 0,
					Accessibility: accessible("Availab&ility filter:"), OnCurrentIndexChanged: g.myAppsViewChanged},
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
			g.listButtons(&p.table, p.model, g.purchaseFilter, &p.download, &p.copy, &p.info,
				PushButton{AssignTo: &p.copyAll, Text: "Copy &all apps", OnClicked: g.copyAllApps},
				PushButton{AssignTo: &p.export, Text: "&Export to JSON", OnClicked: g.exportApps}),
		},
	}
}

// setupMyApps starts the first-visit load when focus first enters one of the
// page's controls. Only focus gains count, so arrowing past the tab, Ctrl+4 and
// switching to another program never start it.
func (g *gui) setupMyApps() {
	p := &g.purchases
	widgets := []walk.Widget{p.load, p.previous, p.next, p.availability, p.filter, p.pageInfo, p.search, p.find, p.sort, p.table, p.download, p.copy, p.info, p.copyAll, p.export}
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
		// E.g. Alt+L focused Load, which started the first-visit load, then
		// clicked it; or Copy all apps during the first-visit load: run it after.
		if then != nil {
			p.whenLoaded = append(p.whenLoaded, then)
			g.setStatus("Your apps are still loading; this will continue when they're ready.")
		}
		return
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
	type loadResult struct {
		apps            []App
		availabilityErr error
	}
	var checked, batches atomic.Int32 // availability progress, for the status bar
	if then != nil {
		p.whenLoaded = []func(){then}
	}
	g.runTask("load all my apps", "Loading all your apps from Apple.",
		func(ctx context.Context, b *backend) (any, error) {
			apps, country, err := b.ownedAppsAll()
			if err != nil {
				return nil, err
			}
			// Then check which apps are still on the account's App Store; a
			// failure there doesn't stop the apps from loading.
			g.app.Synchronize(func() { g.busyStatus = "Checking which of your apps are still on the App Store." })
			var availabilityErr error
			if country == "" {
				availabilityErr = errors.New("the account's App Store country is unknown")
			} else {
				availabilityErr = checkAvailability(ctx, http.DefaultClient, country, apps, func(done, total int) {
					checked.Store(int32(done))
					batches.Store(int32(total))
				})
			}
			return loadResult{apps, availabilityErr}, nil
		},
		func(result any, err error) {
			pending := p.whenLoaded
			p.whenLoaded = nil
			if err != nil {
				if auto {
					_ = p.pageInfo.SetText("Not loaded: " + errorText(err))
					g.setStatus("Could not load your apps: " + errorText(err))
				} else {
					g.error("Could not load your apps", errorText(err))
				}
				return
			}
			r := result.(loadResult)
			p.all, p.availabilityErr, p.loaded, p.page = r.apps, r.availabilityErr, true, 1
			g.showMyApps(!auto)
			status := fmt.Sprintf("%d apps loaded.", len(p.all))
			if r.availabilityErr != nil {
				g.log("availability check failed: " + r.availabilityErr.Error())
				status += " Couldn't check which are still on the App Store."
			} else {
				unavailable := len(filterApps(p.all, 2, "", ""))
				g.log(fmt.Sprintf("availability: %d available, %d unavailable", len(p.all)-unavailable, unavailable))
				status = fmt.Sprintf("%d apps loaded, %d no longer on the App Store.", len(p.all), unavailable)
			}
			g.setStatus(status)
			for _, f := range pending {
				f()
			}
		})
	if g.busy { // the task started
		p.loading = true
		g.afterTask = func() { // also after Escape
			p.loading = false
			if g.cancelled {
				p.whenLoaded = nil
			}
		}
		g.busyProgress = func() string {
			if total := batches.Load(); total > 0 {
				return fmt.Sprintf("%d of %d", checked.Load(), total)
			}
			return ""
		}
	} else {
		p.whenLoaded = nil
	}
}

// showMyApps fills the list with the current page of the filtered, searched and
// sorted apps and updates the Page field. focusList moves focus to the first app.
func (g *gui) showMyApps(focusList bool) {
	p := &g.purchases
	view := g.myAppsView()
	apps, current, pages := pageOf(view, p.page, appsPerPage)
	p.page = current
	p.model.apps = apps
	p.model.PublishRowsReset()
	narrowed := p.availability.CurrentIndex() > 0 || g.purchaseFilter() != "" || p.search.Text() != ""
	info := pageInfoText(current, pages, len(view), len(p.all), narrowed)
	if p.availability.CurrentIndex() > 0 && p.availabilityErr != nil {
		info = "Couldn't check which apps are still on the App Store: " + errorText(p.availabilityErr) +
			" Press Load (Alt+L) to try again."
	}
	_ = p.pageInfo.SetText(info)
	if len(apps) > 0 {
		_ = p.table.SetCurrentIndex(0)
		if focusList {
			_ = p.table.SetFocus()
		}
	}
}

// myAppsView returns every app matching the availability and platform filters
// and Search my apps, in the Sort by order: all pages of what the list shows.
func (g *gui) myAppsView() []App {
	p := &g.purchases
	view := filterApps(p.all, p.availability.CurrentIndex(), g.purchaseFilter(), p.search.Text()) // a new slice, safe to sort
	sortApps(view, p.sort.CurrentIndex())
	return view
}

// withMyApps runs f once the apps are loaded, loading them first if needed.
func (g *gui) withMyApps(f func()) {
	if !g.purchases.loaded {
		g.fetchMyApps(false, f)
		return
	}
	f()
}

// copyAllApps puts every app in the current view on the clipboard, one line each.
func (g *gui) copyAllApps() {
	g.withMyApps(func() {
		view := g.myAppsView()
		if len(view) == 0 {
			g.info("Nothing to copy", "No apps match the current filters and search.")
			return
		}
		if err := walk.Clipboard().SetText(exportLines(view)); err != nil {
			g.error("Could not copy", "Windows didn't accept the text on the clipboard. Try again.")
			return
		}
		text := fmt.Sprintf("Copied %d apps to the clipboard.", len(view))
		g.setStatus(text)
		g.info("Apps copied", text)
	})
}

// exportApps saves every app in the current view, with a description of the
// view, as a JSON file the user chooses.
func (g *gui) exportApps() {
	g.withMyApps(func() {
		p := &g.purchases
		view := g.myAppsView()
		if len(view) == 0 {
			g.info("Nothing to export", "No apps match the current filters and search.")
			return
		}
		dlg := walk.FileDialog{
			Title:          "Export apps to JSON",
			Filter:         "JSON files (*.json)|*.json",
			InitialDirPath: strings.TrimSpace(g.download.output.Text()),
			FilePath:       "My apps.json",
			Flags:          win.OFN_OVERWRITEPROMPT, // walk doesn't ask before replacing a file
		}
		if ok, _ := dlg.ShowSave(g.mw); !ok {
			return
		}
		path := dlg.FilePath
		if !strings.EqualFold(filepath.Ext(path), ".json") {
			path += ".json"
			// The dialog only checked the name as typed, without .json.
			if _, err := os.Stat(path); err == nil && walk.MsgBox(g.mw, "Replace file",
				filepath.Base(path)+" already exists. Replace it?", walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
				return
			}
		}
		data, err := exportJSON(exportInfo{exported: time.Now(), total: len(p.all), availability: p.availability.Text(), search: p.search.Text(),
			platform: g.purchaseFilter(), sortedBy: p.sort.Text()}, view)
		if err == nil {
			err = os.WriteFile(path, data, 0o644)
		}
		if err != nil {
			g.log("error: " + err.Error())
			g.error("Could not save", "The file couldn't be saved to "+path+". Choose another folder, "+
				"or close the file if another program has it open.")
			return
		}
		text := fmt.Sprintf("Saved %d apps to %s.", len(view), path)
		g.log("> export " + fmt.Sprintf("%d apps to %s", len(view), path))
		g.setStatus(text)
		g.info("Apps exported", text)
	})
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

// myAppsViewChanged applies a new availability or platform filter or sort order instantly; focus
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

// myAppsComboEnter is Enter in the availability or platform filter or Sort by: go to the list.
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
