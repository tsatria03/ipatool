package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
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

// lookupDetails asks the lookup service about one app in country (such as "US");
// nil means the App Store doesn't have it.
func lookupDetails(ctx context.Context, client *http.Client, country string, id int64) (*storeDetails, error) {
	found, err := lookupIDs(ctx, client, country, []string{strconv.FormatInt(id, 10)})
	if err != nil || len(found) == 0 {
		return nil, err
	}
	return found[0], nil
}
