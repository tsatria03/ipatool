---
name: screen-reader-user
description: "User relies on the NVDA screen reader; GUIs must use native controls (wxPython or Go walk with fixes), never tkinter/Fyne"
metadata:
  node_type: memory
  type: user
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-09-26T22:17:55.493Z
---

The user uses NVDA on Windows. A tkinter GUI was unusable for them (NVDA only announced the window "pane"), because Tk draws its own widgets and doesn't expose them to accessibility APIs. Fyne/Gio (Go) have the same problem.

**Why:** Custom-drawn controls aren't exposed via MSAA/UIA; native Win32 controls are.
**How to apply:** Build GUIs with wxPython or Go's github.com/tailscale/walk. With walk, apply the fixes in gui/winfix.go of the ipatool repo: unique control IDs (walk uses ID 0 for all, so button clicks get lost), strip the frozen TableView list from tab order and name the real one, and set accessible names on inputs (walk's labels are nested so NVDA can't associate them). Give every control a label, & mnemonics, menu accelerators, set initial focus, and report results via focus changes or message boxes (status bar changes aren't announced). Explain keyboard shortcuts in replies. Test per [[gui-tests-hidden]].
