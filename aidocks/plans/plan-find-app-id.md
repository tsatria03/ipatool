---
name: plan-find-app-id
description: "Plan - Find App ID dialog on the Download page: search the Wayback Machine (never Apple) for saved App Store links by app name, confirm name/developer from the saved page, fill the App ID field; idea 1 of 3 (last); PLAN WRITTEN, waiting for go-ahead"
metadata:
  node_type: memory
  type: project
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-10-14T21:34:15.372Z
---

**Status:** COMPLETED 2026-10-14 (3b173af search code, ae0d96d window, f9b4758 F1, e445dfc README; not pushed). Release build made into releases\ipatool (ipatool-gui.exe 44.6 MB) - the hold is over. Deviations from the plan: all 3 address forms are always searched and merged (no early stop - SixthSense needed both apps.apple.com and itunes results); newArchiveClient refuses redirects that leave web.archive.org (a saved 301 could point to Apple's live store); slug taken with /app/<slug>/id regex (old addresses have :80); titles use a no-break space in "App Store" (handled); "Use selected" also clears Version ID. Real hidden tests: heathcote -> 2 apps (Heathcote 1050456637 + Heathcote High School Enews 889683640), sixthsense -> 2 apps (TechXperts 421794646 2011, TeamHN 580417319), Use selected fills the App field. Scratchpad findid-test.ps1 -name <x>. Last of the three follow-up ideas (order 2 [[plan-apple-id-check]], 3 [[plan-not-signed-message]], 1 this). After it is built, the release build on hold ([[build-output-folder]]) can finally be made.

**Why:** Apple's search only knows apps still for sale; for removed apps not in My apps the user needs the App ID. By hand I found many via the Wayback Machine (SixthSense, Blinded Chef, LookTel VoiceOver Tutorial, Heathcote, Crazy Sounds, Tamil Rhymes, 700 Ringtones) and Virtual Balloon via AppShopper. Credibility rules agreed 2026-10-13 ([[feature-ideas-backlog]]): saved Apple pages first, confirm name + developer before using an ID, never archive.org user uploads.

## How it works
1. **Download page:** a button "Fi&nd App ID" (Alt+N; free on that page - used there: A P V H F B G W D I O R) in the empty third column of the App row. Opens a dialog "Find App ID". If the App field holds text that isn't a number or bundle ID, it is pre-filled as the name.
2. **Dialog** (first-letter nav in the list, [[lists-first-letter-nav]]; no ending punctuation on controls, [[no-control-punctuation]]):
   - "App &name:" edit + "&Search" (default button; Enter searches).
   - "&Results:" ListBox, one line per App ID: "Heathcote; Developer: Fabulae Obscurae, Inc.; App ID: 1050456637; saved 2019" - before details load: "heathcote; App ID: 1050456637; checking the saved page".
   - "&Use selected" (also Enter/double-click on a result): puts the ID into the App field, closes, focus to the App field.
   - "Cancel" (Escape).
   - A read-only status line ("&Status:") for "Searching", "3 apps found", "Nothing found..." so NVDA can read it; focus moves to the list when results arrive, else stays on the name box.
3. **Search** (gui/findid.go, plain net/http to web.archive.org only, User-Agent "ipatool-gui", 60 s timeout, cancellable with Escape through runLocalTask - no passphrase):
   - slug = lowercase name, every run of non-letters/digits -> "-", trimmed; apostrophes dropped first ("Tony's" -> "tonys").
   - CDX queries, in order, stop when IDs are found: `apps.apple.com/us/app/<slug>`, `itunes.apple.com/us/app/<slug>`, `itunes.apple.com/app/<slug>` with `matchType=prefix&fl=timestamp,original,statuscode&collapse=urlkey&limit=500`. Max 3 requests per search.
   - From each original URL take `/id(\d+)`; group by ID; keep the newest snapshot with status 200 (fallback: newest of any status) and the slug text.
   - Retry a 503/504/timeout once; then "The Internet Archive isn't answering right now. Try again in a minute."
4. **Confirm details:** right after the search, fetch the saved page of up to 10 IDs (newest 200 snapshot, `web/<ts>id_/<original>`), 2 at a time, filling each line as it arrives (like Choose older version). Parse best-effort: name from `<title>` ("X on the App Store" / "X for iPhone..." / "App Store: X"), developer from the developer link / "By X" / app-header__identity, price if present. Failed fetch -> line keeps the slug name and says "details not available". More than 10 IDs: a "Check &more" button checks the next 10.
5. **Nothing found:** message suggests fewer words (the start of the name), another spelling, and that some apps were never saved by the archive.
6. **README** (Download page / new "Find App ID" section: sources, verification, limits, that nothing goes to Apple) + **F1** line (Alt+N). Then build both exes into releases\ipatool (all 3 ideas done).

## Code layout
- gui/findid.go: `appSlug(name)`, `searchArchive(ctx, client, name) ([]archiveHit, error)`, `pageDetails(ctx, client, hit) (name, developer, price string, err)`, parsing helpers; base URL in a variable so tests can point it at httptest.
- gui/findidpage.go: the dialog (patterned on chooseVersion in download.go).
- gui/findid_test.go: slug cases (spaces, apostrophes, "&", colons, accents kept? -> strip non-ASCII safely), CDX parsing from saved real samples (heathcote, blinded-chef, sixthsense - multiple IDs), page parsing from small saved HTML samples (2021 apps.apple.com and 2013-style itunes page), retry/timeout via httptest.
- Commits small ([[no-coauthor]]); hidden GUI test with a fake archive server is hard (exe calls the real site) -> GUI test only checks the dialog opens, controls/names, and one real search for "heathcote" (one CDX + up to 2 page requests to archive.org; harmless).

## Review (2026-10-14, after re-reading)
- OK: never contacts Apple; download by the found ID is still the user's separate action (and proves ownership).
- Risk: one slug -> several apps (sixthsense found 3 IDs) -> handled by listing all with developer names; never auto-pick.
- Risk: non-English names: Apple slugs transliterate or drop characters; plan only handles ASCII well -> say so in the "nothing found" message; user can type the English/short name.
- Risk: CDX can return thousands of rows for short slugs ("dice") -> limit=500 + collapse=urlkey; tell the user to type more of the name when 500 rows come back.
- Risk: archive.org outages (it was down for days in Oct 2024) -> clear message, nothing else breaks.
- User 2026-10-14: "We do not need extra check boxes for the first version of the feature. I do not want to over complicate it" -> v1 stays as planned (Apple's saved store addresses only, no options/check boxes). AppleVis / AudioGames.net (accessible apps) discussed as possible LATER sources; not in v1. AudioGames.net checked 2026-10-14: live site is behind Cloudflare's bot check (403 Forbidden for scripts; CDX shows __cf_chl_tk challenge URLs) - never try to get around it; game pages are db.php?action=view&id=<game name> (old ones saved in the Wayback Machine since 2005). The user stopped further digging ("I always forget that the site is guarded by cloudflare").
- Deferred (not in v1): AppShopper (needs the category in the URL - would cost ~20 requests per search) - mention it in the README as a manual tip; country stores other than us; caching results.
- Accessibility check: every control has an accessible name via accessible(); status changes are read because focus moves to the list or the result count goes to the status line.
- FIX from review: in a modal dialog Escape is the dialog's Cancel button (the main window's Escape accelerator doesn't reach it), so closing the dialog must also cancel a running search/detail task (call g.cancel()), otherwise the GUI stays busy for up to a minute. chooseVersion has the same gap - leave it, note only.
- Checked: Alt+N is free on the Download page and not a menu mnemonic (menus: File, View, Task, Help).
