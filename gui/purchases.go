package main

import (
	"fmt"
	"strconv"

	"github.com/tailscale/walk"
	. "github.com/tailscale/walk/declarative"
)

const allPlatforms = "all platforms"

type purchasesPage struct {
	load     *walk.PushButton
	filter   *walk.ComboBox
	pageInfo *walk.LineEdit
	table    *walk.TableView
	model    *appModel
	page     int
}

func (g *gui) purchasesTab() TabPage {
	p := &g.purchases
	p.model = &appModel{}
	p.page = 1

	return TabPage{
		Title:  "My apps",
		Layout: VBox{},
		Children: []Widget{
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				PushButton{AssignTo: &p.load, Text: "&Load", OnClicked: func() { g.loadPurchases(1) }},
				PushButton{Text: "&Previous page", OnClicked: func() { g.loadPurchases(max(1, p.page-1)) }},
				PushButton{Text: "&Next page", OnClicked: func() { g.loadPurchases(p.page + 1) }},
				Label{Text: "Pla&tform filter:"},
				ComboBox{AssignTo: &p.filter, Model: append([]string{allPlatforms}, platforms...), CurrentIndex: 0,
					Accessibility: accessible("Pla&tform filter:")},
				Label{Text: "Pa&ge:"},
				LineEdit{AssignTo: &p.pageInfo, ReadOnly: true, Text: "Not loaded", Accessibility: accessible("Pa&ge:")},
			}},
			Label{Text: "Apps you o&wn:"},
			TableView{AssignTo: &p.table, Model: p.model, Columns: appColumns(),
				OnItemActivated: func() { g.sendToDownload(p.table, p.model, g.purchaseFilter()) }},
			g.listButtons(&p.table, p.model, g.purchaseFilter),
		},
	}
}

func (g *gui) purchaseFilter() string {
	if f := g.purchases.filter.Text(); f != allPlatforms {
		return f
	}
	return ""
}

func (g *gui) loadPurchases(page int) {
	p := &g.purchases
	args := []string{"list-purchases", "--page", strconv.Itoa(page), "--max-results", "25"}
	if f := g.purchaseFilter(); f != "" {
		args = append(args, "--platform", f)
	}
	g.run(args, "Loading your apps.", true, func(result Result) {
		if !g.fillList(p.table, p.model, result, "Could not load your apps") {
			return
		}
		p.page = page
		info := fmt.Sprintf("Page %d", page)
		if total := result.Int("totalCount"); total > 0 {
			info += fmt.Sprintf(", %d apps in total", total)
		}
		_ = p.pageInfo.SetText(info)
	})
}
