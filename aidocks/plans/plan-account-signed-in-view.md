---
name: plan-account-signed-in-view
description: "Plan (2026-09-29, written, NOT approved) - Account page hides the Apple ID password field and Log in button while an account is signed in"
metadata:
  node_type: memory
  type: project
  originSessionId: dd28134d-18dd-41c0-9cf9-7e77df2fdbf4
  modified: 2026-09-30T04:08:34.830Z
---

**Status:** COMPLETED 2026-09-29 (93eed55 feature, 17016a3 README; pushed; releases\ rebuilt). account-view-test.ps1 passed: signed-in view (email/password/Log in hidden, Log out shown), wrong passphrase + F5 -> signed-out view (Log out hidden), correct passphrase + F5 -> signed-in again, focus on Account status, Show boxes toggle protected<->readable, hiding char stays '*' (reveal uses walk SetPasswordMode). Test lessons: other processes can't WM_GETTEXT a password edit (returns ""), so the test reads the real passphrase from settings-backup.json (never printed); the edit's accessible name is "Keychain passphrase (not your Apple ID password)". User suggestion: "The apple id password field, and the log in option should not appear if your account is already signed into the program." Follow [[plans-in-memory-first]], [[gui-tests-hidden]], [[no-coauthor]], [[no-pickers]], [[screen-reader-user]].

## Findings (gui/account.go read 2026-09-29)
- accountPage: passphrase, email, password, status LineEdits + remember CheckBox; buttons Log in (Alt+L), Check account (Alt+C), Log out (Alt+O) are unnamed (no AssignTo). Rows built with labeled() -> Label + control + extra in a 3-column Grid.
- Signed-in state is only known after checkAccount (startup check when a passphrase is saved, F5, Check account) or login; logout sets "Signed out.". A check error (no saved login, wrong passphrase) leaves the state unknown.

## Design
- New accountPage fields: passwordLabel *walk.Label, loginButton (and logoutButton) *walk.PushButton; func (g *gui) setSignedIn(signedIn bool).
- setSignedIn(true) after a successful check or login: hide the password label + field and Log in (SetVisible(false) - hidden controls leave the Tab order and Alt+W/Alt+L stop working, as wanted). setSignedIn(false) after logout, or a check that fails: show them again.
- Startup / not checked yet: show everything (unknown).
- Focus: if the focused control is being hidden (after login: password field or Log in), move focus to Account status so NVDA reads "Signed in as ...". After logout, focus Apple ID password? (or keep on Log out's replacement) - decide in build; never leave focus on a hidden control.
- Enter in the password box still logs in when shown. F1 text + README Account section.

## Steps (after go-ahead)
1. [ ] setSignedIn + hooks in login/checkAccount/logout; unit-free (UI) - keep logic tiny.
2. [ ] hidden-desktop test: startup check with the saved login -> password + Log in hidden (check IsWindowVisible + Tab order), Check account keeps them hidden. Do NOT really log out or log in (would delete the login / send a 2FA code) - test the signed-out view with a wrong passphrase instead (check fails -> fields shown). Settings backed up/restored.
3. [ ] README + commit + push.

## Decided (2026-09-29)
User: "Yes, all this above. Record this into the plan." => option 2, everything:
- Signed in: hide Apple ID email (label + field), Apple ID password (label + field) and Log in. Show Log out.
- Signed out / check failed: show email, password and Log in; hide Log out.
- Unknown (startup before any check, no passphrase): show email, password and Log in; Log out hidden too? -> keep Log out VISIBLE while unknown (a saved login may exist even if the check hasn't run); hide it only after a confirmed signed-out state. Check account (Alt+C), passphrase, Remember passphrase and Account status always visible.
- Needs AssignTo for the email label/field row, password label, loginButton, logoutButton.
- Focus rules: never leave focus on a hidden control; after login -> Account status; after logout -> Apple ID email (so the user can type to log in again).
- ADDED 2026-09-29 (user): a way to reveal what was typed in the passphrase and Apple ID password fields ("a button to show/hide pass phrase/password", "it will reveal what you have typed in the text field").
  - walk has it: LineEdit.SetPasswordMode(bool) (EM_SETPASSWORDCHAR; walk uses '*'); call Invalidate() afterwards so it redraws.
  - Use CHECK BOXES (a toggle whose checked state NVDA announces): "Show &passphrase" (Alt+P) and "S&how password" (Alt+H) - Account page Alt letters already used: K M E W L C O S. Unchecked at every start (never remembered). Password's box hides together with the password field when signed in. No trailing punctuation.
  - Layout: passphrase row's 3rd column already holds Remember passphrase -> put Show passphrase next to it (HBox composite) or on its own row; password row's 3rd column gets Show password.
  - Test: toggle via BM_SETCHECK + BN_CLICKED, read EM_GETPASSWORDCHAR (0 = revealed), and MSAA role/state of the edit (protected vs not).
- Approval to BUILD not yet given explicitly (user only said record it) - ask "build now?" before starting.
