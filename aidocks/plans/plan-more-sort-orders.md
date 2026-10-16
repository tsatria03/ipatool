---
name: plan-more-sort-orders
description: "Plan (2026-09-29, written, NOT approved to build) - My apps Sort by gains size (largest/smallest) and developer (A-Z/Z-A), using data the availability check already downloads"
metadata:
  node_type: memory
  type: project
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-09-29T19:05:00.872Z
---

**Status:** COMPLETED 2026-09-29 (user: "Yes. Now you can."): 5c3abd6 sort orders (sortChoices 8-11, firstStoreSort=8, missing size/developer last both ways, same developer by name, Page-field message if the check failed) + 3338c98 README; pushed. sort2-test.ps1 passed (12 choices; largest NFS No Limits 3.8 GB; smallest 218 KB; removed apps on the last page). Note: the Write tool again failed "modified since read" on commit-msg.txt - wrote it with Set-Content + checked the title before committing. Build AFTER [[plan-app-details]] (reuses its step 1: the availability check keeps Apple's lookup records, incl. fileSizeBytes and artistName). Follow [[plans-in-memory-first]], [[gui-tests-hidden]], [[no-coauthor]], [[no-pickers]], [[no-control-punctuation]], [[protect-apple-account]].

## Findings (code read 2026-09-29)
- gui/myapps.go sortChoices has 8 entries (purchase date, name, bundle ID, App Store age, each both ways); sortApps uses a compare func + tie on bundle ID then ID (stable).
- Sizes/developers come from the availability lookup (no extra requests, no sign-in). Unavailable apps and apps whose check failed have neither.
- My apps has no Size column: the user said sizes were for "Global search only" (2026-09-27, [[plan-search-sizes]]).

## Design
- Add 4 entries at the END of sortChoices (existing indexes stay): "Largest first", "Smallest first", "Developer, A to Z", "Developer, Z to A".
- Size/developer from App.Details (see plan-app-details); apps without them (unavailable/unknown) always go LAST in both directions, then the existing tie order.
- Developer compare case-insensitive; ties inside one developer fall back to name A-Z (more useful than bundle ID), then bundle ID, ID.
- If the availability check failed and a size/developer sort is chosen: Page field explains like the availability filter does ("Couldn't check ... Press Load (Alt+L) to try again.").
- Copy all apps / Export to JSON already follow the sort order; JSON "sortedBy" gets the new names automatically.
- Tests in myapps_test.go: order, unknowns last both ways, ties. F1 text + README Sort by list.

## Steps (after go-ahead, after plan-app-details)
1. [ ] sortChoices + sortApps + tests.
2. [ ] Page-field message when data is missing.
3. [ ] hidden-desktop test (sort, check first/last rows via MSAA).
4. [ ] F1 + README; commits; push.

## Decided
- 2026-09-29 user: "Add both if possible." -> My apps gets a Size column AND a Developer column (from the lookup data; blank for removed/unknown apps). Global search: Developer column too? engine App lacks artistName - not required; keep Global search columns unchanged unless cheap. Add a step for the columns (appModel.columns() for !store: Name, Developer, Bundle ID, Version, Price, Size, Platforms, App ID? - keep Name first; put Developer after Name, Size after Version). Update README column list + export? (Copy all apps/JSON: consider adding size/developer - ask the user then.)
- User: "Let's do feat7 first." = go-ahead for [[plan-app-details]].
- 2026-09-29 REPLACED the column decision: user wants each My apps row read as "Game-board; Version: 1.0; Developer: Muamel Aljanahi; Purchase date: September 5, 2026; Size: 217 MB" (App info now has the rest). => My apps columns = Name, Version, Developer, Purchase date, Size (in that order; drop Bundle ID, Price, Platforms, App ID columns from My apps). Built as its own commit before the sort orders: DONE 25fc13c + 177f322 (README), pushed; columns-test.ps1 (reads LVM_GETITEMTEXT cross-process; UIA sees no rows on the hidden desktop) confirmed the rows. App.Size is now filled from the lookup for available owned apps. Sort orders themselves still NOT built - waiting for the user's go.
- 2026-09-29 user: Global search rows too: "speaking the version, developer, price, size. Get rid of the platform, and bundle/app ids." => Global search columns = Name, Version, Developer, Price, Size. Needs engine change #5: appstore.App gets `ArtistName string json:"artistName,omitempty"` (search/lookup JSON; CLI output unchanged - zerolog lists fields explicitly). GUI App gets a `Developer string` field (from ArtistName in fromStoreApps, from details.Developer in checkAvailability); developerColumn uses a.Developer for both lists (and the developer sort later). Commits: engine+test, GUI, README (engine-changes list + Global search text). DONE: 9409b57 (engine ArtistName), 727048e (GUI), aa52f84 (README), pushed; columns-test.ps1 confirmed both lists. Bundle/platforms/ID column vars removed from search.go. Global search columns unchanged. Copy all apps text format unchanged (not asked).
