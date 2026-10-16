---
name: plan-clearer-errors
description: "Plan (2026-09-26, approved by the user, in progress) to make the GUI's error messages clearer, especially for paid apps the account hasn't bought"
metadata:
  node_type: memory
  type: project
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-09-27T05:23:36.165Z
---

**Status:** COMPLETED 2026-09-26 in f525999 (pushed). messages.go holds errorText/friendlyError; errPaidApp/errFreeNotOwned in backend.download (paid app stops before any purchase attempt); errors_test.go uses a fake appstore.AppStore; hidden test errors-test.ps1 passed (app not found box, wrong passphrase). Test-writing lesson: [T]::Find only sees visible controls — switch to the right page first. Do this before [[plan-my-apps-filter-reload]] (still awaiting go). Follow [[plans-in-memory-first]], [[gui-tests-hidden]], [[no-coauthor]].

## Goal
Every error box says in plain words what happened and what to do next. Raw engine text only as a last resort (it's still written to the Log page).

## Design
- `errorText(err)` (tasks.go) checks error values first with `errors.Is`, then known message text (`friendlyError`, settings.go), then falls back to the raw text.
- **Paid apps:** new `errPaidApp` in backend.go. In `download`, if Apple says a license is required (appstore.ErrLicenseRequired) and the looked-up app has Price > 0, return errPaidApp whether or not "Get a free license" is ticked. Also map the engine's "purchasing paid apps is not supported" (appstore_purchase.go:34, returned when Purchase sees Price > 0) to the same message. Message: "This is a paid app that your Apple Account hasn't bought. Buy it once on your iPhone, iPad or Mac; it will then appear in My apps and you can download it here."
- Messages (engine source in pkg/appstore):
  - ErrLicenseRequired when the price is 0 (box unticked): "Your account doesn't own this app yet. It's free: check "Get a free license if needed" and download again."
  - Review note: numeric app IDs aren't looked up in download, so the price is unknown; then use a general message: "Your account doesn't own this app. If it's free, check "Get a free license if needed" and download again. If it's paid, buy it once on your iPhone, iPad or Mac first." The app captured from Lookup must be kept outside the attempt closure to read its Price.
  - ErrPasswordTokenExpired: "Your sign-in has expired. Log in again on the Account page (Ctrl+1)."
  - ErrSubscriptionRequired: "This app needs a subscription, such as Apple Arcade, that your account doesn't have."
  - ErrTemporarilyUnavailable: "This app is temporarily unavailable from the App Store. Try again later."
  - errCodeNotAccepted / "apple did not complete verification": "Apple didn't accept the two-factor code. Log in again and use the newest code."
  - "2FA code must contain exactly six digits": "The two-factor code must be exactly 6 digits. Log in again and enter the code."
  - "account is disabled": "Apple says this Apple Account is disabled. Sign in at appleid.apple.com to find out why."
  - "too many attempts": "Apple stopped the sign-in after too many attempts. Wait a while, then try again."
  - "apple returned no usable authentication response": "Apple didn't finish signing you in. Try again later, or from another network."
  - "platform version lookup returned no app/offers": "Apple has no version of this app for the selected platform. Try another platform."
  - network failures (request failed + dial/no such host/timeout/connection refused/reset): "Couldn't reach Apple. Check your internet connection and try again."
  - "something went wrong" (Apple gave no details): "Apple reported an error without details. Try again later."
  - keep existing: not signed in (keyring item not found), wrong passphrase (integrity check failed), passphrase required, invalid response, app not found.
  - Wrong Apple ID password needs no mapping: login returns Apple's own customerMessage (appstore_login.go:306-308).

## Steps
1. [ ] Implement errPaidApp in backend.download, errorText with errors.Is + extended friendlyError.
2. [ ] Unit tests (gui/errors_test.go): table of errors -> messages, incl. wrapped errors and the paid-app rule. Hidden-desktop test: a real error box (download "com.example.doesnotexist" -> app not found message; wrong passphrase).
3. [ ] Commit and push.
