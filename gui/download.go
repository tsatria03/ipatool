package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tailscale/walk"
	. "github.com/tailscale/walk/declarative"
)

type downloadPage struct {
	app, version, output, result *walk.LineEdit
	platform                     *walk.ComboBox
	purchase                     *walk.CheckBox
}

func (g *gui) downloadTab() TabPage {
	d := &g.download
	output := g.settings.Output
	if output == "" {
		if home, err := os.UserHomeDir(); err == nil {
			output = filepath.Join(home, "Downloads")
		}
	}

	rows := []Widget{}
	rows = append(rows, labeled("&App (bundle ID or app ID):",
		LineEdit{AssignTo: &d.app, Accessibility: accessible("&App (bundle ID or app ID):")}, nil)...)
	rows = append(rows, labeled("&Platform:",
		ComboBox{AssignTo: &d.platform, Model: platforms, CurrentIndex: 0, Accessibility: accessible("&Platform:")}, nil)...)
	rows = append(rows, labeled("&Version ID (leave blank for latest):",
		LineEdit{AssignTo: &d.version, Accessibility: accessible("&Version ID (leave blank for latest):")},
		PushButton{Text: "C&hoose older version...", OnClicked: g.chooseVersion})...)
	rows = append(rows, labeled("Save to &folder:",
		LineEdit{AssignTo: &d.output, Text: output, Accessibility: accessible("Save to &folder:")},
		PushButton{Text: "&Browse...", OnClicked: g.browseOutput})...)

	return TabPage{
		Title:  "Download",
		Layout: VBox{},
		Children: []Widget{
			Composite{Layout: Grid{Columns: 3, MarginsZero: true}, Children: rows},
			CheckBox{AssignTo: &d.purchase, Text: "&Get a free license if needed (free apps only)", Checked: true},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				PushButton{Text: "&Download", OnClicked: g.startDownload},
				PushButton{Text: "&Open folder", OnClicked: g.openOutputFolder},
				HSpacer{},
			}},
			Composite{Layout: Grid{Columns: 3, MarginsZero: true}, Children: labeled("&Result:",
				LineEdit{AssignTo: &d.result, ReadOnly: true, Accessibility: accessible("&Result:")}, nil)},
			VSpacer{},
		},
	}
}

// appArgs returns -i for numeric app IDs and -b for bundle IDs.
func (g *gui) appArgs() ([]string, bool) {
	target := strings.TrimSpace(g.download.app.Text())
	if target == "" {
		g.error("Missing app", "Enter a bundle ID (like com.example.app) or a numeric app ID.")
		_ = g.download.app.SetFocus()
		return nil, false
	}
	if strings.IndexFunc(target, func(r rune) bool { return r < '0' || r > '9' }) == -1 {
		return []string{"-i", target}, true
	}
	return []string{"-b", target}, true
}

func (g *gui) startDownload() {
	d := &g.download
	app, ok := g.appArgs()
	if !ok {
		return
	}
	folder := strings.TrimSpace(d.output.Text())
	if info, err := os.Stat(folder); err != nil || !info.IsDir() {
		g.error("Folder not found", "Choose an existing folder to save to.")
		return
	}
	g.persist()

	args := append([]string{"download"}, app...)
	args = append(args, "--platform", d.platform.Text(), "-o", folder)
	if v := strings.TrimSpace(d.version.Text()); v != "" {
		args = append(args, "--external-version-id", v)
	}
	if d.purchase.Checked() {
		args = append(args, "--purchase")
	}
	_ = d.result.SetText("Downloading...")
	g.run(args, "Downloading. Large apps can take a while.", true, func(result Result) {
		if !result.OK() {
			_ = d.result.SetText("Download failed: " + result.Error())
			g.error("Download failed", result.Error())
			return
		}
		extra := ""
		if result.Str("purchased") == "true" {
			extra = " A free license was added to your account."
		}
		_ = d.result.SetText(fmt.Sprintf("Saved to %s.%s", result.Str("output"), extra))
		g.info("Download finished", d.result.Text())
	})
}

func (g *gui) browseOutput() {
	dlg := walk.FileDialog{Title: "Choose where to save apps", InitialDirPath: g.download.output.Text()}
	if ok, _ := dlg.ShowBrowseFolder(g.mw); ok {
		_ = g.download.output.SetText(dlg.FilePath)
	}
}

func (g *gui) openOutputFolder() {
	folder := strings.TrimSpace(g.download.output.Text())
	if info, err := os.Stat(folder); err == nil && info.IsDir() {
		_ = exec.Command("explorer", folder).Start()
	}
}

// ----- choosing an older version -----------------------------------------------

func (g *gui) chooseVersion() {
	app, ok := g.appArgs()
	if !ok {
		return
	}
	appArgs := append(app, "--platform", g.download.platform.Text())

	var dlg *walk.Dialog
	var list *walk.ListBox
	var useButton, cancelButton *walk.PushButton
	var ids, items []string
	closed := false

	selected := func() int {
		i := list.CurrentIndex()
		if len(ids) == 0 || i < 0 || i >= len(ids) {
			return -1
		}
		return i
	}
	setItems := func() {
		current := list.CurrentIndex()
		_ = list.SetModel(slices.Clone(items))
		if current >= 0 {
			_ = list.SetCurrentIndex(current)
		}
	}

	// lookup fetches version numbers one at a time, then returns focus to the list.
	var lookup func(pending []int)
	lookup = func(pending []int) {
		if closed {
			return
		}
		if len(pending) == 0 {
			_ = list.SetFocus()
			return
		}
		index := pending[0]
		args := append(append([]string{"get-version-metadata"}, appArgs...), "--external-version-id", ids[index])
		g.run(args, "Looking up version "+ids[index]+".", true, func(result Result) {
			if closed {
				return
			}
			label := "lookup failed"
			if result.OK() {
				label = result.Str("displayVersion")
			}
			items[index] = fmt.Sprintf("Version %s, ID %s", label, ids[index])
			setItems()
			lookup(pending[1:])
		})
	}
	lookupNext := func() {
		var pending []int
		for i, item := range items {
			if item == ids[i] && len(pending) < 10 {
				pending = append(pending, i)
			}
		}
		lookup(pending)
	}
	useSelected := func() {
		if i := selected(); i >= 0 {
			_ = g.download.version.SetText(ids[i])
			dlg.Accept()
		}
	}

	err := Dialog{
		AssignTo:      &dlg,
		Title:         "Choose a version",
		DefaultButton: &useButton,
		CancelButton:  &cancelButton,
		MinSize:       Size{Width: 460, Height: 480},
		Layout:        VBox{},
		Children: []Widget{
			Label{Text: "Newest first. Apple only gives out version IDs. Select one and press Look up to see its version number."},
			Label{Text: "&Versions:"},
			ListBox{AssignTo: &list, Model: []string{"Loading..."}, Accessibility: accessible("&Versions:"),
				OnItemActivated: useSelected},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				PushButton{Text: "&Look up", OnClicked: func() {
					if i := selected(); i >= 0 {
						lookup([]int{i})
					}
				}},
				PushButton{Text: "Look up &next 10", OnClicked: lookupNext},
				HSpacer{},
				PushButton{AssignTo: &useButton, Text: "&Use selected", OnClicked: useSelected},
				PushButton{AssignTo: &cancelButton, Text: "Cancel", OnClicked: func() { dlg.Cancel() }},
			}},
		},
	}.Create(g.mw)
	if err != nil {
		return
	}
	assignControlIDs(dlg.Handle())
	_ = list.SetFocus()

	g.run(append([]string{"list-versions"}, appArgs...), "Loading version list.", true, func(result Result) {
		if closed {
			return
		}
		if !result.OK() {
			g.error("Could not list versions", result.Error())
			dlg.Cancel()
			return
		}
		ids = result.Strings("externalVersionIdentifiers")
		slices.Reverse(ids)
		items = slices.Clone(ids)
		if len(items) == 0 {
			_ = list.SetModel([]string{"No versions found."})
		} else {
			_ = list.SetModel(slices.Clone(items))
			_ = list.SetCurrentIndex(0)
		}
		_ = list.SetFocus()
	})

	dlg.Run()
	closed = true
}
