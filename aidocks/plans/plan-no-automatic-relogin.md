---
name: plan-no-automatic-relogin
description: "Fix - the GUI never signs in again on its own; an expired sign-in shows a plain message pointing to the Account page, where the user can type their two-factor code; COMPLETED 2026-10-31"
metadata:
  type: project
---

**Status:** COMPLETED 2026-10-31 (e6d64f6 fix + messages + tests, fd5fe4d README; not pushed yet). User approved building ("yes"). TestExpiredSignInNoRelogin fails on the old code with exactly "auth code is required" + 1 sign-in, passes now. Both exes rebuilt into releases\ipatool. gofmt flags findid.go / findid_test.go (older, untouched). Replaces the earlier draft plan-no-relogin-for-app-id-downloads (never committed, deleted).

**Why:** 2026-10-31 the saved sign-in had expired. Downloading iFoley Footsteps (App ID 907997410) by App ID got "sign in required" (ErrPasswordTokenExpired); backend.download signed in again with the saved password; the account uses two-factor, so Apple wanted a code the download can't ask for -> "Download failed, auth code is required", plus a sign-in prompt on the user's iPhone. Ready to Roll then failed the same way. After the user logged out and in on the Account page (typing the code), both downloaded fine. So the automatic sign-in can never finish on this account and only sends surprise prompts. This also covers the case [[plan-no-relogin-on-refused-license]] fixed for the Purchase step (Sound Shaker, 2026-10-14), which may itself have been an expired sign-in.

**Scope:** GUI only (gui/). cmd/ stays as upstream (the CLI keeps its retry; the fork doesn't change cmd/). Checked against the code 2026-10-31.

**Design:**
1. gui/backend.go withAccount (around line 181): stop signing in again on ErrPasswordTokenExpired; it just gets AccountInfo and runs fn, returning fn's error. Affects listVersions, versionDetails, ownedAppsAll and any other user. Update its comment (it says it "retries once, as cmd/ does").
2. gui/backend.go download (loop around lines 249-327): remove the Login branch at the top of the attempt (`if errors.Is(lastErr, appstore.ErrPasswordTokenExpired)`), and remove ErrPasswordTokenExpired from `retry`, so an expired sign-in returns at once. The license retry (ErrLicenseRequired with getLicense ticked: purchase, then download again) stays.
3. Purchase step with an unknown price (the errNoFreeLicense check, around line 274): keep returning errNoFreeLicense, but reword its message to cover both causes, because without a fresh sign-in the GUI can't tell an expired sign-in from a refused license: "Apple asked for a fresh sign-in before giving a license for this app. Log in again on the Account page (Ctrl+1), where you can type your two-factor code, then try once more. If Apple asks again right after you've logged in, this was probably a paid app your account didn't buy."
4. gui/messages.go: the ErrPasswordTokenExpired message (errorText, line ~33) and the "password token is expired" text match in friendlyError (line ~68) both become: "Your sign-in has expired. Log in again on the Account page (Ctrl+1), where you can type your two-factor code, then try again."
5. gui/messages.go errorText: add appstore.ErrAuthCodeRequired -> "Apple asked for a two-factor code. Log in again on the Account page (Ctrl+1), where you can type the code." (Should no longer happen during downloads, but keep it as a safety net instead of the raw "auth code is required".) The Account page's own login (backend.login with askCode) is unchanged; it already asks for the code.

**Tests (gui/errors_test.go fakeStore, no requests to Apple):**
- Change the test at line ~196 ("known price ... want a sign-in attempt"): known-price app, Download returns ErrPasswordTokenExpired -> returns ErrPasswordTokenExpired, store.logins == 0.
- App ID target, Download returns ErrPasswordTokenExpired -> no Login, one download attempt.
- listVersions (through withAccount) with ErrPasswordTokenExpired -> error returned, no Login.
- Existing removed-paid-app test (line ~181, logins == 0) keeps passing; update its message check from "probably a paid app you didn't buy" to the new wording.
- errorText tests for the three changed/new messages.
- go vet, go test in gui/.

**Docs:** README line ~311 ("Same behavior as the CLI ... including signing in again and retrying when Apple reports an expired password token ... One exception ..."): say the GUI does NOT sign in again on its own, because with two-factor that can't finish and only sends a prompt to your devices; it asks you to log in again on the Account page. Drop the "One exception" sentence (now the general rule) and mention the reworded license message. Also update [[plan-no-relogin-on-refused-license]] with a pointer here.

**Then:** build both programs into releases\ipatool ([[build-output-folder]]); small separate commits on main: fix (backend), messages, tests if separate, README, memory ([[no-coauthor]]); ask before pushing. No real downloads to test ([[protect-apple-account]]).

**Related:** [[plan-merge-upstream-store-frameworks]] (when merging upstream again, keep this behaviour: the new engine must not reach Apple's own sign-in flow from a download).
