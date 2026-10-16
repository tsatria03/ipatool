---
name: plan-itunes-file-names
description: "Plan (2026-09-27, awaiting go-ahead) - name downloads 'App Name Version.ipa' like iTunes 12.6.5.3 instead of bundleID_appID_version.ipa"
metadata:
  node_type: memory
  type: project
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-09-28T03:41:45.899Z
---

**Status:** COMPLETED 2026-09-27 (321831b engine Name/Version, 35ec3d7 GUI rename, dc0d223 README; pushed - reworded with the user's OK and force-pushed with --force-with-lease, replacing 88d8b74/71b769a; file tree identical). names-dl-test.ps1 passed: "Band Tools 1.2.ipa" (downloaded twice, replaced), "101 Free Alerts - Change your text tone, new email alert, new voicemail alert and more 1.0.0.ipa" (version now 1.0.0 instead of unknown). MISTAKE: 88d8b74 got the engine commit's message (the Write of commit-msg.txt failed "modified since read" and I committed anyway in a parallel call) - asked the user whether to reword + force-push. Lesson: write the commit message file, confirm success, THEN commit (never in the same parallel batch). User: wants "Dice Only 1.9.ipa" (like iTunes 12.6.5.3) instead of "com.atastypixel.loopy-hd_467923185_1.6.33.ipa"; duplicates: "replace it if the ipa already exists". Follow [[plans-in-memory-first]], [[gui-tests-hidden]], [[no-coauthor]], [[no-pickers]], [[protect-apple-account]] (test with 1-2 small downloads only).

## Findings
- Engine names files in pkg/appstore/appstore_download.go packageFileName: bundleID_appID_version.ext; version only from item.Metadata["bundleShortVersionString"], else "unknown".
- Version is only known from Apple's download answer -> rename after the download.
- Every .ipa contains iTunesMetadata.plist (binary plist) = the item metadata: itemName (store name), bundleShortVersionString (may be missing), bundleVersion (e.g. "1.0.0" for 101 Free Alerts, which had no short version), softwareVersionBundleId, artistName. Mac .pkg has no such file.

## Design
- Engine change (#4): DownloadOutput gets Name and Version (from item.Metadata itemName; version = bundleShortVersionString, else bundleVersion) - small, also fixes nothing else; README note. Optionally make the engine's own "unknown" fall back to bundleVersion too? Keep engine naming unchanged for the CLI.
- GUI backend.download: after Download + ReplicateSinf succeed, rename DestinationPath to "<Name> <Version>.<ext>" in the same folder; replace an existing file (os.Remove then Rename; Rename on Windows fails if target exists). If Name empty -> keep the engine's name. If Version empty -> "<Name>.ipa".
- Sanitize for Windows: replace \ / : * ? " < > | and control chars (":" -> " -", others -> "-" or remove), collapse spaces, trim trailing dots/spaces, avoid reserved names (CON, PRN, AUX, NUL, COM1..9, LPT1..9), cap the name at ~150 characters (cut at a word boundary) so the path stays short.
- Result field/message show the new path. Pure function itunesFileName(name, version, ext) unit-tested.

## Steps (after go-ahead)
1. [ ] engine Name/Version in DownloadOutput + test. 2. [ ] itunesFileName + rename + replace + tests (fake store). 3. [ ] hidden-desktop test: download 101 Free Alerts by App ID into the scratchpad twice (second replaces), check the name "101 Free Alerts - ... 1.0.0.ipa". 4. [ ] README; commits; push.
