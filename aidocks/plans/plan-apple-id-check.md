---
name: plan-apple-id-check
description: "Plan - before Install on iPhone, warn when the .ipa was downloaded with a different Apple ID than the signed-in account (reads iTunesMetadata.plist only); idea 2 of 3 (order 2, 3, 1); NOT STARTED"
metadata:
  node_type: memory
  type: project
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-10-14T20:20:09.240Z
---

**Status:** COMPLETED 2026-10-14 (b72f529 feat, 92135d9 docs; not pushed). Built as planned (g.accountEmail set in showAccount, cleared in setAccountState when not signed in; signatureInfo.AppleID via metadataAppleID; otherAppleID + msgOtherAppleID). EXTRA change found by the hidden test: file warnings now come BEFORE the phone error (installCheck.phoneErr) so they show with no phone connected. Hidden test: scratchpad make-other-ipa.py + install-other-test.ps1 -> warning shown, No -> "Not installed.". No release build (on hold until all 3 ideas are built). Next: idea 3 ("Not signed for your iPhone"), needs one unsigned self-built .ipa from the user to learn the real phone error codes. The user chose the order of the three follow-up ideas from [[feature-ideas-backlog]]: 2 (this), then 3 ("Not signed for your iPhone" message), then 1 (Find App ID).

**Why:** an App Store app only opens on a phone that has the keys of the Apple ID that bought it (tested: [[plan-install-on-iphone]]). Installing someone else's download usually succeeds but the app won't open, which is confusing.

## Facts checked
- ipatool writes iTunesMetadata.plist (binary plist) at the .ipa root with `apple-id` and `userName` = the downloading account's email (pkg/appstore/appstore_download.go writeMetadata). Confirmed in a real download (Virtual Balloon): both keys present.
- .ipa files from iTunes/other tools may instead have `com.apple.iTunesStore.downloadInfo` -> `accountInfo` -> `AppleID` (read it too, as a fallback).
- Self-built / dev / ad hoc .ipa files (e.g. a game engine's .ipa has iTunesMetadata.plist WITHOUT any account keys) -> no account in the file -> no warning.
- The GUI learns the signed-in email only through checkAccount -> showAccount(acc) (acc.Email; may be a phone number for phone-number logins - compare the same way).

## Steps (small commits, [[no-coauthor]])
1. **gui/account.go:** keep the signed-in account's email in a gui field (`g.accountEmail`), set in showAccount, cleared on logout and when the check fails (signed out).
2. **gui/signature.go (or a small new file):** `ipaAppleID(z *zip.Reader) string` reading iTunesMetadata.plist with howett.net/plist (binary or XML): `apple-id`, else `userName`, else downloadInfo.accountInfo.AppleID; "" when none. Read it in checkSignature's zip pass (no second open) -> add `AppleID string` to signatureInfo (rename nothing else).
3. **Warning:** in startInstall, when both the file's Apple ID and g.accountEmail are known and differ (trimmed, case-insensitive), add to the warnings: "This app was downloaded with a different Apple ID. It will only open on your iPhone if that Apple ID has been signed in on it." Same "Check before installing" box, "Install anyway?", No default. Never show or log the other email (privacy). No warning when either side is unknown (not signed in / unknown account / no account in the file).
4. **Tests:** fake .ipa with iTunesMetadata.plist in binary and XML forms, the downloadInfo fallback, no keys (like a self-built app), case differences; warning text test. GUI check on the hidden desktop only if cheap (no phone needed: the warning box appears before sending; answer No).
5. **README** (Install on iPhone steps + Caution bullet) and nothing in F1 (no new shortcut). NO release build here: user 2026-10-14 "Do not compile the ipa tool into an exe until all 3 plans are built" -> build releases\ipatool only after ideas 2, 3 and 1 are all done ([[build-output-folder]]). Test builds in the scratchpad for hidden-desktop tests are fine.

**Not included:** blocking the install (the user may have signed that Apple ID in on the phone before - our tests showed the keys stay).
