---
name: plan-first-letter-navigation
description: "Proposed plan (2026-09-27) - first-letter (type-ahead) navigation in the app lists; walk's virtual list view ignores LVN_ODFINDITEM"
metadata:
  node_type: memory
  type: project
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-09-28T01:46:42.807Z
---

**Status:** COMPLETED 2026-09-27 (e75b6a1 feature in gui/typeahead.go, 49c5992 README; pushed). typeahead-test.ps1 passed (WM_CHAR posted to the list): y -> YouTube Music, y again -> YouTube Studio, t-a-p-t -> TapTapSee, no match stays, both lists. Test-script gotcha: never name a PowerShell function Type (alias of Get-Content) or Pause. User: "first letter navigation in list controls, example in the apps list in the my apps tab. Right now pressing any letter to jump to an item does not work very well, or if at all." Follow [[plans-in-memory-first]], [[gui-tests-hidden]], [[no-coauthor]], [[no-pickers]].

## Cause (verified in walk source)
walk's TableView creates its list views with LVS_OWNERDATA (virtual: the list asks for rows on demand). Virtual list views can't search their own text, so on a typed letter they send LVN_ODFINDITEM to the parent and expect the matching row index; walk (tableview.go WM_NOTIFY) never handles it, so letters do nothing useful. tailscale/win defines LVN_ODFINDITEM but not NMLVFINDITEM/LVFINDINFO.

## Design
- winfix.go: subclass the TableView wrapper (tv.Handle(), parent of both SysListView32) like tabs.go does; on WM_NOTIFY from the real list with code LVN_ODFINDITEMW, read NMLVFINDITEMW {NMHDR; iStart int32; LVFINDINFOW{flags uint32; psz *uint16; lParam uintptr; pt POINT; vkDirection uint32}}, search the model's app names case-insensitively by prefix from iStart to the end, wrapping (LVFI_WRAP), return the index or -1. The list view itself handles typing speed (several quick letters = a prefix like "tap"; the same letter repeated cycles through matches).
- Pure function findByPrefix(names, prefix, start) int, unit-tested (wrap, case, repeat cycling, no match, non-Latin).
- Both lists: Global search results and Apps you own (fixTableView gets the model, or a separate enableTypeAhead(tv, model)).
- ANSWERED: current page only ("I prefer 1 page at a time, so option 1").
- User: "should apply to all list controls... Is it possible for it to work in any type of list control?" Findings: only walk TableViews are broken (virtual). The Choose a version ListBox (walk ListBox without ItemStyler = plain LBS with strings) and all combo boxes (CBS_DROPDOWNLIST) already have Windows' built-in first-letter navigation. Make the fix a general helper enableTypeAhead(tv, textAt func(row) string) applied to every TableView (both today; any future TableView must call it - note in README walk workarounds). Rule for the future: every new list control must support first-letter navigation.

## Steps (after answer + go-ahead)
1. [ ] findByPrefix + tests. 2. [ ] subclass + hook both lists. 3. [ ] hidden-desktop test: post WM_CHAR letters to the list, check the focused row (single letter, repeat, multi-letter prefix, no match). 4. [ ] README (walk workarounds + feature line), F1 line; commit; push.
