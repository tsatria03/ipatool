---
name: plan-merge-upstream-store-frameworks
description: Merge upstream 5dc4c80 (Apple store frameworks rewrite) and d9c0247 (compress bump); merged, pushed, then UNDONE 2026-10-31 (hard reset + force-push) because the new emulator breaks on Apple's sign-in dialogs; wait for an upstream fix
metadata:
  type: project
---

**Status:** UNDONE 2026-10-31, ON HOLD. Merged as 1550de0 (+ README commits 9cab964, 288f574 and plan notes), pushed, then the user asked for a hard reset to this plan's commit (8dea26c) and a force-push (--force-with-lease). Local branch backup-before-reset (5a02490) still holds the merged version; never push it. Don't merge upstream again until the bug below is fixed upstream (fetch and check its commits first).

**The bug that undid it:** after the merge the user tried to download iFoley Footsteps (App ID 907997410, $1.99, bought 2017, never refunded, dropped from the Purchased list by Apple). The GUI showed: "failed to send http request: request failed: store request: guest bridge at 0x...: runtime: unsupported instance message -[NSURLComponents setQuery:]; recent=[... AMSFinanceDialogResponse dialogRequest, AMSAuthenticateOptions setPromptTitle:/setReason: ...]". Apple answered with a "sign in to confirm" finance dialog; the old engine returned that as a plain failure (FailureTypeSignInRequired -> ErrPasswordTokenExpired) that the fork's errNoFreeLicense / errPaidApp handling relies on; the new engine runs Apple's own AMS code in internal/store/emulator, which needs NSURLComponents setQuery: (missing in network.go, around the NSURLComponents switch). Nothing was charged and no re-login happened.

**When merging again (after upstream fixes it):**
- Do NOT just add setQuery: ourselves: Apple's code would carry on into AMSAuthenticate, the path that sent a surprise 2FA prompt to the user's iPhone with Sound Shaker on 2026-10-14 ([[plan-no-relogin-on-refused-license]]).
- Check that the fork's error mapping still works with the new engine: sign-in required during Purchase/Download must still reach errNoFreeLicense / errPaidApp / errFreeNotOwned, without a re-login. gui/errors_test.go uses a fake store, so it can't catch engine changes; check the engine's error types by reading the code, not by sending requests to Apple ([[protect-apple-account]]).
- Merge fixes that worked last time: .github/workflows/*.yml stay deleted ([[fork-windows-only]]); .gitignore keeps the fork's lines plus upstream's .claude; gui/go.mod drops the "lzfse-cgo => ../internal/lzfsecompat" replace (upstream deleted that folder) and go mod tidy, inside the merge commit; engine tests need go generate ./... first (mocks are gitignored).
- README: remove the lzfse replacement sentence from "Shared login"; GUI size became about 54 MB (56,741,376 bytes).
- Rebuild both programs into releases\ipatool ([[build-output-folder]]).
