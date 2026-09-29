package main

import (
	"strings"
	"testing"
	"time"
)

func TestDetailsTextAvailable(t *testing.T) {
	app := App{ID: 1445804669, BundleID: "com.example.dice", Name: "Dice", Version: "1.0",
		Platforms: []string{"iphone", "ipad"}, PurchaseDate: time.Date(2019, 3, 1, 12, 0, 0, 0, time.UTC),
		Details: &storeDetails{Name: "Ready to Roll - RPG Dice", Developer: "Example Games", Seller: "Example Games",
			Version: "2.1", Size: "73400000", Price: "$1.99", Category: "Games", AgeRating: "4+", MinimumOS: "12.0",
			Rating: 4.5, Ratings: 1234, Languages: []string{"EN", "FR"}, Released: "2019-01-20T08:00:00Z",
			Updated: "2024-05-02T17:30:00Z", URL: "https://apps.apple.com/us/app/id1445804669",
			ReleaseNotes: "Bug fixes.\nFaster rolls.", Description: "Roll dice.\r\nMany dice."}}
	got := detailsText(app)
	want := strings.Join([]string{
		"Name: Ready to Roll - RPG Dice",
		"Developer: Example Games",
		"Version: 2.1",
		"Size: 73 MB",
		"Price: $1.99",
		"Category: Games",
		"Age rating: 4+",
		"Minimum OS version: 12.0",
		"Rating: 4.5 out of 5, 1234 ratings",
		"Languages: EN, FR",
		"Released: January 20, 2019",
		"Updated: May 2, 2024",
		"Platforms: iphone, ipad",
		"Purchased: " + app.PurchaseDate.Local().Format("January 2, 2006"),
		"Bundle ID: com.example.dice",
		"App ID: 1445804669",
		"App Store link: https://apps.apple.com/us/app/id1445804669",
		"",
		"What's new:",
		"Bug fixes.",
		"Faster rolls.",
		"",
		"Description:",
		"Roll dice.",
		"Many dice.",
	}, "\r\n")
	if got != want {
		t.Errorf("detailsText:\n%s\n\nwant:\n%s", got, want)
	}
}

func TestDetailsTextSellerAndNoRatings(t *testing.T) {
	got := detailsText(App{Name: "X", ID: 5, Details: &storeDetails{Developer: "Maker", Seller: "Maker LLC"}})
	if !strings.Contains(got, "Seller: Maker LLC") {
		t.Errorf("a different seller should be shown:\n%s", got)
	}
	if strings.Contains(got, "Rating") {
		t.Errorf("an app without ratings shouldn't show a rating:\n%s", got)
	}
	got = detailsText(App{Name: "X", ID: 5, Details: &storeDetails{Rating: 4.84615, Ratings: 13}})
	if !strings.Contains(got, "Rating: 4.8 out of 5, 13 ratings") {
		t.Errorf("rating should be rounded to one decimal:\n%s", got)
	}
}

func TestDetailsTextRemovedApp(t *testing.T) {
	got := detailsText(App{ID: 1391020357, BundleID: "com.example.dusk", Name: "Dusk", Version: "1.2",
		Platforms: []string{"iphone"}, Availability: availabilityUnavailable})
	want := strings.Join([]string{
		"This app is no longer on the App Store, so Apple has no details for it.",
		"Name: Dusk",
		"Version: 1.2",
		"Platforms: iphone",
		"Bundle ID: com.example.dusk",
		"App ID: 1391020357",
	}, "\r\n")
	if got != want {
		t.Errorf("detailsText:\n%s\n\nwant:\n%s", got, want)
	}
}
