---
name: plan-sort-and-rename
description: "Proposed plan (2026-09-26, awaiting the user's answers) - a Sort by combo box on My apps (ascending/descending) and renaming the Search tab to Global search"
metadata:
  node_type: memory
  type: project
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-09-27T06:02:49.152Z
---

**Status:** COMPLETED 2026-09-26 (0da2325 GUI, 45b13fd README; pushed). sort-test.ps1 passed: tab "Global search", list "Global search results", all 8 orders give distinct first rows with focus kept on Sort by, Enter to list, search+sort combined, My apps rows no longer report a price. Approved 2026-09-26 ("Do all the above... all 8 sort methods"). Decisions: 8 sort choices (below), results list renamed "Global search results", Price column blank on My apps (purchase history has no prices — the owned-apps parser in pkg/appstore/appstore_owned_apps.go fills only ID, BundleID, Name, Version, PurchaseDate, Platforms, so it showed "Free" for everything). User asked for: sorting My apps ascending/descending via a combo box; renaming the Search tab to "App search" or preferably "Global search". Follow [[plans-in-memory-first]], [[gui-tests-hidden]], [[no-coauthor]], [[no-pickers]]. Builds on [[plan-my-apps-filter-reload]] (session cache, instant local views).

## Sort by (My apps)
- New combo box "Sort &by:" (Alt+B; free on My apps: used L P N T G W D C S) next to the platform filter, with the same accessible-name/label pattern.
- Choices (default first = today's order), all agreed:
  1. Newest purchase first (PurchaseDate desc)
  2. Oldest purchase first (PurchaseDate asc)
  3. Name, A to Z  4. Name, Z to A (case-insensitive)
  5. Bundle ID, A to Z  6. Bundle ID, Z to A (groups by developer)
  7. Newest to the App Store first  8. Oldest to the App Store first (by app ID; approximate age)
  Ties fall back to a fixed order (bundle ID, then ID) so the list never shuffles.
- Price column: appModel gets a showPrice flag; My apps' model leaves it off (blank cells), Global search keeps real prices.
- Placement: row 2 of My apps next to "Search my apps"; add the combo to setupMyApps' first-visit widgets; Enter handling in keyRouter covers both My apps combos.
- Data: add PurchaseDate (time.Time) to the GUI's App type; fromStoreApps copies appstore.App.PurchaseDate. Name sorts case-insensitive with a stable fallback (bundle ID) so ties keep a fixed order.
- Behaviour like the filter: applied instantly as you arrow, focus stays on the combo, Enter moves to the list, page goes back to 1; works together with search and platform filter. If nothing is loaded yet, the choice is remembered and used by the first load.
- Implement in myapps.go as sortApps(apps, mode) (pure, unit-tested); showMyApps applies filter -> search -> sort -> page. Enter handling: extend keyRouter's combo Enter check to the sort combo.

## Rename Search tab -> "Global search"
- pageTitles (main.go), TabPage Title (search.go), View menu item text comes from pageTitles ("Global search page", Ctrl+2), F1 help text, README mentions of "Search page"/"Search tab". The results list becomes "Global search results" (label "Global search resul&ts:", fixTableView name) — user chose this.
- Mnemonics on that page are unchanged.

## Steps (after the user confirms)
1. [ ] PurchaseDate in App + sortApps + tests.
2. [ ] Sort by combo on My apps (instant, Enter to list).
3. [ ] Rename Search -> Global search everywhere.
4. [ ] Hidden-desktop tests: each sort order's first row, sort + filter + search together, Enter to list, tab name announced "Global search", Ctrl+2 still goes there.
5. [ ] README + F1; commits; push.
