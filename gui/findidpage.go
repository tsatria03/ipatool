package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/tailscale/walk"
	. "github.com/tailscale/walk/declarative"
)

// The Find App ID window (Download page, Alt+N). findid.go does the searching;
// only the Internet Archive is asked, never Apple.

// detailsPerCheck is how many results get their saved page read at a time.
const detailsPerCheck = 10

// findAppID opens the Find App ID window. A chosen App ID goes into the App field.
func (g *gui) findAppID() {
	d := &g.download
	var dlg *walk.Dialog
	var nameBox, status *walk.LineEdit
	var list *walk.ListBox
	var searchButton, useButton, moreButton, cancelButton *walk.PushButton
	var hits []archiveHit
	var full bool
	closed := false
	running := false // a task of this window is running

	items := func() []string {
		lines := make([]string, len(hits))
		for i, h := range hits {
			lines[i] = h.line()
		}
		return lines
	}
	refresh := func() {
		current := list.CurrentIndex()
		_ = list.SetModel(items())
		if current >= 0 && current < len(hits) {
			_ = list.SetCurrentIndex(current)
		}
	}
	unchecked := func() []int {
		var pending []int
		for i, h := range hits {
			if !h.Checked {
				pending = append(pending, i)
			}
		}
		return pending
	}
	say := func(text string, focus bool) {
		_ = status.SetText(text)
		if focus {
			_ = status.SetFocus()
			status.SetTextSelection(0, -1)
		}
	}
	summary := func() string {
		text := fmt.Sprintf("%d apps found.", len(hits))
		if len(hits) == 1 {
			text = "1 app found."
		}
		if n := len(unchecked()); n > 0 {
			text += fmt.Sprintf(" %d not checked yet; press Check more to read their saved pages.", n)
		}
		if full {
			text += " Many saved addresses matched, so some apps may be missing; type more of the name to narrow it down."
		}
		return text
	}
	moreButtonState := func() {
		moreButton.SetEnabled(len(unchecked()) > 0)
		useButton.SetEnabled(len(hits) > 0)
	}

	// check reads the saved pages of the given results, two at a time, filling
	// in each line as its page arrives.
	// Each job carries its own copy of the result, made on the UI thread, since
	// hits is only touched there.
	type job struct {
		i int
		h archiveHit
	}
	jobs := func(pending []int) []job {
		out := make([]job, len(pending))
		for n, i := range pending {
			out[n] = job{i, hits[i]}
		}
		return out
	}
	check := func(ctx context.Context, client *http.Client, todo []job) {
		var wg sync.WaitGroup
		slots := make(chan struct{}, 2)
		for _, j := range todo {
			if ctx.Err() != nil {
				break
			}
			i, h := j.i, j.h
			wg.Add(1)
			slots <- struct{}{}
			go func(i int, h archiveHit) {
				defer wg.Done()
				defer func() { <-slots }()
				if err := pageDetails(ctx, client, &h); err != nil {
					if ctx.Err() != nil {
						return
					}
					h.Checked = true // the page couldn't be read: "details not available"
				}
				g.app.Synchronize(func() {
					if closed || i >= len(hits) || hits[i].ID != h.ID {
						return
					}
					hits[i] = h
					refresh()
				})
			}(i, h)
		}
		wg.Wait()
	}
	checkMore := func() {
		pending := unchecked()
		if len(pending) == 0 || g.busy {
			return
		}
		if len(pending) > detailsPerCheck {
			pending = pending[:detailsPerCheck]
		}
		say(fmt.Sprintf("Checking the saved pages of %d more apps.", len(pending)), false)
		todo := jobs(pending)
		running = true
		g.runLocalTask(fmt.Sprintf("check %d saved App Store pages", len(pending)), "Checking saved pages.",
			func(ctx context.Context) (any, error) {
				check(ctx, newArchiveClient(), todo)
				return nil, ctx.Err()
			},
			func(any, error) {
				running = false
				if closed {
					return
				}
				moreButtonState()
				say(summary(), false)
				_ = list.SetFocus()
			})
	}
	search := func() {
		name := strings.TrimSpace(nameBox.Text())
		if name == "" {
			say("Type the app's name, then press Search.", false)
			_ = nameBox.SetFocus()
			return
		}
		if g.busy {
			g.showBusy()
			return
		}
		hits, full = nil, false
		_ = list.SetModel([]string{})
		moreButtonState()
		say(fmt.Sprintf("Searching the Internet Archive for %s.", name), false)
		running = true
		g.runLocalTask("find the App ID of "+name+" in the Internet Archive", "Searching the Internet Archive.",
			func(ctx context.Context) (any, error) {
				client := newArchiveClient()
				result, err := searchArchive(ctx, client, name)
				if err != nil || len(result.Hits) == 0 {
					return result, err
				}
				g.app.Synchronize(func() {
					if closed {
						return
					}
					hits, full = result.Hits, result.Full
					refresh()
					_ = list.SetCurrentIndex(0)
					_ = list.SetFocus()
					moreButtonState()
					say(fmt.Sprintf("%d found. Checking their saved pages.", len(hits)), false)
				})
				var todo []job
				for i := 0; i < len(result.Hits) && i < detailsPerCheck; i++ {
					todo = append(todo, job{i, result.Hits[i]})
				}
				check(ctx, client, todo)
				return result, ctx.Err()
			},
			func(result any, err error) {
				running = false
				if closed {
					return
				}
				moreButtonState()
				switch {
				case errors.Is(err, errNoName):
					say("Type the app's name using letters or digits, then press Search.", true)
				case errors.Is(err, errArchiveBusy):
					say("The Internet Archive isn't answering right now. Try again in a minute.", true)
				case err != nil:
					say("Couldn't search the Internet Archive: "+err.Error(), true)
				case len(hits) == 0:
					say("Nothing found. Try fewer words (the start of the name), another spelling, or the English name. "+
						"Some apps were never saved by the Internet Archive.", true)
				default:
					say(summary(), false)
					_ = list.SetFocus()
				}
			})
	}
	useSelected := func() {
		i := list.CurrentIndex()
		if i < 0 || i >= len(hits) {
			return
		}
		_ = d.app.SetText(strconv.FormatInt(hits[i].ID, 10))
		_ = d.version.SetText("") // a version ID from another app wouldn't fit
		dlg.Accept()
	}
	// Enter runs the default button: on a result it uses it, elsewhere it searches.
	defaultAction := func() {
		if list.Focused() && len(hits) > 0 {
			useSelected()
			return
		}
		search()
	}

	err := Dialog{
		AssignTo:      &dlg,
		Title:         "Find App ID",
		DefaultButton: &searchButton,
		CancelButton:  &cancelButton,
		MinSize:       Size{Width: 560, Height: 480},
		Layout:        VBox{},
		Children: []Widget{
			Label{Text: "Searches the Internet Archive's saved App Store pages for apps that left the App Store. Apple isn't asked."},
			Composite{Layout: Grid{Columns: 3, MarginsZero: true}, Children: labeled("App &name:",
				LineEdit{AssignTo: &nameBox, Text: findIDPrefill(d.app.Text()), Accessibility: accessible("App &name:")},
				PushButton{AssignTo: &searchButton, Text: "&Search", OnClicked: defaultAction})},
			Label{Text: "&Results:"},
			ListBox{AssignTo: &list, Model: []string{}, Accessibility: accessible("&Results:"), OnItemActivated: useSelected},
			Composite{Layout: Grid{Columns: 3, MarginsZero: true}, Children: labeled("S&tatus:",
				LineEdit{AssignTo: &status, ReadOnly: true, Accessibility: accessible("S&tatus:")}, nil)},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				PushButton{AssignTo: &useButton, Text: "&Use selected", Enabled: false, OnClicked: useSelected},
				PushButton{AssignTo: &moreButton, Text: "Check &more", Enabled: false, OnClicked: checkMore},
				HSpacer{},
				PushButton{AssignTo: &cancelButton, Text: "Cancel", OnClicked: func() { dlg.Cancel() }},
			}},
		},
	}.Create(g.mw)
	if err != nil {
		return
	}
	assignControlIDs(dlg.Handle())
	_ = nameBox.SetFocus()
	nameBox.SetTextSelection(0, -1)

	if dlg.Run() == walk.DlgCmdOK {
		_ = d.app.SetFocus()
		d.app.SetTextSelection(0, -1)
	}
	closed = true
	if running {
		g.cancel() // closing the window stops its search; Escape here is the window's Cancel
	}
}

// findIDPrefill returns the App field's text when it looks like a name rather
// than an App ID or a bundle ID.
func findIDPrefill(app string) string {
	app = strings.TrimSpace(app)
	if app == "" {
		return ""
	}
	if _, err := strconv.ParseInt(app, 10, 64); err == nil {
		return ""
	}
	if strings.Contains(app, ".") && !strings.Contains(app, " ") {
		return "" // a bundle ID like com.example.app
	}
	return app
}
