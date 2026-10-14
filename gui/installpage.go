package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tailscale/walk"
	. "github.com/tailscale/walk/declarative"
	"github.com/tailscale/win"
)

// The Download page's Install on iPhone button and check box (install.go does
// the work). Installing never uses the Apple account, so it needs no passphrase.

// chooseAndInstall asks for an .ipa, offering the one downloaded last, and installs it.
func (g *gui) chooseAndInstall() {
	d := &g.download
	dlg := walk.FileDialog{
		Title:          "Choose an app to install on your iPhone",
		Filter:         "iPhone apps (*.ipa)|*.ipa",
		InitialDirPath: strings.TrimSpace(d.output.Text()),
	}
	if d.lastFile != "" {
		if _, err := os.Stat(d.lastFile); err == nil {
			dlg.FilePath = d.lastFile
			dlg.InitialDirPath = filepath.Dir(d.lastFile)
		}
	}
	if ok, _ := dlg.ShowOpen(g.mw); ok {
		g.startInstall(dlg.FilePath, "")
	}
}

// installCheck is what startInstall finds before installing. phoneErr is kept
// apart so warnings about the file come first: answering No to them makes the
// phone irrelevant.
type installCheck struct {
	signature signatureInfo
	phones    []phone
	phoneErr  error
}

// startInstall checks the file and finds the phone, warns about anything the
// phone would likely refuse, then installs. prefix starts every Result message
// (after a download it says where the file was saved).
func (g *gui) startInstall(file, prefix string) {
	d := &g.download
	_ = d.result.SetText(prefix + "Checking the app and looking for your iPhone.")
	g.runLocalTask("check "+file+" and look for iPhones", "Checking the app.",
		func(ctx context.Context) (any, error) {
			signature, err := checkSignature(file)
			if err != nil {
				return nil, err
			}
			phones, err := connectedPhones()
			return installCheck{signature, phones, err}, nil
		},
		func(result any, err error) {
			if err != nil {
				g.installFailed(prefix, err)
				return
			}
			check := result.(installCheck)
			w := check.signature.warnings()
			if otherAppleID(check.signature.AppleID, g.accountEmail) {
				w = append(w, msgOtherAppleID)
			}
			if len(w) > 0 {
				message := strings.Join(w, "\n\n") + "\n\nInstall anyway?"
				if walk.MsgBox(g.mw, "Check before installing", message,
					walk.MsgBoxYesNo|walk.MsgBoxIconWarning|walk.MsgBoxDefButton2) != win.IDYES {
					_ = d.result.SetText(prefix + "Not installed.")
					return
				}
			}
			if check.phoneErr != nil {
				g.log("error: " + check.phoneErr.Error())
				g.setStatus("Failed.")
				g.installFailed(prefix, check.phoneErr)
				return
			}
			p := check.phones[0]
			if len(check.phones) > 1 {
				var ok bool
				if p, ok = g.choosePhone(check.phones); !ok {
					_ = d.result.SetText(prefix + "Not installed.")
					return
				}
			}
			g.install(file, p, prefix)
		})
	if g.busy {
		_ = d.result.SetFocus()
	}
}

// install sends the file to the phone, showing the phone's progress in the
// Result field and the status bar.
func (g *gui) install(file string, p phone, prefix string) {
	d := &g.download
	name := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	percent, sending := 0, false
	_ = d.result.SetText(prefix + fmt.Sprintf("Sending %s to %s.", name, p.Name))
	g.runLocalTask(fmt.Sprintf("install %s on %s", file, p), "Installing on iPhone.",
		func(ctx context.Context) (any, error) {
			return nil, installIPA(ctx, p, file, func(r installProgress) {
				g.app.Synchronize(func() {
					if !g.busy {
						return
					}
					percent, sending = r.Percent, r.Sending
					_ = d.result.SetText(prefix + installProgressText(p.Name, r))
				})
			})
		},
		func(_ any, err error) {
			if err != nil {
				g.installFailed(prefix, err)
				return
			}
			text := prefix + fmt.Sprintf("Installed %s on %s.", name, p.Name)
			_ = d.result.SetText(text)
			g.info("Install finished", text)
		})
	if g.busy {
		_ = d.result.SetFocus()
		g.busyProgress = func() string {
			if percent == 0 {
				return ""
			}
			if sending {
				return fmt.Sprintf("sending, %d percent", percent)
			}
			return fmt.Sprintf("%d percent", percent)
		}
	}
}

// installProgressText describes install progress for the Result field, for
// example "Sending to Alex's iPhone: 45 percent, 120 of 260 MB." and then
// "Installing on Alex's iPhone: 40 percent, checking the app's signature."
func installProgressText(phoneName string, r installProgress) string {
	if r.Sending {
		return fmt.Sprintf("Sending to %s: %d percent, %s of %s MB.", phoneName, r.Percent,
			megabytes(r.Sent), megabytes(r.Total))
	}
	return fmt.Sprintf("Installing on %s: %d percent, %s.", phoneName, r.Percent, strings.ToLower(r.Stage))
}

// megabytes formats a size in whole MB, or with one decimal below 10 MB.
func megabytes(bytes uint64) string {
	mb := float64(bytes) / 1e6
	if mb < 10 {
		return fmt.Sprintf("%.1f", mb)
	}
	return fmt.Sprintf("%.0f", mb)
}

func (g *gui) installFailed(prefix string, err error) {
	text := installErrorText(err)
	_ = g.download.result.SetText(prefix + "Install failed: " + text)
	g.error("Install failed", text)
}

// choosePhone asks which phone to install on when more than one is connected.
func (g *gui) choosePhone(phones []phone) (phone, bool) {
	var dlg *walk.Dialog
	var list *walk.ListBox
	var installButton, cancelButton *walk.PushButton
	names := make([]string, len(phones))
	for i, p := range phones {
		names[i] = p.String()
	}
	chosen := -1
	accept := func() {
		if i := list.CurrentIndex(); i >= 0 {
			chosen = i
			dlg.Accept()
		}
	}
	err := Dialog{
		AssignTo:      &dlg,
		Title:         "Choose a device",
		DefaultButton: &installButton,
		CancelButton:  &cancelButton,
		MinSize:       Size{Width: 400, Height: 260},
		Layout:        VBox{},
		Children: []Widget{
			Label{Text: "Connected &devices:"},
			ListBox{AssignTo: &list, Model: names, Accessibility: accessible("Connected &devices:"),
				OnItemActivated: accept},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				HSpacer{},
				PushButton{AssignTo: &installButton, Text: "&Install", OnClicked: accept},
				PushButton{AssignTo: &cancelButton, Text: "Cancel", OnClicked: func() { dlg.Cancel() }},
			}},
		},
	}.Create(g.mw)
	if err != nil {
		return phone{}, false
	}
	assignControlIDs(dlg.Handle())
	_ = list.SetCurrentIndex(0)
	_ = list.SetFocus()
	dlg.Run()
	if chosen < 0 {
		return phone{}, false
	}
	return phones[chosen], true
}
