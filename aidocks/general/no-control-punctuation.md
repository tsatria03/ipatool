---
name: no-control-punctuation
description: "Never end form control text (buttons, list items, check boxes, menu items) with punctuation such as \"...\" or \".\""
metadata:
  node_type: memory
  type: feedback
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-09-27T19:02:03.097Z
---

Never put punctuation at the end of form control text: no "..." on buttons (e.g. "Export to JSON", not "Export to JSON..."), no trailing "." on list items or placeholders ("Loading", "No versions found"). Accessible names from labels already drop the trailing colon (accessible() in gui/main.go).

**Why:** The user (NVDA, [[screen-reader-user]]) said "Remember to never add punctuation at the end of form controls, such as buttons and lists." NVDA reads the dots ("Export to JSON... button"), which is noise.
**How to apply:** Applies to every button, check box, radio, list/combo item, menu item and tab title in the GUI, including new ones and dialogs. Message box texts, status bar messages and field values (e.g. the Result field) are sentences and may keep normal punctuation. Fixed 2026-09-27: Export to JSON, Choose older version, Browse, version list "Loading" / "No versions found".
