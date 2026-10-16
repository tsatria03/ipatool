---
name: plan-not-signed-message
description: "Plan - Install on iPhone explains self-built/unsigned apps (e.g. NVGT games) as 'not signed for your iPhone' instead of 'may be damaged'; idea 3 of 3 (order 2, 3, 1); IN PROGRESS"
metadata:
  node_type: memory
  type: project
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-10-14T20:42:05.297Z
---

**Status:** COMPLETED 2026-10-14 (85a58ad feat, 2f30288 docs, 3a8e40a fix; not pushed). User: "The message shouldn't say anything about NVGT" -> msgNotSigned is now "This app isn't signed for your iPhone. Apps you build yourself must be signed first, for example with Sideloadly." Then (user: "fix the readme as well, as I did not make the nvgt engine") NVGT was removed from the README (8dff124) and from code comments/test names (4df21f1). Never mention NVGT (or name other engines as if the user made them) anywhere in this repo - say "a game engine" / "apps you build yourself". The user did NOT make NVGT; they make games with it. Built: signatureInfo.Encrypted (LC_ENCRYPTION_INFO(_64) cryptid) + HasProfile (app/embedded.mobileprovision); warnings(): !Signed OR (!Encrypted && !HasProfile) -> msgNotSigned (replaces the old "no signature" text); installErrorText maps 0xe800801c / "no code signature found" / 0xe8008015 / "valid provisioning profile" -> msgNotSigned. Test helpers: fakeProgram is now encrypted by default, fakeProgramWith(arm64, sig, encrypted), fakeIPA(t, program, extra ...ipaFile) with withProfile / withMetadata. Hidden GUI test on a scratch COPY of Classic Stand (scratchpad install-file-test.ps1 -file; paths with spaces break run-hidden's -Arguments, so copy to a no-space name) -> warning shown, No -> "Not installed."; copy deleted. Started 2026-10-14 after [[plan-apple-id-check]]. Next after this: Find App ID ([[feature-ideas-backlog]]). No release build until all 3 are done ([[build-output-folder]]).

**Why:** the user builds iOS games with NVGT. NVGT's .ipa has no signature for any phone, and the GUI today would say "This app has no signature, so your iPhone will probably refuse it." before, and after a refusal "Your iPhone couldn't verify this app's signature. The file may be damaged; download it again." - misleading for an app you built yourself.

## Test file (from the user, 2026-10-14)
`%USERPROFILE%\Downloads\Classic Stand.ipa` (23.1 MB, the user's own NVGT game; bundle com.NVGTUser.ClassicStand, 178 files): main program `ClassicStand` is thin arm64, 38 load commands, NO LC_CODE_SIGNATURE at all, LC_ENCRYPTION_INFO_64 cryptid 0, no embedded.mobileprovision, no _CodeSignature folder, iTunesMetadata.plist without account keys. Never copy, change or delete the user's file.

**Step 1 RESULT (2026-10-14):** spike install of Classic Stand refused at VerifyingApplication 40% after 3 s, nothing installed: "...original error: 'ApplicationVerificationFailed' errorDescription:'Failed to verify code signature of .../Payload/ClassicStand.app : 0xe800801c (No code signature found.)'". Also map the well-known 0xe8008015 ("A valid provisioning profile for this executable was not found", signed for other devices) to the same message (not tested on this phone).

## Plan
1. **Learn the phone's answer:** one install attempt of Classic Stand with the go-ios spike (scratchpad goios-spike\spike.exe) to record the exact refusal (expected at VerifyingApplication; nothing gets installed). Record the error text/code here.
2. **Detect before sending (gui/signature.go):** add to signatureInfo: `Encrypted bool` (LC_ENCRYPTION_INFO(_64) cryptid != 0 in the arm64/only slice) and `HasProfile bool` (Payload/*.app/embedded.mobileprovision present). A "self-built / not signed for this phone" app = not encrypted AND no provisioning profile AND (no signature OR signature without an Apple-issued chain - keep simple: no signature, or no profile). Warning text replaces the generic "no signature" one for this case: "This app isn't signed for your iPhone. Apps you build yourself, like NVGT games, must be signed first, for example with Sideloadly." (keep "Install anyway?", No default).
3. **After a refusal (gui/install.go installErrorText):** map the codes found in step 1 to the same message instead of "may be damaged".
4. Tests with made-up files (unsigned, unencrypted, profile present/absent) + the real error text; README (Install on iPhone + App signing: "apps you build yourself"). Commits small, [[no-coauthor]]; no release build.
