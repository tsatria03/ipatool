package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
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
		PushButton{Text: "C&hoose older version", OnClicked: g.chooseVersion})...)
	rows = append(rows, labeled("Save to &folder:",
		LineEdit{AssignTo: &d.output, Text: output, Accessibility: accessible("Save to &folder:")},
		PushButton{Text: "&Browse", OnClicked: g.browseOutput})...)

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

// appTarget returns the app to work on: a bundle ID or a numeric app ID.
func (g *gui) appTarget() (string, bool) {
	target := strings.TrimSpace(g.download.app.Text())
	if target == "" {
		g.error("Missing app", "Enter a bundle ID (like com.example.app) or a numeric app ID.")
		_ = g.download.app.SetFocus()
		return "", false
	}
	return target, true
}

func (g *gui) startDownload() {
	d := &g.download
	target, ok := g.appTarget()
	if !ok {
		return
	}
	folder := strings.TrimSpace(d.output.Text())
	if info, err := os.Stat(folder); err != nil || !info.IsDir() {
		g.error("Folder not found", "Choose an existing folder to save to.")
		return
	}
	g.persist()

	req := downloadRequest{
		target:     target,
		platform:   d.platform.Text(),
		versionID:  strings.TrimSpace(d.version.Text()),
		output:     folder,
		getLicense: d.purchase.Checked(),
	}
	name := fmt.Sprintf("download %s (%s)", req.target, req.platform)
	if req.versionID != "" {
		name += ", version ID " + req.versionID
	}

	// The engine feeds the bar as the file arrives; nothing is drawn (io.Discard).
	bar := progressbar.NewOptions64(1, progressbar.OptionSetWriter(io.Discard), progressbar.OptionThrottle(time.Second))
	_ = d.result.SetText("Starting download...")
	g.runTask(name, "Downloading.",
		func(ctx context.Context, b *backend) (any, error) { return b.download(ctx, req, bar) },
		func(result any, err error) {
			if err != nil {
				_ = d.result.SetText("Download failed: " + errorText(err))
				g.error("Download failed", errorText(err))
				return
			}
			r := result.(downloadResult)
			extra := ""
			if r.purchased {
				extra = " A free license was added to your account."
			}
			_ = d.result.SetText(fmt.Sprintf("Saved to %s.%s", r.path, extra))
			g.info("Download finished", d.result.Text())
		})
	if g.busy { // the task started
		// Screen readers read the Result field, and Windows brings focus back to
		// it when the finished or failed message box closes.
		_ = d.result.SetFocus()
		g.busyProgress = func() string {
			text := downloadProgressText(bar.State())
			if text != "" {
				_ = d.result.SetText("Downloading: " + text)
			}
			return text
		}
	}
}

// downloadProgressText describes download progress, for example
// "45 percent, 120 MB downloaded", or "" before any data has arrived.
func downloadProgressText(s progressbar.State) string {
	if s.CurrentBytes <= 0 {
		return ""
	}
	return fmt.Sprintf("%d percent, %.0f MB downloaded", int(s.CurrentPercent*100), s.CurrentBytes/1e6)
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
	target, ok := g.appTarget()
	if !ok {
		return
	}
	platform := g.download.platform.Text()

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

	// lookup fetches the version numbers of the given list positions in one engine
	// task, filling in each line as it arrives, then returns focus to the list.
	lookup := func(pending []int) {
		if len(pending) == 0 {
			return
		}
		indexOf := map[string]int{}
		wanted := make([]string, len(pending))
		for n, i := range pending {
			wanted[n] = ids[i]
			indexOf[ids[i]] = i
		}
		done := 0
		name := fmt.Sprintf("look up %d version number(s) of %s (%s)", len(wanted), target, platform)
		g.runTask(name, "Looking up versions.",
			func(ctx context.Context, b *backend) (any, error) {
				return nil, b.versionDetails(ctx, target, platform, wanted, func(id, label string) {
					g.app.Synchronize(func() {
						done++
						if closed {
							return
						}
						items[indexOf[id]] = fmt.Sprintf("%s, ID %s", label, id)
						setItems()
					})
				})
			},
			func(_ any, err error) {
				if closed {
					return
				}
				if err != nil {
					g.error("Could not look up versions", errorText(err))
				}
				_ = list.SetFocus()
			})
		if g.busy {
			total := len(wanted)
			g.busyProgress = func() string { return fmt.Sprintf("%d of %d", done, total) }
		}
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
			ListBox{AssignTo: &list, Model: []string{"Loading"}, Accessibility: accessible("&Versions:"),
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

	g.runTask(fmt.Sprintf("list versions of %s (%s)", target, platform), "Loading version list.",
		func(ctx context.Context, b *backend) (any, error) { return b.listVersions(target, platform) },
		func(result any, err error) {
			if closed {
				return
			}
			if err != nil {
				g.error("Could not list versions", errorText(err))
				dlg.Cancel()
				return
			}
			ids = slices.Clone(result.([]string))
			slices.Reverse(ids)
			items = slices.Clone(ids)
			if len(items) == 0 {
				_ = list.SetModel([]string{"No versions found"})
			} else {
				_ = list.SetModel(slices.Clone(items))
				_ = list.SetCurrentIndex(0)
			}
			_ = list.SetFocus()
		})

	dlg.Run()
	closed = true
}
