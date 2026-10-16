---
name: plan-my-apps-filter-reload
description: "Combined plan (2026-09-26, awaiting go) for My apps - fetch all owned apps once per session (engine cap raised), then instant local search, platform filter and paging; auto-load on first Tab into the page (option 4)"
metadata:
  node_type: memory
  type: project
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-09-27T05:43:44.423Z
---

**Status:** user chose option 2 (engine change + cache) combined with option 4 on 2026-09-26. Recorded and reviewed against the code; user said go 2026-09-26; COMPLETED 2026-09-26 (commits 7945752 engine cap, a83e523 cache+local view, 32e8335 UI, 321be5b README; pushed). myapps-test.ps1 passed all cases (first Tab load 13 s with focus kept, instant filter/search/paging 0.03 s, Enter moves to list, no Busy box for Alt+L, Load refresh). tasks.go gained showBusy() and g.afterTask (clean-up that runs even after Escape). Follow [[plans-in-memory-first]], test per [[gui-tests-hidden]], commit per [[no-coauthor]], ask in plain text per [[no-pickers]].

## Why
Measured 2026-09-26: every OwnedApps request takes 11-14 s because pkg/appstore/appstore_owned_apps.go (OwnedApps -> fetchOwnedApps) downloads the whole purchase history (storefronts 34 and 13) each time, then sorts and slices one page locally (ownedAppsPage). The account has 3554 apps; the cap MaxOwnedAppsLimit = 100 would force ~36 requests (~8 min) to get them all.

## Design
1. **Engine change (ipatool code, own commit):** raise `MaxOwnedAppsLimit` from 100 to 100000 in pkg/appstore/appstore_owned_apps.go. Update pkg/appstore/appstore_owned_apps_test.go lines ~378/381, which hard-code 100/101, to use MaxOwnedAppsLimit and MaxOwnedAppsLimit+1 so they stay correct. cmd/purchases.go validates against the same constant (the CLI's --max-results can be larger; harmless). Run `go test ./pkg/appstore/` in the repo root.
2. **backend.ownedAppsAll():** OwnedApps{Page: 1, Limit: MaxOwnedAppsLimit, Platform: ""} inside withAccount -> all apps, already sorted by purchase date, with platforms.
3. **Session cache in purchasesPage:** `all []App`, `loaded bool`. Kept in memory while the GUI runs.
4. **Local view:** view = all apps matching the platform filter AND the search text (case-insensitive substring of name or bundle ID), split into pages of 25 in memory. Page field: "Page 2 of 143, 3554 apps" or, when searching/filtering, "Page 1 of 2, 30 of 3554 apps match".
5. **New search box** on My apps: label "&Search my apps:" (user chose this name 2026-09-26; Alt+S is free on this page; used letters L P N T G W D C), accessible name set; Enter applies the search and moves focus to the list (first match read by NVDA); status "30 apps match". Empty search = all apps. If nothing is loaded yet, Enter first loads from Apple, then applies the search.
6. **Platform filter:** instant — apply as soon as the selection changes (user confirmed 2026-09-26; no Enter/Tab-out rule). Focus stays on the filter so arrowing keeps working. Enter in the filter moves focus to the list. If nothing is loaded yet, a filter change only records the choice (the first load applies it).
7. **Previous/Next page:** instant, local, keep filter and search.
8. **Load:** fetches a fresh list from Apple (~12 s), keeps filter and search, goes to page 1, focus to list.
9. **First visit (option 4):** the first time each session focus enters a My apps control (Tab, Shift+Tab, Alt shortcut; not arrowing past the tab or Ctrl+4), fetch all apps without moving focus; status "3554 apps loaded." Needs AssignTo handles for Previous/Next and the My apps copies of Download selected / Copy bundle ID; attach FocusedChanged. Ignore focus changes caused by leaving the window (Alt+Tab) — only react when focus is inside the main window.
10. **Busy rules:** automatic loads never show the Busy box; Alt+L on a fresh page (which focuses Load and starts the first-visit load) must not show the Busy box either.
11. **Enter on the filter:** walk doesn't claim Enter for drop-down lists (only editable combos, comboBoxEditWndProc); intercept in keyRouter.OnPreTranslate (tabs.go) when msg.HWnd is the filter.
12. fillList: add a keep-focus option for automatic loads.
13. README (Features, How it works: session cache + the engine change as a fork change; source layout) and F1 help.

## Steps
1. [x] (Done in 7945752. Also had to update cmd/purchases_test.go "max results over limit" (passed "101" as text). ipatool's tests need `go generate ./...` first (mocks are gitignored). All `go test ./...` pass; CLI list-purchases --max-results 5000 returned 3554/3554 in 15.8 s.) Engine change + upstream test update; `go test ./pkg/appstore/`; commit "feat(appstore): allow fetching all owned apps in one request".
2. [ ] ownedAppsAll + session cache + local view (pure function with unit tests: filter, search, paging, page text).
3. [ ] My apps UI: search box, instant filter, local paging, Load refresh, first-visit auto-load, keyRouter Enter, busy rules.
4. [ ] Hidden-desktop tests: Ctrl+4 only -> no load; Tab in -> one load (~12 s), focus stays; paging and filter instant (no Apple requests in Log); arrowing the filter changes the list instantly with focus staying on the filter; search "youtube" (Alt+S box) finds owned YouTube apps and focuses the list; empty search restores all; Load refreshes; Alt+L on fresh page -> no Busy box; Alt+Tab-like focus loss -> no load. CLI check: `go run . list-purchases --max-results 5000` works.
5. [ ] README + F1; commits; push.

## Risks
- The engine change is a fork change to ipatool's own code: keep it in its own commit and mention it in the README so upstream merges are easy to review.
- First load is ~12 s; the status bar seconds counter shows it's working.
- The cached list isn't live: purchases made meanwhile appear after Load.
