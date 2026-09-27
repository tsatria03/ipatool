package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

// Availability: whether an owned app is still on the account's App Store. The
// purchase history keeps apps after they leave the store, so My apps asks
// Apple's public lookup service which of them it still sells.

type availability int8

const (
	availabilityUnknown availability = iota // not checked, or the check failed
	availabilityAvailable
	availabilityUnavailable
)

// availabilityChoices are the "Availability filter" options; index 0 shows all apps.
var availabilityChoices = []string{"All apps", "Available", "Unavailable"}

// lookupURL is Apple's public lookup service (a variable so tests can replace it).
var lookupURL = "https://itunes.apple.com/lookup"

const (
	lookupBatch   = 150 // app IDs per lookup request
	lookupWorkers = 4   // requests in flight at once; each takes about a second
)

// checkAvailability marks every app available or unavailable in the given
// country (such as "US"). An app is unavailable when the lookup service doesn't
// return it. If any request fails, every app stays unknown and the error is
// returned, so a failed check never makes apps look unavailable. progress, if
// set, is called (from other goroutines) after each request.
func checkAvailability(ctx context.Context, client *http.Client, country string, apps []App,
	progress func(done, total int)) error {
	var batches [][]string
	for start := 0; start < len(apps); start += lookupBatch {
		batch := apps[start:min(start+lookupBatch, len(apps))]
		ids := make([]string, len(batch))
		for i, app := range batch {
			ids[i] = strconv.FormatInt(app.ID, 10)
		}
		batches = append(batches, ids)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		mu       sync.Mutex
		found    = map[int64]bool{}
		firstErr error
		done     int
		wg       sync.WaitGroup
	)
	work := make(chan []string)
	for range min(lookupWorkers, len(batches)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ids := range work {
				ids := ids
				returned, err := lookupIDs(ctx, client, country, ids)
				mu.Lock()
				if err != nil && firstErr == nil {
					firstErr = err
					cancel() // stop the other requests
				}
				for _, id := range returned {
					found[id] = true
				}
				done++
				n := done
				mu.Unlock()
				if progress != nil {
					progress(n, len(batches))
				}
			}
		}()
	}
feed:
	for _, ids := range batches {
		select {
		case work <- ids:
		case <-ctx.Done():
			break feed
		}
	}
	close(work)
	wg.Wait()
	if firstErr == nil {
		firstErr = ctx.Err() // cancelled with Escape
	}
	if firstErr != nil {
		return firstErr
	}
	for i := range apps {
		if found[apps[i].ID] {
			apps[i].Availability = availabilityAvailable
		} else {
			apps[i].Availability = availabilityUnavailable
		}
	}
	return nil
}

// lookupIDs asks the lookup service about ids and returns the IDs it knows.
func lookupIDs(ctx context.Context, client *http.Client, country string, ids []string) ([]int64, error) {
	query := url.Values{"id": {strings.Join(ids, ",")}, "country": {strings.ToLower(country)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, lookupURL+"?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the App Store lookup service answered %s", res.Status)
	}
	var body struct {
		Results []struct {
			TrackID int64 `json:"trackId"`
		} `json:"results"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("invalid response from the App Store lookup service: %w", err)
	}
	returned := make([]int64, 0, len(body.Results))
	for _, r := range body.Results {
		returned = append(returned, r.TrackID)
	}
	return returned, nil
}

// matchesAvailability reports whether app belongs in the chosen
// availabilityChoices entry. Unknown apps only show under "All apps".
func matchesAvailability(app App, choice int) bool {
	switch choice {
	case 1:
		return app.Availability == availabilityAvailable
	case 2:
		return app.Availability == availabilityUnavailable
	}
	return true
}
