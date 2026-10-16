---
name: plan-search-sizes
description: "Plan (2026-09-27, approved) - show app sizes in KB/MB/GB in Global search results only"
metadata:
  node_type: memory
  type: project
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-09-27T07:33:45.735Z
---

**Status:** COMPLETED 2026-09-27 (engine, GUI and README commits pushed). size-test.ps1 passed: YouTube "Size: 393 MB", My apps rows have no size. User asked whether apps show their size in KB/MB/GB. Options offered: Global search only / + My apps (extra lookups) / + download total; user chose Global search only. Follow [[plans-in-memory-first]], [[gui-tests-hidden]], [[no-coauthor]], [[no-pickers]].

## Facts
- Apple's search/lookup API (itunes.apple.com/search, /lookup) returns `fileSizeBytes` as a JSON string (YouTube: "392695808"). The engine's appstore.App drops it. visionOS search hydrates through /lookup, which also has it.
- The purchase history (My apps) has no size -> My apps unchanged (no Size column).

## Engine change (pkg/appstore/app.go)
- Add `FileSizeBytes string `json:"fileSizeBytes,omitempty"`` to App. String because Apple sends a string; the GUI parses it (a bad value just shows blank, never fails the search).
- CLI output unchanged: App.MarshalZerologObject lists fields explicitly; don't add size there (keeps `ipatool search` output identical).
- Run `go generate ./...` + `go test ./...` in the root module; README "Change to ipatool's own code" gets a line.

## GUI
- App gets Size int64; fromStoreApps parses FileSizeBytes (strconv, 0 on error).
- appModel: replace showPrice with `store bool` (App Store search results: have price and size). Columns built per page: Global search = Name, Bundle ID, Version, Price, Size, Platforms, App ID; My apps = same as today (Price blank, no Size). Value() maps by column key so indices can't drift.
- formatSize(bytes): decimal units like the App Store: "850 KB", "393 MB", "1.2 GB" (MB whole numbers, GB one decimal; 0 -> ""). Unit-tested.

## Steps
1. [ ] Engine field + root tests.
2. [ ] GUI Size + formatSize + tests.
3. [ ] Hidden-desktop test: Global search row shows size; My apps row has no size.
4. [ ] README (features + engine change note); separate commits (engine, GUI, docs); push.
