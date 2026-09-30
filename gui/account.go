package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/majd/ipatool/v2/pkg/appstore"
	"github.com/tailscale/walk"
	. "github.com/tailscale/walk/declarative"
	"github.com/tailscale/win"
)

type accountPage struct {
	passphrase, email, password, status *walk.LineEdit
	remember, showPassphrase            *walk.CheckBox
	showPassword                        *walk.CheckBox
	emailLabel, passwordLabel           *walk.Label
	login, logout                       *walk.PushButton
}

// accountState is whether an account is signed in, as far as the GUI knows.
type accountState int8

const (
	accountUnknown   accountState = iota // not checked yet
	accountSignedIn                      // a check or login succeeded
	accountSignedOut                     // logged out, or the check found no usable login
)

func (g *gui) accountTab() TabPage {
	a := &g.account

	rows := []Widget{}
	rows = append(rows, labeled("&Keychain passphrase (not your Apple ID password):",
		LineEdit{AssignTo: &a.passphrase, Text: g.settings.Passphrase, PasswordMode: true,
			Accessibility: accessible("&Keychain passphrase (not your Apple ID password):")},
		Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
			CheckBox{AssignTo: &a.remember, Text: "Re&member passphrase", Checked: g.settings.Passphrase != ""},
			CheckBox{AssignTo: &a.showPassphrase, Text: "Show &passphrase",
				OnCheckedChanged: func() { reveal(a.passphrase, a.showPassphrase.Checked()) }},
		}})...)
	rows = append(rows,
		Label{AssignTo: &a.emailLabel, Text: "Apple ID &email:"},
		LineEdit{AssignTo: &a.email, Text: g.settings.Email, Accessibility: accessible("Apple ID &email:")},
		HSpacer{},
		Label{AssignTo: &a.passwordLabel, Text: "Apple ID pass&word:"},
		LineEdit{AssignTo: &a.password, PasswordMode: true, Accessibility: accessible("Apple ID pass&word:"),
			OnKeyDown: func(key walk.Key) {
				if key == walk.KeyReturn {
					g.login()
				}
			}},
		CheckBox{AssignTo: &a.showPassword, Text: "S&how password",
			OnCheckedChanged: func() { reveal(a.password, a.showPassword.Checked()) }},
	)

	return TabPage{
		Title:  "Account",
		Layout: VBox{},
		Children: []Widget{
			Composite{Layout: Grid{Columns: 3, MarginsZero: true}, Children: rows},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				PushButton{AssignTo: &a.login, Text: "&Log in", OnClicked: g.login},
				PushButton{Text: "&Check account", OnClicked: func() { g.checkAccount(true) }},
				PushButton{AssignTo: &a.logout, Text: "Log &out", OnClicked: g.logout},
				HSpacer{},
			}},
			Composite{Layout: Grid{Columns: 3, MarginsZero: true}, Children: labeled("Account &status:",
				LineEdit{AssignTo: &a.status, ReadOnly: true, Text: "Not checked yet.",
					Accessibility: accessible("Account &status:")}, nil)},
			Label{Text: "The keychain passphrase protects the file where ipatool saves your login. Pick any " +
				"passphrase the first time you log in, then always use the same one. \"Remember passphrase\" " +
				"saves it unencrypted in your AppData folder."},
			VSpacer{},
		},
	}
}

func (g *gui) login() {
	a := &g.account
	email, password := strings.TrimSpace(a.email.Text()), a.password.Text()
	if email == "" || password == "" {
		g.error("Missing details", "Enter your Apple ID email and password.")
		return
	}
	g.persist()
	g.runTask("log in as "+email, "Logging in. The first login can take a minute.",
		func(ctx context.Context, b *backend) (any, error) {
			return b.login(email, password, g.askTwoFactorCode)
		},
		func(result any, err error) {
			switch {
			case errors.Is(err, errLoginCancelled):
				g.setStatus("Login cancelled.")
			case err != nil:
				g.error("Login failed", errorText(err))
			default:
				_ = g.account.password.SetText("")
				g.showAccount(result.(appstore.Account))
				g.setAccountState(accountSignedIn)
				// The message box returns focus here, so the status is read again.
				_ = g.account.status.SetFocus()
				g.info("Logged in", g.account.status.Text())
			}
		})
}

// setAccountState shows only the controls that apply: while signed in, the
// Apple ID email and password and Log in are hidden; once signed out, Log out
// is. Before the first check everything is shown, since a saved login may
// exist. Focus never stays on a control that was just hidden.
func (g *gui) setAccountState(state accountState) {
	a := &g.account
	signedIn := state == accountSignedIn
	if signedIn {
		// Hide the typed password again, so it's hidden when the box comes back.
		a.showPassword.SetChecked(false)
	}
	for _, w := range []walk.Widget{a.emailLabel, a.email, a.passwordLabel, a.password, a.showPassword, a.login} {
		w.SetVisible(!signedIn)
	}
	a.logout.SetVisible(state != accountSignedOut)
	if focused := win.GetFocus(); focused != 0 && !win.IsWindowVisible(focused) {
		if signedIn {
			_ = a.status.SetFocus()
		} else {
			_ = a.email.SetFocus()
		}
	}
}

// reveal shows what was typed in a password box, or hides it again with the
// same character the box started with.
func reveal(box *walk.LineEdit, show bool) {
	box.SetPasswordMode(!show)
	box.Invalidate() // Windows doesn't redraw the text by itself
}

// askTwoFactorCode is called from the login task's goroutine: it shows the code
// prompt on the UI thread and waits for the answer. Only digits are kept.
func (g *gui) askTwoFactorCode() (string, bool) {
	type answer struct {
		code string
		ok   bool
	}
	answers := make(chan answer)
	g.app.Synchronize(func() {
		code, ok := prompt(g.mw, "Two-factor code", "Apple sent a 6-digit code to your trusted devices. Enter it here:")
		answers <- answer{strings.Map(func(r rune) rune {
			if unicode.IsDigit(r) {
				return r
			}
			return -1
		}, code), ok}
	})
	a := <-answers
	return a.code, a.ok
}

// checkAccount shows the signed-in account. When announce is true, focus moves to
// the Account status field so screen readers read the result aloud.
func (g *gui) checkAccount(announce bool) {
	g.runTask("check account", "Checking account.",
		func(ctx context.Context, b *backend) (any, error) { return b.accountInfo() },
		func(result any, err error) {
			if err != nil {
				_ = g.account.status.SetText(errorText(err))
				g.setAccountState(accountSignedOut)
			} else {
				g.showAccount(result.(appstore.Account))
				g.setAccountState(accountSignedIn)
			}
			g.setStatus(g.account.status.Text())
			if announce {
				g.focusWidget(0, g.account.status)
				g.account.status.SetTextSelection(0, -1)
			}
		})
}

func (g *gui) showAccount(acc appstore.Account) {
	if acc.Email != "" {
		_ = g.account.status.SetText(fmt.Sprintf("Signed in as %s (%s)", acc.Name, acc.Email))
	} else {
		_ = g.account.status.SetText("Signed in.")
	}
}

func (g *gui) logout() {
	if walk.MsgBox(g.mw, "Log out", "Remove the saved login from this computer?",
		walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
		return
	}
	g.runTask("log out", "Logging out.",
		func(ctx context.Context, b *backend) (any, error) { return nil, b.logout() },
		func(_ any, err error) {
			if err != nil {
				g.error("Log out failed", errorText(err))
				return
			}
			_ = g.account.status.SetText("Signed out.")
			g.setAccountState(accountSignedOut)
			_ = g.account.email.SetFocus() // ready to log in again
			g.info("Logged out", "You are signed out.")
		})
}

// prompt shows a small dialog with one text box and returns what was typed.
func prompt(owner walk.Form, title, label string) (string, bool) {
	var dlg *walk.Dialog
	var edit *walk.LineEdit
	var okButton, cancelButton *walk.PushButton
	text := ""

	err := Dialog{
		AssignTo:      &dlg,
		Title:         title,
		DefaultButton: &okButton,
		CancelButton:  &cancelButton,
		MinSize:       Size{Width: 360, Height: 130},
		Layout:        VBox{},
		Children: []Widget{
			Label{Text: label},
			LineEdit{AssignTo: &edit, Accessibility: Accessibility{Name: label}},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				HSpacer{},
				PushButton{AssignTo: &okButton, Text: "OK", OnClicked: func() {
					text = edit.Text()
					dlg.Accept()
				}},
				PushButton{AssignTo: &cancelButton, Text: "Cancel", OnClicked: func() { dlg.Cancel() }},
			}},
		},
	}.Create(owner)
	if err != nil {
		return "", false
	}
	assignControlIDs(dlg.Handle())
	_ = edit.SetFocus()
	accepted := dlg.Run() == walk.DlgCmdOK
	return text, accepted
}
