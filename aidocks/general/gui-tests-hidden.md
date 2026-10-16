---
name: gui-tests-hidden
description: Automated GUI tests must never show windows or steal focus; run them on a separate hidden desktop
metadata:
  node_type: memory
  type: feedback
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-09-27T18:51:04.801Z
---

When testing a GUI app, never let its windows appear on the user's screen or take focus from what they're doing.

**Why:** The user uses NVDA ([[screen-reader-user]]); test windows popping up yanked them out of whatever app they were in and NVDA announced them. They explicitly asked for tests to run "in the background".
**How to apply:** Launch both the app and the test driver on a separate desktop (CreateDesktop + CreateProcess with STARTUPINFO.lpDesktop), write results to a file, then close the desktop. Don't use Start-Process for GUI apps, BM_CLICK, or anything needing the window to be foreground. A hidden-desktop runner script exists in the session scratchpad pattern: run-hidden.ps1.

**Never let a test write into the user's folders.** 2026-09-27 an export test couldn't find the Save dialog's file-name box, pressed OK anyway, and created %USERPROFILE%\Downloads\My apps.json (I verified it was the test's own new file, deleted it, told the user). Rule: in a Save/Open dialog, set the path, READ IT BACK, and press OK only if it points into the scratchpad; otherwise press Cancel (IDCANCEL) and fail the test. Same for anything else that writes files, settings (back up/restore settings.json) or the clipboard (back up/restore).
