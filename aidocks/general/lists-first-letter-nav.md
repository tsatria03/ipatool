---
name: lists-first-letter-nav
description: "Every list control in the GUI must support first-letter (type-ahead) navigation, one page at a time"
metadata:
  node_type: memory
  type: feedback
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-09-28T01:35:44.046Z
---

Every list control the GUI has or adds (table views, list boxes, combo boxes) must support first-letter navigation: typing letters jumps to the next item starting with them. For paged lists (My apps), search the current page only.

**Why:** The user (NVDA, [[screen-reader-user]]) asked for it "to apply to all list controls" and prefers "1 page at a time". walk's TableView is a virtual list view and ignores LVN_ODFINDITEM, so letters did nothing there.
**How to apply:** Any new walk TableView must get the type-ahead helper from [[plan-first-letter-navigation]]; plain ListBox (no ItemStyler/owner-draw) and drop-down ComboBox already work natively - owner-drawn list boxes would need LBS_HASSTRINGS or WM_CHARTOITEM handling. Test type-ahead on the hidden desktop for new lists.
