package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestInstallErrorText(t *testing.T) {
	tests := []struct {
		err  error
		want string // a phrase the message must contain
	}{
		{errNoPhone, "No iPhone found"},
		{fmt.Errorf("%w: Please accept the PairingDialog on the device and run pairing again!", errNotTrusted), "tap Trust"},
		{context.Canceled, "Install cancelled"},
		{errors.New("Could not connect to usbmuxd socket, is it running? dial tcp 127.0.0.1:27015: connectex: No connection could be made"), "Apple Devices"},
		{errors.New("could not retrieve PairRecord with error: ReadPair failed with errorcode '2', is the device paired?"), "tap Trust"},
		{errors.New("StartSession failed: map[Error:PasswordProtected] error: x"), "Unlock your iPhone, then"},
		// The real refusal from iOS 27 (Idle Crafting Kingdom 1.19.208, CodeDirectory v20200).
		{errors.New("your app is not properly signed for this device, check your codesigning and provisioningprofile. " +
			"original error: 'ApplicationVerificationFailed' errorDescription:'Failed to verify code signature of " +
			"/var/installd/Library/Caches/com.apple.mobile.installd.staging/temp.DEZEMz/extracted/Payload/CraftingKingdom.app : " +
			"0xe8008029 (The code signature version is no longer supported.)'"), "signature is too old"},
		{errors.New("original error: 'DeviceOSVersionTooLow' errorDescription:'x'"), "newer iOS version"},
		{errors.New("original error: 'IncorrectArchitecture' errorDescription:'x'"), "32-bit only"},
		{errors.New("original error: 'ApplicationVerificationFailed' errorDescription:'0xe8008017'"), "may be damaged"},
		// The real refusal from iOS 27 for an unsigned, self-built game.
		{errors.New("your app is not properly signed for this device, check your codesigning and provisioningprofile. " +
			"original error: 'ApplicationVerificationFailed' errorDescription:'Failed to verify code signature of " +
			"/var/installd/Library/Caches/com.apple.mobile.installd.staging/temp.jLCLFR/extracted/Payload/ClassicStand.app : " +
			"0xe800801c (No code signature found.)'"), "isn't signed for your iPhone"},
		{errors.New("the phone refused the app: 'ApplicationVerificationFailed' errorDescription:'Failed to verify code signature of x : " +
			"0xe8008015 (A valid provisioning profile for this executable was not found.)'"), "isn't signed for your iPhone"},
		{errors.New("zip: not a valid zip file"), "isn't a complete .ipa"},
		{errors.New("open x.ipa: The system cannot find the file specified."), "wasn't found"},
		{errors.New("write tcp: An existing connection was forcibly closed by the remote host."), "connection to your iPhone was lost"},
		{errors.New("something new"), "Couldn't install the app on your iPhone: something new"},
	}
	for _, tt := range tests {
		got := installErrorText(tt.err)
		if !strings.Contains(got, tt.want) {
			t.Errorf("installErrorText(%q) = %q, want it to contain %q", tt.err, got, tt.want)
		}
		if strings.Contains(strings.ToLower(got), "provisioning") {
			t.Errorf("installErrorText(%q) = %q, repeats go-ios's misleading provisioning text", tt.err, got)
		}
	}
}

func TestInstallStage(t *testing.T) {
	stages := []string{"CreatingStagingDirectory", "ExtractingPackage", "InspectingPackage", "PreflightingApplication",
		"VerifyingApplication", "CreatingContainer", "InstallingApplication", "PostflightingApplication",
		"SandboxingApplication", "GeneratingApplicationMap", "InstallComplete", "SomethingNew"}
	for _, s := range stages {
		if installStage(s) == "" {
			t.Errorf("installStage(%q) is empty", s)
		}
	}
}
