---
name: plan-removed-app-downloads
description: "Plan (2026-09-27, approved) - download owned apps removed from the App Store: fall back from bundle ID lookup to the known App ID"
metadata:
  node_type: memory
  type: project
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-09-27T21:37:58.470Z
---

**Status:** COMPLETED 2026-09-27 (b92cf4c fix, a96de1a README; pushed). removed2-test.ps1 passed: My apps Unavailable -> "101 free alerts" -> Download selected -> Choose older version lists versions -> Download saved com.mobgen.101freealerts_476204078_unknown.ipa (2.8 MB) into the scratchpad; license box verified unticked via BM_SETCHECK/BM_GETCHECK (clicking a check box with WM_COMMAND does NOT toggle it). Version shows "unknown" in the file name when Apple's metadata lacks it (engine behaviour). User noticed removed-but-owned apps download in iTunes 12.6.5.3 / on iPhone but not in ipatool GUI. Diagnosed + proven on the hidden desktop: by bundle ID com.mobgen.101freealerts -> "Apple couldn't find that app" (backend.download calls store.Lookup = public itunes lookup, which doesn't know removed apps); by App ID 476204078 -> downloaded (2.8 MB, named 476204078_unknown.ipa). Follow [[plans-in-memory-first]], [[gui-tests-hidden]], [[no-coauthor]], [[no-pickers]], [[no-control-punctuation]]. Related: [[plan-availability-filter]].

## Design
1. sendToDownload (Download selected / Enter on a row, both lists) remembers bundle ID -> App ID in downloadPage.known. startDownload / chooseVersion pass the remembered ID as a hint when the App field still holds that bundle ID.
2. backend: one helper resolves the target: numeric -> App{ID}; bundle ID -> Lookup; if Lookup fails with "app not found" and a hint exists -> App{ID: hint, BundleID: bundleID} (price unknown, so skip the paid/free-not-owned rules that rely on the lookup's price). Used by download, listVersions, versionDetails.
3. File name keeps the bundle ID (engine fileName uses App.BundleID).
4. friendlyError "app not found": suggest entering the App ID if the app was removed but owned (My apps shows it).
5. Tests: errors_test fake store with Lookup returning not found -> download succeeds with hint, fails without; hidden-desktop: Download selected on an Unavailable app in My apps downloads into the scratchpad (license box: make sure it's unticked by reading its state; never buy).
6. README + commit + push.
