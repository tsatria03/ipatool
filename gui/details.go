package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tailscale/walk"
	. "github.com/tailscale/walk/declarative"
	"github.com/tailscale/win"
)

// storeDetails is what Apple's public lookup service tells about an app that is
// on the App Store. My apps gets it for every available app during the
// availability check; Global search looks up one app when asked.
type storeDetails struct {
	ID           int64    `json:"trackId"`
	Name         string   `json:"trackName"`
	Developer    string   `json:"artistName"`
	Seller       string   `json:"sellerName"`
	Category     string   `json:"primaryGenreName"`
	AgeRating    string   `json:"contentAdvisoryRating"`
	MinimumOS    string   `json:"minimumOsVersion"`
	Price        string   `json:"formattedPrice"`
	Size         string   `json:"fileSizeBytes"`
	Version      string   `json:"version"`
	Released     string   `json:"releaseDate"`               // first release, like "2019-03-01T08:00:00Z"
	Updated      string   `json:"currentVersionReleaseDate"` // the current version's release
	Rating       float64  `json:"averageUserRating"`
	Ratings      int64    `json:"userRatingCount"`
	Languages    []string `json:"languageCodesISO2A"`
	Description  string   `json:"description"`
	ReleaseNotes string   `json:"releaseNotes"`
	URL          string   `json:"trackViewUrl"`
}

// parseStoreDetails reads one lookup result. A field of an unexpected type is
// left empty instead of losing the whole app (or the whole batch).
func parseStoreDetails(raw json.RawMessage) *storeDetails {
	var d storeDetails
	_ = json.Unmarshal(raw, &d) // on type errors, Unmarshal still fills the other fields
	return &d
}

// sizeBytes is the app's size in bytes, or 0 when Apple didn't give one.
func (d *storeDetails) sizeBytes() int64 {
	size, _ := strconv.ParseInt(d.Size, 10, 64)
	return size
}

// detailsText is what App info shows: one fact per line, then What's new and
// the description, with Windows line breaks. Without store details (the app
// left the App Store) it says so and shows what the list knows.
func detailsText(app App) string {
	var lines []string
	add := func(label, value string) {
		if value = strings.TrimSpace(value); value != "" {
			lines = append(lines, label+": "+value)
		}
	}
	d := app.Details
	if d == nil {
		lines = append(lines, "This app is no longer on the App Store, so Apple has no details for it.")
		d = &storeDetails{}
	}
	add("Name", firstNonEmpty(d.Name, app.Name))
	add("Developer", d.Developer)
	if !strings.EqualFold(d.Seller, d.Developer) {
		add("Seller", d.Seller)
	}
	add("Version", firstNonEmpty(d.Version, app.Version))
	add("Size", formatSize(max(d.sizeBytes(), app.Size)))
	add("Price", d.Price)
	add("Category", d.Category)
	add("Age rating", d.AgeRating)
	add("Minimum OS version", d.MinimumOS)
	if d.Ratings > 0 {
		rating := strconv.FormatFloat(math.Round(d.Rating*10)/10, 'f', -1, 64) // 4.84615 -> 4.8, like the App Store
		add("Rating", fmt.Sprintf("%s out of 5, %d ratings", rating, d.Ratings))
	}
	add("Languages", strings.Join(d.Languages, ", "))
	add("Released", longDate(d.Released))
	add("Updated", longDate(d.Updated))
	add("Platforms", strings.Join(app.Platforms, ", "))
	if !app.PurchaseDate.IsZero() {
		add("Purchased", app.PurchaseDate.Local().Format("January 2, 2006"))
	}
	add("Bundle ID", app.BundleID)
	if app.ID != 0 {
		add("App ID", strconv.FormatInt(app.ID, 10))
	}
	add("App Store link", d.URL)
	for _, block := range []struct{ title, text string }{{"What's new", d.ReleaseNotes}, {"Description", d.Description}} {
		if text := strings.TrimSpace(block.text); text != "" {
			lines = append(lines, "", block.title+":", text)
		}
	}
	text := strings.Join(lines, "\n")
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	return strings.ReplaceAll(text, "\n", "\r\n")
}

// longDate turns a lookup date like "2019-03-01T08:00:00Z" into "March 1, 2019";
// anything else is returned as it is.
func longDate(value string) string {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.Format("January 2, 2006")
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// lookupDetails asks the lookup service about one app in country (such as "US");
// nil means the App Store doesn't have it.
func lookupDetails(ctx context.Context, client *http.Client, country string, id int64) (*storeDetails, error) {
	found, err := lookupIDs(ctx, client, country, []string{strconv.FormatInt(id, 10)})
	if err != nil || len(found) == 0 {
		return nil, err
	}
	return found[0], nil
}

// ----- the App info window -----------------------------------------------------

// showAppInfo is the App info button and Alt+Enter in an app list. My apps
// already has the details of its available apps; other apps (Global search, or
// when the availability check failed) are looked up once per session.
func (g *gui) showAppInfo(table *walk.TableView, model *appModel) {
	app, ok := selectedApp(table, model)
	if !ok {
		g.info("Nothing selected", "Select an app in the list first.")
		return
	}
	if app.Details != nil || app.Availability == availabilityUnavailable {
		g.openAppInfo(table, app)
		return
	}
	if d, ok := g.detailsCache[app.ID]; ok {
		app.Details = d
		g.openAppInfo(table, app)
		return
	}
	g.runTask(fmt.Sprintf("look up app info of %s (%d)", app.Name, app.ID), "Getting app info.",
		func(ctx context.Context, b *backend) (any, error) { return b.appDetails(ctx, app.ID) },
		func(result any, err error) {
			if err != nil {
				g.error("Could not get app info", errorText(err))
				return
			}
			if g.detailsCache == nil {
				g.detailsCache = map[int64]*storeDetails{}
			}
			app.Details, _ = result.(*storeDetails)
			g.detailsCache[app.ID] = app.Details
			g.openAppInfo(table, app)
		})
}

// openAppInfo shows detailsText in a text window; focus then returns to the list.
func (g *gui) openAppInfo(table *walk.TableView, app App) {
	g.textWindow("App info, "+app.Name, "&Details:", detailsText(app), "Copied the app info of "+app.Name+".", table)
}

// textWindow shows text in a read-only text box that screen readers read line
// by line, with Copy all and Close (or Escape) buttons. copied is the status
// bar text after Copy all. Focus then returns to back, or where it was if back
// is nil.
func (g *gui) textWindow(title, label, text, copied string, back walk.Widget) {
	previous := win.GetFocus()
	var dlg *walk.Dialog
	var box *walk.TextEdit
	var closeButton *walk.PushButton
	err := Dialog{
		AssignTo:     &dlg,
		Title:        title,
		CancelButton: &closeButton,
		MinSize:      Size{Width: 600, Height: 500},
		Layout:       VBox{},
		Children: []Widget{
			Label{Text: label},
			TextEdit{AssignTo: &box, ReadOnly: true, VScroll: true, Text: text, Accessibility: accessible(label)},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				PushButton{Text: "&Copy all", OnClicked: func() {
					if err := walk.Clipboard().SetText(box.Text()); err != nil {
						g.error("Could not copy", "Windows didn't accept the text on the clipboard. Try again.")
						return
					}
					g.setStatus(copied)
				}},
				HSpacer{},
				PushButton{AssignTo: &closeButton, Text: "Close", OnClicked: func() { dlg.Cancel() }},
			}},
		},
	}.Create(g.mw)
	if err != nil {
		return
	}
	assignControlIDs(dlg.Handle())
	_ = box.SetFocus()
	box.SetTextSelection(0, 0) // start at the top, nothing selected
	dlg.Run()
	if back != nil {
		_ = back.SetFocus()
	} else if previous != 0 {
		win.SetFocus(previous)
	}
}
