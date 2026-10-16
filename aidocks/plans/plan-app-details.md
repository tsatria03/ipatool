---
name: plan-app-details
description: "Plan (2026-09-29, written, NOT approved to build) - App info button/Alt+Enter showing an app's store details in an accessible read-only text dialog (My apps + Global search)"
metadata:
  node_type: memory
  type: project
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-09-29T18:22:57.038Z
---

**Status:** COMPLETED 2026-09-29 (da717ca store details kept, 8aab2ef App info window + Alt+O/Alt+Enter, 9dda968 README; pushed). Hidden test appinfo-test.ps1 passed: Global search Dice Only (1 lookup, 2nd press cached), My apps unavailable (11.ai) + available (Game-board) with 0 lookups, Escape/Close return focus to the list, Copy all == text. Dialog: "Details" read-only TextEdit (Alt+D), "Copy all" (Alt+C), "Close". Rating rounded to 1 decimal. App.Details (*storeDetails) now available for [[plan-more-sort-orders]] (Developer, sizeBytes()). User chose backlog ideas 7 (App details) and 8 ([[plan-more-sort-orders]]): "We can build one thing at a time." Build this one FIRST; 8 reuses its step 1. Follow [[plans-in-memory-first]], [[gui-tests-hidden]], [[no-coauthor]], [[no-pickers]], [[no-control-punctuation]], [[protect-apple-account]].

## Findings (code read 2026-09-29)
- gui/availability.go lookupIDs already gets Apple's full public lookup record for every owned app still on the store (batches of 150, 4 workers), but decodes only trackId. So for My apps the details cost ZERO extra requests: keep the records.
- Global search: engine appstore.App keeps only id/bundle/name/version/price/fileSizeBytes (pkg/appstore/app.go), so details there need ONE public lookup (lookupURL?id=<id>&country=<cc>, no sign-in) per press. Country: appstore.CountryCode(account StoreFront) from b.store.AccountInfo() (local keychain, no network) - same as ownedAppsAll in backend.go:403.
- Unavailable apps (~961) have no lookup record: show only what the purchase history has.
- Free Alt letters: My apps uses L P N I T G S R B W D C A E; Global search uses F P N S T D C. "O" is free on both -> button "App inf&o" (Alt+O). No trailing punctuation.

## Design
- New gui/details.go:
  - type storeDetails struct: Developer (artistName), Seller (sellerName), Category (primaryGenreName), AgeRating (contentAdvisoryRating), MinimumOS (minimumOsVersion), Price (formattedPrice), Size (fileSizeBytes -> int64), Released (releaseDate, first release), Updated (currentVersionReleaseDate), Version, Rating (averageUserRating) + Ratings (userRatingCount), Languages (languageCodesISO2A count), Description, ReleaseNotes (What's new), URL (trackViewUrl).
  - lookupIDs returns records (id -> storeDetails) instead of bare IDs; checkAvailability stores them on App (new field `Details *storeDetails`, nil = none). Memory: ~3500 descriptions, a few MB - fine.
  - lookupOne(ctx, client, country, id) for Global search (reuses lookupIDs with one ID).
  - detailsText(app App) string: plain lines, CRLF, one fact per line ("Name: ...", "Developer: ...", "Version: ...", "Size: 73 MB", "Age rating: 4+", "Category: Games", "Requires iOS 12.0 or later", "Rating: 4.5 out of 5, 1234 ratings", "Released: March 1, 2019", "Updated: ...", "Bundle ID", "App ID", "Purchased: ..." (My apps), "App Store link: ..."), then a blank line, "What's new:" block, blank line, "Description:" block. Unavailable app: first line "This app is no longer on the App Store.", then name/bundle/version/platforms/App ID/purchase date. Availability unknown (check failed): say "Details couldn't be loaded" + Load (Alt+L) hint.
- UI: "App inf&o" button in listButtons for BOTH lists (after Copy bundle ID; in My apps before Copy all apps) + Alt+Enter on a list row (Windows' usual "properties" key). Opens a modal dialog "App info - <name>" with a multi-line read-only LineEdit/TextEdit (focus starts there at the top so NVDA reads it; arrows read line by line; Ctrl+A/Ctrl+C copy) and buttons "&Copy" (all text) and "Close" (Escape closes). Dialog must follow winfix/tabs accessibility patterns (test with MSAA names).
- Global search: while the single lookup runs use runTask ("Getting app info.") so Escape cancels; failure -> plain-language message. Nothing selected -> existing "Select an app in the list first." message.
- F1 text + README (My apps and Global search control tables) updated.

## Steps (after go-ahead)
1. [ ] storeDetails + lookup records kept by the availability check (+ tests with the fake lookup server in availability_test.go).
2. [ ] detailsText + tests (available, unavailable, unknown).
3. [ ] dialog + App info button + Alt+Enter on both lists; Global search single lookup.
4. [ ] hidden-desktop test: My apps (an available and an unavailable app), Global search (1 lookup). Settings/clipboard backed up.
5. [ ] F1 + README; small commits; push to origin.

## Open question for the user (ask ONE at a time)
- Button name "App info" (Alt+O) ok? (default if not asked: yes)
