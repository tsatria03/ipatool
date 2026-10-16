---
name: plan-search-button-result-focus
description: "Proposed plan (2026-09-26, awaiting the user's answers) - a Search button next to Search my apps, and the Download button moving focus to the Result field"
metadata:
  node_type: memory
  type: project
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-09-27T06:39:50.533Z
---

**Status:** COMPLETED 2026-09-26 (de3fd5a Search button, 0f48bab Result focus, 54117de README; pushed). searchbtn-test.ps1 passed: click and Alt+R search and focus the list, no-match box, Download focuses Result ("Starting download..."), focus back on Result after the failed box closes. User's words: "When you search your apps, a search button should appear." and "pressing the download button should focus you to the results status bar." Follow [[plans-in-memory-first]], [[gui-tests-hidden]], [[no-coauthor]], [[no-pickers]]. Builds on [[plan-sort-and-rename]].

## 1. Search button on My apps
- Add PushButton "Sea&rch" (Alt+R; R is free on My apps, used: L P N T G W D C S B) right after the "Search my apps" box, before "Sort by". Tab order: search box -> Search -> Sort by.
- OnClicked = g.searchMyApps() (same as Enter in the box: loads first if needed, page 1, focus to the list, "No matches" box if nothing).
- Add it to setupMyApps' first-visit widget list and to purchasesPage (field `find`).
- CONFIRMED by the user: always visible ("Always there").

## 2. Download button -> Result field
- In startDownload, after the task starts, focus d.result (the read-only "Result:" edit on the Download page), so NVDA reads "Result, Starting download...". While downloading it updates with the percentage (read with NVDA+Tab / arrow keys). After the finished/failed message box closes, Windows returns focus to Result.
- Only when the task actually started (not on validation errors: missing app/folder keep their current focus).
- CONFIRMED by the user: "I meant the results read only field" (the Download page's Result field, focused after pressing Download).

## Steps (after the user confirms)
1. [ ] Search button + test.
2. [ ] Result focus + test (hidden desktop; back up/restore settings.json; use a small free app and cancel with Escape or a bad app ID so nothing big downloads).
3. [ ] F1 text, README; commits; push after approval.
