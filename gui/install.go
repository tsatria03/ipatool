package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/zipconduit"
)

// Installing on an iPhone goes over USB through Apple Mobile Device Service
// (part of Apple Devices or iTunes), the same way iTunes and Xcode install apps.
// The .ipa is sent unchanged: Apple's signature and the App Store license stay
// as they are, and nothing is sent to Apple.

var (
	errNoPhone    = errors.New("no iPhone is connected")
	errNotTrusted = errors.New("the iPhone doesn't trust this computer yet")
)

// phone is one connected iPhone, iPad or iPod touch.
type phone struct {
	entry   ios.DeviceEntry
	Name    string
	Model   string // Apple's model identifier, like iPhone15,2
	Version string // iOS version, like 27.0.1
}

func (p phone) String() string {
	return fmt.Sprintf("%s, iOS %s", p.Name, p.Version)
}

// connectedPhones lists the devices connected by USB. A device that hasn't
// trusted this computer yet is asked to (the Trust prompt appears on it), and
// errNotTrusted is returned so the user can answer it and try again.
func connectedPhones() ([]phone, error) {
	list, err := ios.ListDevices()
	if err != nil {
		return nil, err
	}
	if len(list.DeviceList) == 0 {
		return nil, errNoPhone
	}
	phones := make([]phone, 0, len(list.DeviceList))
	for _, entry := range list.DeviceList {
		if _, err := ios.ReadPairRecord(entry.Properties.SerialNumber); err != nil {
			if err := ios.Pair(entry); err != nil {
				return nil, fmt.Errorf("%w: %v", errNotTrusted, err)
			}
		}
		values, err := ios.GetValues(entry)
		if err != nil {
			return nil, err
		}
		phones = append(phones, phone{
			entry:   entry,
			Name:    values.Value.DeviceName,
			Model:   values.Value.ProductType,
			Version: values.Value.ProductVersion,
		})
	}
	return phones, nil
}

// installMu allows one install at a time: go-ios reports progress only through
// its logger, which is shared by the whole program.
var installMu sync.Mutex

// installIPA sends the .ipa at path to the phone and waits until the phone has
// installed it. progress gets the phone's own stages (percent and a short
// description). Cancelling ctx closes the connection; if the phone has already
// received the whole file it may still finish installing.
func installIPA(ctx context.Context, p phone, path string, progress func(percent int, stage string)) error {
	installMu.Lock()
	defer installMu.Unlock()

	ios.SetLogger(slog.New(progressHandler{report: progress}))
	defer ios.SetLogger(nil)

	conn, err := zipconduit.New(p.entry)
	if err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- conn.SendFile(path) }()
	select {
	case err := <-done:
		conn.Close()
		return err
	case <-ctx.Done():
		conn.Close()
		<-done
		return ctx.Err()
	}
}

// progressHandler picks go-ios's "installing" log records, which carry the
// phone's progress, and drops all other logging.
type progressHandler struct {
	report func(percent int, stage string)
}

func (h progressHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h progressHandler) Handle(_ context.Context, r slog.Record) error {
	if h.report == nil || r.Message != "installing" {
		return nil
	}
	var status string
	percent := -1
	r.Attrs(func(a slog.Attr) bool {
		switch a.Key {
		case "status":
			status = a.Value.String()
		case "percentComplete":
			if a.Value.Kind() == slog.KindInt64 {
				percent = int(a.Value.Int64())
			}
		}
		return true
	})
	if percent >= 0 {
		h.report(percent, installStage(status))
	}
	return nil
}

func (h progressHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h progressHandler) WithGroup(string) slog.Handler      { return h }

// installStage puts the phone's install stages into plain words.
func installStage(status string) string {
	switch status {
	case "CreatingStagingDirectory", "ExtractingPackage":
		return "Unpacking the app"
	case "InspectingPackage", "PreflightingApplication", "VerifyingApplication":
		return "Checking the app's signature"
	case "CreatingContainer", "InstallingApplication", "PostflightingApplication":
		return "Installing"
	case "SandboxingApplication", "GeneratingApplicationMap":
		return "Finishing"
	case "InstallComplete":
		return "Done"
	}
	return "Installing"
}

// installErrorText turns an error from connecting or installing into the
// message shown to the user. The phone's error codes are matched, never go-ios's
// own wording around them (it blames "codesigning and provisioning profile" for
// every refusal).
func installErrorText(err error) string {
	switch {
	case errors.Is(err, errNoPhone):
		return "No iPhone found. Connect it with a USB cable, unlock it, and try again."
	case errors.Is(err, errNotTrusted):
		return "Unlock your iPhone and tap Trust when it asks whether to trust this computer, then try again."
	case errors.Is(err, context.Canceled):
		return "Install cancelled."
	}
	lowered := strings.ToLower(err.Error())
	has := func(parts ...string) bool {
		for _, p := range parts {
			if strings.Contains(lowered, p) {
				return true
			}
		}
		return false
	}
	switch {
	case has("usbmuxd socket", "127.0.0.1:27015"):
		return "Apple Mobile Device Service isn't running. Install Apple Devices from the Microsoft Store (or iTunes), " +
			"then connect your iPhone and try again."
	case has("is the device paired", "pairrecord", "pairingdialog", "userdeniedpairing", "invalidhostid"):
		return "Unlock your iPhone and tap Trust when it asks whether to trust this computer, then try again."
	case has("passwordprotected", "device is locked"):
		return "Unlock your iPhone, then try again."
	case has("0xe8008029"):
		return "Your iPhone refused this version. Its signature is too old for your iOS version. Try a newer version of the app."
	case has("deviceosversiontoolow"):
		return "This app needs a newer iOS version than your iPhone has."
	case has("incorrectarchitecture", "badcputype"):
		return "This app is 32-bit only. iPhones can't run 32-bit apps since iOS 11."
	case has("insufficient", "no space left", "enospc", "diskfull"):
		return "Your iPhone doesn't have enough free space for this app. Free up some space and try again."
	case has("applicationverificationfailed"):
		return "Your iPhone couldn't verify this app's signature. The file may be damaged; download it again."
	case has("not a valid zip file", "zip: "):
		return "This file isn't a complete .ipa. Download it again."
	case has("the system cannot find the file", "no such file"):
		return "The .ipa file wasn't found. It may have been moved or deleted."
	case has("forcibly closed", "broken pipe", "connection reset", "eof"):
		return "The connection to your iPhone was lost. Check the USB cable, unlock your iPhone, and try again."
	}
	return "Couldn't install the app on your iPhone: " + strings.TrimSpace(err.Error())
}
