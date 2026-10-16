---
name: plan-no-relogin-on-refused-license
description: "Plan/fix - when a removed app's price is unknown and Apple refuses the free license (sign-in required), stop with a plain message instead of re-logging in (which sent a surprise 2FA prompt to the phone); IN PROGRESS 2026-10-14"
metadata:
  node_type: memory
  type: project
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-10-15T00:01:48.370Z
---

**Status:** COMPLETED 2026-10-14 (ab17394 fix, 342e199 docs; not pushed); both exes rebuilt into releases\ipatool. Test TestRemovedPaidAppNoRelogin (fakeStore gained refuse + Login counting).

**Why (2026-10-14):** the user downloaded Sound Shaker by zinc Roe (336912043, $1.99, removed, NOT owned) by App ID with "Get a free license" ticked. Price unknown (lookup has no removed apps) -> backend.download tried Purchase with price 0 -> Apple answered sign-in required (FailureTypeSignInRequired) -> engine maps it to ErrPasswordTokenExpired -> the GUI's retry signed in again with the saved password -> Apple asked for 2FA -> "auth code is required" and an Apple ID sign-in prompt popped up on the user's iPhone. Nothing was charged. The user approved the fix without discussion ("Please plan it, then build the fix").

**Fix (gui/backend.go download):** in the purchase step, when the price is unknown (!priceKnown) and Purchase fails with ErrPasswordTokenExpired, return a new errNoFreeLicense right away - no re-login, no retry. Message (gui/messages.go errorText): "Your account doesn't own this app, and Apple wouldn't give a free license, so it was probably a paid app you didn't buy. It can only be downloaded if your account bought it." Real token expiry elsewhere (download step, known-price purchases) keeps the re-login behaviour.

**Tests (gui/errors_test.go fakeStore):** removed app by numeric ID + license required + Purchase returning wrapped ErrPasswordTokenExpired -> errNoFreeLicense, Login never called, one purchase attempt; errorText test for the new message. Then README "How it works" bullet if it describes the retry, build into releases\ipatool, small commits ([[no-coauthor]]). Related: [[protect-apple-account]], [[plan-removed-app-downloads]].
