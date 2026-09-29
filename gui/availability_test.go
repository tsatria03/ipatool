package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// fakeLookup serves the lookup service for the given available IDs and counts requests.
func fakeLookup(t *testing.T, available map[string]bool, requests *atomic.Int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if got := r.URL.Query().Get("country"); got != "us" {
			t.Errorf("country = %q, want us", got)
		}
		ids := strings.Split(r.URL.Query().Get("id"), ",")
		if len(ids) > lookupBatch {
			t.Errorf("%d IDs in one request, want at most %d", len(ids), lookupBatch)
		}
		var results []map[string]any
		for _, id := range ids {
			if available[id] {
				var n int64
				fmt.Sscan(id, &n)
				results = append(results, map[string]any{"trackId": n, "artistName": "Developer " + id,
					"fileSizeBytes": fmt.Sprint(n * 1000)})
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"resultCount": len(results), "results": results})
	}))
}

func TestCheckAvailability(t *testing.T) {
	apps := sampleApps(320) // IDs 1..320: three requests of 150, 150 and 20
	available := map[string]bool{}
	for i := 1; i <= 320; i++ {
		if i%4 != 0 { // every fourth app has left the store
			available[fmt.Sprint(i)] = true
		}
	}
	var requests atomic.Int32
	server := fakeLookup(t, available, &requests)
	defer server.Close()
	old := lookupURL
	lookupURL = server.URL
	defer func() { lookupURL = old }()

	progressCalls := atomic.Int32{}
	if err := checkAvailability(context.Background(), server.Client(), "US", apps, func(done, total int) {
		progressCalls.Add(1)
		if total != 3 || done < 1 || done > 3 {
			t.Errorf("progress %d of %d", done, total)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 3 || progressCalls.Load() != 3 {
		t.Errorf("%d requests and %d progress calls, want 3 and 3", requests.Load(), progressCalls.Load())
	}
	if got := len(filterApps(apps, 1, "", "")); got != 240 {
		t.Errorf("Available shows %d apps, want 240", got)
	}
	unavailable := filterApps(apps, 2, "", "")
	if len(unavailable) != 80 || unavailable[0].ID != 4 {
		t.Errorf("Unavailable shows %d apps starting with %d, want 80 starting with 4", len(unavailable), unavailable[0].ID)
	}
	if got := len(filterApps(apps, 0, "", "")); got != 320 {
		t.Errorf("All apps shows %d, want 320", got)
	}
	// Available apps keep their store details; apps that left the store have none.
	if d := apps[4].Details; d == nil || d.Developer != "Developer 5" || d.sizeBytes() != 5000 {
		t.Errorf("app 5 details = %+v", d)
	}
	if apps[3].Details != nil {
		t.Errorf("app 4 left the store but has details %+v", apps[3].Details)
	}
	// Combined with search: "App 4" matches apps 4 and 40-49; of those, 4, 40, 44 and 48 left.
	if got := len(filterApps(apps, 2, "", "App 4")); got != 4 {
		t.Errorf("Unavailable + search shows %d, want 4", got)
	}
}

func TestCheckAvailabilityFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "busy", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	old := lookupURL
	lookupURL = server.URL
	defer func() { lookupURL = old }()

	apps := sampleApps(10)
	if err := checkAvailability(context.Background(), server.Client(), "US", apps, nil); err == nil {
		t.Fatal("expected an error")
	}
	// A failed check must never make apps look unavailable.
	for _, app := range apps {
		if app.Availability != availabilityUnknown {
			t.Fatalf("app %d is %v after a failed check, want unknown", app.ID, app.Availability)
		}
	}
	if got := len(filterApps(apps, 2, "", "")); got != 0 {
		t.Errorf("Unavailable shows %d apps after a failed check, want 0", got)
	}
}

func TestParseStoreDetailsOddField(t *testing.T) {
	// A field of an unexpected type (here a number instead of text) loses only that field.
	d := parseStoreDetails(json.RawMessage(`{"trackId": 7, "artistName": 42, "trackName": "Dice Only", "averageUserRating": 4.5}`))
	if d.ID != 7 || d.Name != "Dice Only" || d.Rating != 4.5 || d.Developer != "" {
		t.Errorf("details = %+v", d)
	}
}

func TestLookupDetails(t *testing.T) {
	var requests atomic.Int32
	server := fakeLookup(t, map[string]bool{"5": true}, &requests)
	defer server.Close()
	old := lookupURL
	lookupURL = server.URL
	defer func() { lookupURL = old }()

	d, err := lookupDetails(context.Background(), server.Client(), "US", 5)
	if err != nil || d == nil || d.ID != 5 {
		t.Fatalf("lookupDetails(5) = %+v, %v", d, err)
	}
	if d, err := lookupDetails(context.Background(), server.Client(), "US", 6); err != nil || d != nil {
		t.Errorf("lookupDetails(6) = %+v, %v; want nil, nil (not on the store)", d, err)
	}
	if requests.Load() != 2 {
		t.Errorf("%d requests, want 2", requests.Load())
	}
}

func TestExportAvailability(t *testing.T) {
	apps := []App{
		{Name: "Gone", ID: 1, Availability: availabilityUnavailable},
		{Name: "Here", ID: 2, Availability: availabilityAvailable},
		{Name: "Unknown", ID: 3},
	}
	data, err := exportJSON(exportInfo{total: 3, availability: "Unavailable"}, apps)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Availability string
		Apps         []map[string]any
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Availability != "Unavailable" {
		t.Errorf("heading availability = %q", doc.Availability)
	}
	if doc.Apps[0]["available"] != false || doc.Apps[1]["available"] != true {
		t.Errorf("available fields: %v, %v", doc.Apps[0]["available"], doc.Apps[1]["available"])
	}
	if _, has := doc.Apps[2]["available"]; has {
		t.Errorf("unchecked app should have no available field: %v", doc.Apps[2])
	}
	// Copy all apps never mentions availability.
	if strings.Contains(exportLines(apps), "vailab") {
		t.Errorf("copied text mentions availability:\n%s", exportLines(apps))
	}
}
