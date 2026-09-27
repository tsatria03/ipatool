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
	price     float64
	owned     bool
	purchases int
}

func (f *fakeStore) AccountInfo() (appstore.AccountInfoOutput, error) {
	return appstore.AccountInfoOutput{Account: appstore.Account{Email: "a@example.com"}}, nil
}

func (f *fakeStore) Lookup(appstore.LookupInput) (appstore.LookupOutput, error) {
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

func (f *fakeStore) Download(appstore.DownloadInput) (appstore.DownloadOutput, error) {
	if !f.owned {
		return appstore.DownloadOutput{}, appstore.ErrLicenseRequired
	}
	return appstore.DownloadOutput{DestinationPath: "app.ipa"}, nil
}

func (f *fakeStore) ReplicateSinf(appstore.ReplicateSinfInput) error { return nil }

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
