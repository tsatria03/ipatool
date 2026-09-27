package main

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/tailscale/walk"
	. "github.com/tailscale/walk/declarative"
)

type accountPage struct {
	exe, passphrase, email, password, status *walk.LineEdit
	remember                                 *walk.CheckBox
}

func (g *gui) accountTab() TabPage {
	a := &g.account

	rows := []Widget{}
	rows = append(rows, labeled("ipatool &program (exe or source folder):",
		LineEdit{AssignTo: &a.exe, Text: resolveExe(g.settings.Exe), Accessibility: accessible("ipatool &program (exe or source folder):")},
		PushButton{Text: "&Browse...", OnClicked: g.browseExe})...)
	rows = append(rows, labeled("&Keychain passphrase (not your Apple ID password):",
		LineEdit{AssignTo: &a.passphrase, Text: g.settings.Passphrase, PasswordMode: true,
			Accessibility: accessible("&Keychain passphrase (not your Apple ID password):")},
		CheckBox{AssignTo: &a.remember, Text: "Re&member passphrase", Checked: g.settings.Passphrase != ""})...)
	rows = append(rows, labeled("Apple ID &email:",
		LineEdit{AssignTo: &a.email, Text: g.settings.Email, Accessibility: accessible("Apple ID &email:")}, nil)...)
	rows = append(rows, labeled("Apple ID pass&word:",
		LineEdit{AssignTo: &a.password, PasswordMode: true, Accessibility: accessible("Apple ID pass&word:"),
			OnKeyDown: func(key walk.Key) {
				if key == walk.KeyReturn {
					g.login()
				}
			}}, nil)...)

	return TabPage{
		Title:  "Account",
		Layout: VBox{},
		Children: []Widget{
			Composite{Layout: Grid{Columns: 3, MarginsZero: true}, Children: rows},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				PushButton{Text: "&Log in", OnClicked: g.login},
				PushButton{Text: "&Check account", OnClicked: func() { g.checkAccount(true) }},
				PushButton{Text: "Log &out", OnClicked: g.logout},
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
	args := []string{"auth", "login", "-e", email, "-p", password}
	g.run(args, "Logging in. The first login can take a minute.", true, func(r Result) { g.onLogin(r, args) })
}

func (g *gui) onLogin(result Result, args []string) {
	if !result.OK() {
		g.error("Login failed", result.Error())
		return
	}
	alreadySentCode := false
	for _, arg := range args {
		alreadySentCode = alreadySentCode || arg == "--auth-code"
	}
	if result.HasMessage(twoFAHint) {
		if alreadySentCode {
			g.error("Login failed", "The code was not accepted. Try logging in again.")
			return
		}
		code, ok := prompt(g.mw, "Two-factor code",
			"Apple sent a 6-digit code to your trusted devices. Enter it here:")
		code = strings.Map(func(r rune) rune {
			if unicode.IsDigit(r) {
				return r
			}
			return -1
		}, code)
		if !ok || code == "" {
			g.setStatus("Login cancelled.")
			return
		}
		retry := append(append([]string{}, args...), "--auth-code", code)
		g.run(retry, "Verifying code.", true, func(r Result) { g.onLogin(r, retry) })
		return
	}
	_ = g.account.password.SetText("")
	g.showAccount(result)
	g.info("Logged in", g.account.status.Text())
}

// checkAccount runs `auth info`. When announce is true, focus moves to the
// Account status field so screen readers read the result aloud.
func (g *gui) checkAccount(announce bool) {
	g.run([]string{"auth", "info"}, "Checking account.", true, func(result Result) {
		if result.OK() {
			g.showAccount(result)
		} else {
			_ = g.account.status.SetText(result.Error())
		}
		g.setStatus(g.account.status.Text())
		if announce {
			g.focusWidget(0, g.account.status)
			g.account.status.SetTextSelection(0, -1)
		}
	})
}

func (g *gui) showAccount(result Result) {
	name, email := result.Str("name"), result.Str("email")
	if email != "" {
		_ = g.account.status.SetText(fmt.Sprintf("Signed in as %s (%s)", name, email))
	} else {
		_ = g.account.status.SetText("Signed in.")
	}
}

func (g *gui) logout() {
	if walk.MsgBox(g.mw, "Log out", "Remove the saved login from this computer?",
		walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
		return
	}
	g.run([]string{"auth", "revoke"}, "Logging out.", true, func(result Result) {
		if !result.OK() {
			g.error("Log out failed", result.Error())
			return
		}
		_ = g.account.status.SetText("Signed out.")
		g.info("Logged out", "You are signed out.")
	})
}

func (g *gui) browseExe() {
	dlg := walk.FileDialog{Title: "Choose ipatool.exe", Filter: "Programs (*.exe)|*.exe"}
	if ok, _ := dlg.ShowOpen(g.mw); ok {
		_ = g.account.exe.SetText(dlg.FilePath)
		g.persist()
	}
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
