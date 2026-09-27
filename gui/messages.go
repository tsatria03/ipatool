package main

import (
	"errors"
	"strings"

	"github.com/majd/ipatool/v2/pkg/appstore"
)

// Plain-language error messages. Each says what happened and what to do next;
// the raw error is still written to the Log page.

const (
	msgPaidApp = "This is a paid app that your Apple Account hasn't bought. Buy it once on your iPhone, iPad " +
		"or Mac; it will then appear in My apps and you can download it here."
	msgNotOwned = "Your account doesn't own this app. If it's free, check \"Get a free license if needed\" and " +
		"download again. If it's paid, buy it once on your iPhone, iPad or Mac first."
)

// errorText turns an error from ipatool's engine into the message shown to the user.
func errorText(err error) string {
	switch {
	case errors.Is(err, errPaidApp):
		return msgPaidApp
	case errors.Is(err, errFreeNotOwned):
		return "Your account doesn't own this app yet. It's free: check \"Get a free license if needed\" and download again."
	case errors.Is(err, appstore.ErrLicenseRequired):
		return msgNotOwned
	case errors.Is(err, appstore.ErrPasswordTokenExpired):
		return "Your sign-in has expired. Log in again on the Account page (Ctrl+1)."
	case errors.Is(err, appstore.ErrSubscriptionRequired):
		return "This app needs a subscription, such as Apple Arcade, that your account doesn't have."
	case errors.Is(err, appstore.ErrTemporarilyUnavailable):
		return "This app is temporarily unavailable from the App Store. Try again later."
	case errors.Is(err, errCodeNotAccepted):
		return "Apple didn't accept the two-factor code. Log in again and use the newest code."
	}
	return friendlyError(err.Error())
}

// friendlyError matches known error text, for errors the engine doesn't export
// as values (and errors that only arrive as text).
func friendlyError(text string) string {
	lowered := strings.ToLower(strings.TrimSpace(text))
	has := func(parts ...string) bool {
		for _, p := range parts {
			if strings.Contains(lowered, p) {
				return true
			}
		}
		return false
	}
	switch {
	case has("purchasing paid apps is not supported"):
		return msgPaidApp
	case has("license is required"):
		return msgNotOwned
	case has("could not be found in the keyring"):
		return "You are not signed in. Log in on the Account page first (Ctrl+1)."
	case has("integrity check failed"):
		return "The keychain passphrase is wrong. Use the same one you chose when you first logged in."
	case has("keychain passphrase is required"):
		return "Enter a keychain passphrase on the Account page."
	case has("password token is expired"):
		return "Your sign-in has expired. Log in again on the Account page (Ctrl+1)."
	case has("apple did not complete verification"):
		return "Apple didn't accept the two-factor code. Log in again and use the newest code."
	case has("2fa code must contain exactly six digits"):
		return "The two-factor code must be exactly 6 digits. Log in again and enter the code."
	case has("account is disabled"):
		return "Apple says this Apple Account is disabled. Sign in at appleid.apple.com to find out why."
	case has("too many attempts"):
		return "Apple stopped the sign-in after too many attempts. Wait a while, then try again."
	case has("no usable authentication response"):
		return "Apple didn't finish signing you in. Try again later, or from another network."
	case has("platform version lookup returned no app", "platform version lookup returned no offers"):
		return "Apple has no version of this app for the selected platform. Try another platform."
	case has("no such host", "dial tcp", "connection refused", "connection reset", "i/o timeout",
		"tls handshake timeout", "network is unreachable"):
		return "Couldn't reach Apple. Check your internet connection and try again."
	case lowered == "invalid response" || strings.HasSuffix(lowered, ": invalid response"):
		return "Apple didn't send a download for this app. Check that the platform is right, or try again later."
	case has("app not found"):
		return "Apple couldn't find that app. Check the bundle ID. If the app was removed from the App Store but you own it, " +
			"enter its App ID instead (My apps shows it), or send it from My apps with Download selected."
	case lowered == "something went wrong" || strings.HasSuffix(lowered, ": something went wrong"):
		return "Apple reported an error without details. Try again later."
	}
	return strings.TrimSpace(text)
}
