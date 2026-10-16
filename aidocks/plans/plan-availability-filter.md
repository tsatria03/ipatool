---
name: plan-availability-filter
description: "Proposed plan (2026-09-27, planning) - a second My apps combo box (user called it 'category') with All / Available / Unavailable, checked against the account's store"
metadata:
  node_type: memory
  type: project
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-09-27T21:06:54.460Z
---

**Status:** COMPLETED 2026-09-27 (204ad8c engine CountryCode, 4711289 GUI, acb5414 README; pushed). avail-test.ps1 passed: load+check ~14 s (4 parallel lookups; sequential was >60 s), 2593 available / 961 unavailable, Tab order Availability -> Platform filter, Enter to list, Unavailable+A-Z+"radio" = 14, copy text has no availability, JSON marks available=false. Also fixed: Search/Copy/Export pressed during a load now wait (purchasesPage.whenLoaded) instead of silently doing nothing. User's idea: instead of a sort method, "another combo box called category... all, available, unavailable, etc."; "if it was set to unavailable... sort and search methods would also carry over. Same for the copy/export options." Account is US ([[apple-account-us]]). Follow [[plans-in-memory-first]], [[gui-tests-hidden]], [[no-coauthor]], [[no-pickers]] (ONE question per reply), [[no-control-punctuation]]. Builds on [[plan-export-my-apps]], [[plan-sort-and-rename]].

## My view (told the user)
A separate combo box is better than a sort method: sorting orders apps, this narrows them, and it combines with every sort order. Works like the platform filter.

## Draft design
- Combo box on My apps, choices ANSWERED (option 1): "All apps" (default), "Available", "Unavailable". Label/name ANSWERED, then renamed by the user: "Availab&ility filter:" (Alt+I), matching "Platform filter" - user: "I like availability a lot better than category."
- Mnemonic: avoid letters used on My apps (L P N T G W D C S B R A E) and menu letters (F V T H). "Availab&ility" -> Alt+I, or "Categor&y" -> Alt+Y.
- Instant like the platform filter; Enter moves to the list (extend keyRouter's combo list); page back to 1; Page field counts it as narrowed; Search, Sort by, platform filter, Copy all apps and Export to JSON all follow it.
- The check: after the purchase history loads, look up all owned app IDs with Apple's public lookup (itunes.apple.com/lookup?id=<150 ids>&country=<account country>), ~24 requests. Part of the load task (adds a few seconds; status "Checking which apps are still on the App Store"). Unavailable = not returned. Load (Alt+L) redoes both.
- Country: the account's StoreFront; engine's countryCodeFromStoreFront is private -> small engine change: export it (e.g. appstore.CountryCode(storeFront)), third fork change, README note. Keep GUI's own HTTP request (net/http, JSON decode trackId only).
- If the check fails (network): apps load anyway, Available/Unavailable choices show a message "Couldn't check availability: ..." and All apps works.
- App gets Available *bool (nil = unknown). JSON export: heading "availability": "<choice>", each app "available": true/false (omitted if unknown). Copy lines ANSWERED: unchanged, never add "Available: yes/no" to the clipboard text (user: "I do not want it to say available, yes"). Placement (user correction): row 1, right BEFORE the platform filter (after Next page): Load, Previous page, Next page, Availability, Platform filter, Page. All design questions answered; waiting for go-ahead.
- Pure functions + tests: filterApps gains availability arg; batching helper.

## Steps (after answers + go-ahead)
1. [ ] engine CountryCode export + test. 2. [ ] backend availability check (batched) + App.Available. 3. [ ] combo + filtering + Enter + page info. 4. [ ] export fields. 5. [ ] hidden-desktop test (counts 2593/961 or current; unavailable + search + sort; copy/export into scratchpad only). 6. [ ] F1, README; commits; push.
