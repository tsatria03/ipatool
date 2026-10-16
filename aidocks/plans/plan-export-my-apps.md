---
name: plan-export-my-apps
description: "Plan (2026-09-27, final design, awaiting go-ahead) - Copy all apps (plain text lines) and Export to JSON (with metadata) for the whole My apps list, following filter/search/sort"
metadata:
  node_type: memory
  type: project
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-09-27T18:53:26.674Z
---

**Status:** COMPLETED 2026-09-27 (b49a10b feature, ed7842a README; pushed). export-test.ps1 passed: copy 3554 lines / 13 for youtube A-Z, JSON narrowed 13 + full 3554 with heading, CJK names kept, overwrite prompt, Downloads untouched (after one test mishap, see [[gui-tests-hidden]]). Also: "update readme.md for the new my app tab options. Including the search app button" -> README gets a My apps section describing every My apps control (Load, Previous/Next, Platform filter, Page, Search my apps + Search button, Sort by, list, Download selected, Copy bundle ID, Copy all apps, Export to JSON). User's words: copy/export all apps from My apps, "not the first 100 apps... all 3554", following how it's sorted; then "a json file with the apps, and the metadata"; final: "option 3, but if you copy the apps, that should just be plain text listing the app, according to my earlier app example" = Export saves JSON, Copy copies plain text lines (no heading). Follow [[plans-in-memory-first]], [[gui-tests-hidden]], [[no-coauthor]], [[no-pickers]] (one question per reply). Builds on [[plan-sort-and-rename]], [[plan-my-apps-filter-reload]].

## Decisions
- Scope (answered option 1): follow platform filter + Search my apps + Sort by; every page of the matches; all 3554 when nothing is narrowed. Data comes from purchasesPage.all (in memory, instant, no Apple request).
- Not loaded yet: load first, then copy/export (like Search).

## Copy all apps (Alt+A) - plain text
- One line per app, no heading, CRLF line endings, exactly: `Game-board; Bundle ID: net.muamal.gameboard; Version: 1.0.5; Platforms: iphone; App ID: 6786885206` (empty fields left out).
- Message box "Copied 3554 apps to the clipboard." + status bar.

## Export to JSON... (Alt+E)
- Save dialog, filter "JSON file (*.json)", default name "My apps.json" in the Download page's Save to folder.
- UTF-8 (no BOM; JSON standard), indented 2 spaces, non-ASCII kept as-is (腾讯微博 readable).
- Shape (shown to the user and liked):
  {"exportedFrom":"ipatool GUI, My apps","exported":"<RFC 3339 local time>","totalApps":3554,"exportedApps":N,
   "search":"","platform":"all platforms","sortedBy":"<sort choice>",
   "apps":[{"name","bundleID","version","platforms":[...],"id":<number>,"purchaseDate":"<RFC 3339>"}]}
  App field names match `ipatool list-purchases --format json`. Omit version/purchaseDate when empty.
- Message box "Saved N apps to <path>." + status bar. Write errors -> plain-language error box.

## Code
- myapps.go: exportLines(apps) string and exportJSON(meta, apps) ([]byte, error) - pure, unit-tested.
- purchases.go: two buttons after Copy bundle ID (need a My-apps-only button row; listButtons is shared with Global search -> add the two buttons in purchasesTab next to listButtons' composite or extend listButtons with extra widgets). Add to setupMyApps widget list. Helper currentView() returns the filtered+sorted slice (shared with showMyApps).
- F1 + README.

## Steps (after go-ahead)
1. [ ] exportLines/exportJSON + tests. 2. [ ] Buttons, clipboard, save dialog. 3. [ ] Hidden-desktop test: clipboard line count + first line per sort/filter; JSON export to the scratchpad (never user folders), parse it back, check counts; back up/restore settings.json and clipboard. 4. [ ] F1, README; commits; push.
