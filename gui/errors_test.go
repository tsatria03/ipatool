package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/majd/ipatool/v2/pkg/appstore"
)

func TestErrorText(t *testing.T) {
	tests := []struct {
		err  error
		want string // a phrase the message must contain
	}{
		{errPaidApp, "paid app that your Apple Account hasn't bought"},
		{errors.New("purchasing paid apps is not supported"), "paid app that your Apple Account hasn't bought"},
		{errFreeNotOwned, "It's free"},
		{fmt.Errorf("download failed: %w", appstore.ErrLicenseRequired), "If it's paid, buy it"},
		{appstore.ErrPasswordTokenExpired, "sign-in has expired"},
		{appstore.ErrSubscriptionRequired, "Apple Arcade"},
		{appstore.ErrTemporarilyUnavailable, "temporarily unavailable"},
		{errCodeNotAccepted, "newest code"},
		{errors.New("apple did not complete verification; try a fresh 2FA code"), "newest code"},
		{errors.New("2FA code must contain exactly six digits"), "exactly 6 digits"},
		{errors.New("account is disabled"), "appleid.apple.com"},
		{errors.New("too many attempts"), "too many attempts"},
		{errors.New("apple returned no usable authentication response (HTTP 500): x"), "didn't finish signing you in"},
		{errors.New("failed to get account: failed to get item: The specified item could not be found in the keyring"), "not signed in"},
		{errors.New("failed to get account: failed to get item: aes.KeyUnwrap(): integrity check failed."), "passphrase is wrong"},
		{errors.New("keychain passphrase is required"), "Enter a keychain passphrase"},
		{errors.New("app not found"), "couldn't find that app"},
		{errors.New("invalid response"), "didn't send a download"},
		{errors.New("app 1 in storefront US (catalogs: iphone): platform version lookup returned no app"), "another platform"},
		{errors.New(`request failed: Get "https://x": dial tcp: lookup x: no such host`), "Couldn't reach Apple"},
		{errors.New("something went wrong"), "without details"},
		{errors.New("Your Apple ID or password was entered incorrectly."), "Your Apple ID or password was entered incorrectly."},
	}
	for _, tt := range tests {
		if got := errorText(tt.err); !strings.Contains(got, tt.want) {
			t.Errorf("errorText(%q) = %q, want it to contain %q", tt.err, got, tt.want)
		}
	}
}

// fakeStore stands in for Apple: one app with the given price that the account
// doesn't own. Methods the download path doesn't use panic (nil embedded interface).
type fakeStore struct {
	appstore.AppStore
	price      float64
	owned      bool
	removed    bool // left the App Store: the lookup service no longer knows it
	purchases  int
	downloaded appstore.App
}

func (f *fakeStore) AccountInfo() (appstore.AccountInfoOutput, error) {
	return appstore.AccountInfoOutput{Account: appstore.Account{Email: "a@example.com"}}, nil
}

func (f *fakeStore) Lookup(appstore.LookupInput) (appstore.LookupOutput, error) {
	if f.removed {
		return appstore.LookupOutput{}, errors.New("app not found") // what the real engine returns
	}
	return appstore.LookupOutput{App: appstore.App{ID: 1, BundleID: "com.example.app", Price: f.price}}, nil
}

func (f *fakeStore) Purchase(in appstore.PurchaseInput) error {
	f.purchases++
	if in.App.Price > 0 { // what the real engine does
		return errors.New("purchasing paid apps is not supported")
	}
	f.owned = true
	return nil
}

func (f *fakeStore) Download(in appstore.DownloadInput) (appstore.DownloadOutput, error) {
	f.downloaded = in.App
	if !f.owned {
		return appstore.DownloadOutput{}, appstore.ErrLicenseRequired
	}
	return appstore.DownloadOutput{DestinationPath: "app.ipa"}, nil
}

func (f *fakeStore) ReplicateSinf(appstore.ReplicateSinfInput) error { return nil }

func TestRemovedAppDownload(t *testing.T) {
	// Sent from My apps: the bundle ID lookup fails, the remembered app ID is used.
	store := &fakeStore{owned: true, removed: true}
	b := &backend{store: store}
	req := downloadRequest{target: "com.mobgen.101freealerts", appID: 476204078, platform: "iphone"}
	if _, err := b.download(context.Background(), req, nil); err != nil {
		t.Fatalf("with a known app ID: %v", err)
	}
	if store.downloaded.ID != 476204078 || store.downloaded.BundleID != "com.mobgen.101freealerts" {
		t.Errorf("downloaded %+v, want the app ID with the bundle ID kept for the file name", store.downloaded)
	}

	// Typed by hand, no app ID known: the lookup error comes through.
	store = &fakeStore{owned: true, removed: true}
	b = &backend{store: store}
	req.appID = 0
	_, err := b.download(context.Background(), req, nil)
	if err == nil || !strings.Contains(errorText(err), "enter its App ID") {
		t.Errorf("without an app ID: err = %v, message %q", err, errorText(err))
	}

	// Other lookup errors are not hidden by the fallback.
	if _, _, err := (&backend{store: &errLookupStore{}}).resolveApp(appstore.Account{}, "com.x", 5, appstore.PlatformIPhone); err == nil {
		t.Error("a network error during the lookup should not fall back to the app ID")
	}
}

type errLookupStore struct{ appstore.AppStore }

func (errLookupStore) Lookup(appstore.LookupInput) (appstore.LookupOutput, error) {
	return appstore.LookupOutput{}, errors.New("request failed: dial tcp: i/o timeout")
}

func TestDownloadLicenseRules(t *testing.T) {
	tests := []struct {
		name          string
		price         float64
		getLicense    bool
		wantErr       error
		wantPurchases int
	}{
		// The GUI stops before trying: the engine refuses paid purchases anyway.
		{"paid, license box ticked", 4.99, true, errPaidApp, 0},
		{"paid, license box unticked", 4.99, false, errPaidApp, 0},
		{"free, license box unticked", 0, false, errFreeNotOwned, 0},
		{"free, license box ticked", 0, true, nil, 1},
	}
	for _, tt := range tests {
		store := &fakeStore{price: tt.price}
		b := &backend{store: store}
		res, err := b.download(context.Background(),
			downloadRequest{target: "com.example.app", platform: "iphone", getLicense: tt.getLicense}, nil)
		if !errors.Is(err, tt.wantErr) {
			t.Errorf("%s: err = %v, want %v", tt.name, err, tt.wantErr)
		}
		if store.purchases != tt.wantPurchases {
			t.Errorf("%s: %d purchase attempts, want %d", tt.name, store.purchases, tt.wantPurchases)
		}
		if tt.wantErr == nil && !res.purchased {
			t.Errorf("%s: expected purchased=true", tt.name)
		}
	}
}
