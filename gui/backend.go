package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/byteness/keyring"
	cookiejar "github.com/juju/persistent-cookiejar"
	"github.com/majd/ipatool/v2/pkg/appstore"
	"github.com/majd/ipatool/v2/pkg/keychain"
	"github.com/majd/ipatool/v2/pkg/util/machine"
	"github.com/majd/ipatool/v2/pkg/util/operatingsystem"
	"github.com/schollz/progressbar/v3"
)

// The GUI uses ipatool's engine (pkg/) directly instead of running ipatool.exe.
// cmd/ keeps its setup helpers private, so this file repeats them. It must stay
// identical to cmd/constants.go, cmd/common.go (newCookieJar, newKeychain,
// initWithCommand) and cmd/state_directory.go, otherwise the GUI and the command
// line tool would stop sharing the saved login.
const (
	configDirectoryName = ".ipatool"
	cookieJarFileName   = "cookies"
	keychainServiceName = "ipatool-auth.service"
)

// backend is one session with ipatool's engine, set up the way the command line
// tool sets itself up for each command.
type backend struct {
	store appstore.AppStore
	dir   string // the state directory holding the cookies and the saved login
}

// newBackend opens the saved-login store with the given keychain passphrase. A
// new backend is made for every task, like each CLI run, so a corrected
// passphrase takes effect immediately.
func newBackend(passphrase string) (*backend, error) {
	os := operatingsystem.New()
	mach := machine.New(machine.Args{OS: os})

	dir, err := prepareStateDirectory(os, mach.HomeDirectory())
	if err != nil {
		return nil, err
	}

	jar, err := cookiejar.New(&cookiejar.Options{Filename: filepath.Join(dir, cookieJarFileName)})
	if err != nil {
		return nil, fmt.Errorf("failed to open cookies: %w", err)
	}

	ring, err := keyring.Open(keyring.Config{
		AllowedBackends: []keyring.BackendType{
			keyring.KeychainBackend,
			keyring.SecretServiceBackend,
			keyring.FileBackend,
		},
		ServiceName:              keychainServiceName,
		KeychainTrustApplication: true,
		FileDir:                  dir,
		FilePasswordFunc: func(string) (string, error) {
			if passphrase == "" {
				return "", errors.New("keychain passphrase is required")
			}
			return passphrase, nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open keychain: %w", err)
	}

	store := appstore.NewAppStore(appstore.Args{
		CookieJar:       jar,
		OperatingSystem: os,
		Keychain:        keychain.New(keychain.Args{Keyring: ring, Label: keychainServiceName}),
		Machine:         mach,
	})
	return &backend{store: store, dir: dir}, nil
}

// prepareStateDirectory is a copy of cmd/state_directory.go: it resolves (and if
// needed migrates) the folder where ipatool keeps its session.
func prepareStateDirectory(os operatingsystem.OperatingSystem, homeDirectory string) (string, error) {
	legacyDirectory := filepath.Join(homeDirectory, configDirectoryName)
	stateDirectory := legacyDirectory

	for _, variable := range []string{"XDG_STATE_HOME", "XDG_DATA_HOME"} {
		if base := os.Getenv(variable); filepath.IsAbs(base) {
			stateDirectory = filepath.Join(base, "ipatool")

			break
		}
	}

	info, err := os.Stat(legacyDirectory)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("could not read legacy state directory metadata: %w", err)
	}

	if err == nil {
		if !info.IsDir() {
			return "", fmt.Errorf("legacy state path is not a directory: %s", legacyDirectory)
		}
		// Never merge sessions or move a directory into itself. Keeping the legacy
		// directory also preserves authentication when migration is unavailable.
		relative, err := filepath.Rel(legacyDirectory, stateDirectory)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return legacyDirectory, nil
		}

		if _, err := os.Stat(stateDirectory); err == nil || !os.IsNotExist(err) {
			return legacyDirectory, nil
		}

		if err := os.MkdirAll(filepath.Dir(stateDirectory), 0700); err != nil {
			return legacyDirectory, nil
		}
		// Rename keeps cookies and encrypted credentials together, with their
		// existing permissions. Cross-filesystem moves fall back to legacy storage.
		if err := os.Rename(legacyDirectory, stateDirectory); err != nil {
			return legacyDirectory, nil
		}

		return stateDirectory, nil
	}

	if err := os.MkdirAll(stateDirectory, 0700); err != nil {
		return "", fmt.Errorf("failed to create state directory: %w", err)
	}

	return stateDirectory, nil
}

var (
	errLoginCancelled  = errors.New("login cancelled")
	errCodeNotAccepted = errors.New("the two-factor code was not accepted; try logging in again")
	errPaidApp         = errors.New("this is a paid app that the account hasn't bought")
	errFreeNotOwned    = errors.New("the account doesn't own this free app and getting a license is off")
)

// isPaidPurchaseRefusal reports the engine's refusal to buy a paid app
// (pkg/appstore/appstore_purchase.go), which has no exported error value.
func isPaidPurchaseRefusal(err error) bool {
	return err != nil && strings.Contains(err.Error(), "purchasing paid apps is not supported")
}

// login signs in like `ipatool auth login` in interactive mode: if Apple asks for
// a two-factor code, askCode is called (it shows a prompt and waits) and the same
// session retries with the code.
func (b *backend) login(email, password string, askCode func() (string, bool)) (appstore.Account, error) {
	out, err := b.store.Login(appstore.LoginInput{Email: email, Password: password})
	if errors.Is(err, appstore.ErrAuthCodeRequired) {
		code, ok := askCode()
		if !ok || code == "" {
			return appstore.Account{}, errLoginCancelled
		}
		out, err = b.store.Login(appstore.LoginInput{Email: email, Password: password, AuthCode: code})
		if errors.Is(err, appstore.ErrAuthCodeRequired) {
			return appstore.Account{}, errCodeNotAccepted
		}
	}
	if err != nil {
		return appstore.Account{}, err
	}
	return out.Account, nil
}

// logout removes the saved login (`ipatool auth revoke`).
func (b *backend) logout() error {
	return b.store.Revoke()
}

// withAccount runs fn with the signed-in account. If Apple reports that the
// password token expired, it signs in again with the saved credentials and
// retries once, as cmd/ does for list-purchases and download.
func (b *backend) withAccount(fn func(acc appstore.Account) error) error {
	info, err := b.store.AccountInfo()
	if err != nil {
		return err
	}
	err = fn(info.Account)
	if errors.Is(err, appstore.ErrPasswordTokenExpired) {
		out, loginErr := b.store.Login(appstore.LoginInput{Email: info.Account.Email, Password: info.Account.Password})
		if loginErr != nil {
			return loginErr
		}
		err = fn(out.Account)
	}
	return err
}

// search is `ipatool search`.
func (b *backend) search(term string, limit int64, platform string) ([]App, error) {
	p, err := appstore.ParsePlatform(platform)
	if err != nil {
		return nil, err
	}
	info, err := b.store.AccountInfo()
	if err != nil {
		return nil, err
	}
	out, err := b.store.Search(appstore.SearchInput{Account: info.Account, Term: term, Limit: limit, Platform: p})
	if err != nil {
		return nil, err
	}
	return fromStoreApps(out.Results), nil
}

type downloadRequest struct {
	target     string // bundle ID, or a numeric app ID
	appID      int64  // the target's app ID if known (from a list); used if the bundle ID lookup fails
	platform   string
	versionID  string // external version ID; "" for the latest version
	output     string // folder or file path
	getLicense bool   // obtain a free license if the account doesn't own the app
}

type downloadResult struct {
	path      string
	purchased bool
	renameErr error // the file couldn't be given its iTunes-style name (it keeps ipatool's)
}

// download is `ipatool download`, following cmd/download.go: the account is read
// once, then up to 3 attempts, signing in again when the password token expired
// (the new login is kept for later attempts) and obtaining a license when one
// is required (if allowed), then copying the license data (sinf) into the
// package. progress receives the download progress; ctx cancels it.
func (b *backend) download(ctx context.Context, req downloadRequest, progress *progressbar.ProgressBar) (downloadResult, error) {
	platform, err := appstore.ParsePlatform(req.platform)
	if err != nil {
		return downloadResult{}, err
	}
	info, err := b.store.AccountInfo()
	if err != nil {
		return downloadResult{}, err
	}
	acc := info.Account
	var lastErr error
	var app appstore.App // kept outside the attempt so its price can be checked on failure
	priceKnown := false  // app came from the lookup service, which includes the price
	var renameErr error  // renaming to the iTunes-style name failed
	purchaseRequired, purchased := false, false
	for attempt := 1; ; attempt++ {
		path, err := func() (string, error) {
			if errors.Is(lastErr, appstore.ErrPasswordTokenExpired) {
				login, err := b.store.Login(appstore.LoginInput{Email: acc.Email, Password: acc.Password})
				if err != nil {
					return "", err
				}
				acc = login.Account
			}

			app, priceKnown, err = b.resolveApp(acc, req.target, req.appID, platform)
			if err != nil {
				return "", err
			}

			if errors.Is(lastErr, appstore.ErrLicenseRequired) {
				purchaseRequired = true
			}
			if purchaseRequired {
				err := b.store.Purchase(appstore.PurchaseInput{Account: acc, App: app, Platform: platform})
				if err != nil && !errors.Is(err, appstore.ErrLicenseAlreadyExists) {
					return "", err
				}
				purchaseRequired, purchased = false, true
			}

			out, err := b.store.Download(appstore.DownloadInput{
				Context:           ctx,
				Account:           acc,
				App:               app,
				OutputPath:        req.output,
				Progress:          progress,
				ExternalVersionID: req.versionID,
				Platform:          platform,
			})
			if err != nil {
				return "", err
			}
			// cmd/download.go replicateDownloadSinf: Mac packages without sinfs need none.
			if platform != appstore.PlatformMacOS || len(out.Sinfs) > 0 {
				err := b.store.ReplicateSinf(appstore.ReplicateSinfInput{Sinfs: out.Sinfs, PackagePath: out.DestinationPath})
				if err != nil {
					return "", err
				}
			}
			// Last, name the finished file like iTunes ("Dice Only 1.9.ipa"). If
			// that fails, the download is still fine under ipatool's name.
			path, err := renameLikeITunes(out.DestinationPath, out.Name, out.Version)
			if err != nil {
				renameErr = err
			}
			return path, nil
		}()
		if err == nil {
			return downloadResult{path: path, purchased: purchased, renameErr: renameErr}, nil
		}
		// ipatool can only get licenses for free apps, so a paid app the account
		// hasn't bought can't be downloaded; say so instead of retrying.
		if app.Price > 0 && (errors.Is(err, appstore.ErrLicenseRequired) || isPaidPurchaseRefusal(err)) {
			return downloadResult{}, errPaidApp
		}
		// A free app (price known from the lookup) with the license box unticked.
		if errors.Is(err, appstore.ErrLicenseRequired) && !req.getLicense && priceKnown {
			return downloadResult{}, errFreeNotOwned
		}
		retry := errors.Is(err, appstore.ErrPasswordTokenExpired) ||
			(errors.Is(err, appstore.ErrLicenseRequired) && req.getLicense)
		if !retry || attempt == 3 || ctx.Err() != nil {
			return downloadResult{}, err
		}
		lastErr = err
	}
}

// resolveApp turns a bundle ID or numeric app ID into an app, looking bundle IDs
// up in the account's store as cmd/ does. The lookup service only knows apps
// that are for sale, so for an app removed from the App Store it answers "app
// not found"; if the app ID is known (appID, from a list), that is used instead,
// which is how iTunes and iPhones download purchases that left the store.
// priceKnown reports whether the app came from the lookup (with its price).
func (b *backend) resolveApp(acc appstore.Account, target string, appID int64, platform appstore.Platform) (app appstore.App, priceKnown bool, err error) {
	if id, err := strconv.ParseInt(target, 10, 64); err == nil {
		return appstore.App{ID: id}, false, nil
	}
	lookup, err := b.store.Lookup(appstore.LookupInput{Account: acc, BundleID: target, Platform: platform})
	if err != nil {
		if appID != 0 && errors.Is(err, appstore.ErrAppNotFound) {
			return appstore.App{ID: appID, BundleID: target}, false, nil // the bundle ID keeps the file name readable
		}
		return appstore.App{}, false, err
	}
	return lookup.App, true, nil
}

// listVersions is `ipatool list-versions`: the app's external version IDs, oldest first.
// appID is the target's app ID if known (see resolveApp); ctx cancels it.
func (b *backend) listVersions(ctx context.Context, target string, appID int64, platform string) ([]string, error) {
	p, err := appstore.ParsePlatform(platform)
	if err != nil {
		return nil, err
	}
	var ids []string
	err = b.withAccount(func(acc appstore.Account) error {
		app, _, err := b.resolveApp(acc, target, appID, p)
		if err != nil {
			return err
		}
		out, err := b.store.ListVersions(appstore.ListVersionsInput{Context: ctx, Account: acc, App: app, Platform: p})
		if err != nil {
			return err
		}
		ids = out.ExternalVersionIdentifiers
		return nil
	})
	return ids, err
}

// versionDetails is `ipatool get-version-metadata` for several version IDs in one
// session: the app is looked up once, and report is called after each ID with
// its description (for example "Version 21.38.3, released September 20, 2026").
// A failed lookup is reported as such and the rest continue; ctx stops the loop.
func (b *backend) versionDetails(ctx context.Context, target string, appID int64, platform string, ids []string, report func(id, label string)) error {
	p, err := appstore.ParsePlatform(platform)
	if err != nil {
		return err
	}
	return b.withAccount(func(acc appstore.Account) error {
		app, _, err := b.resolveApp(acc, target, appID, p)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			out, err := b.store.GetVersionMetadata(appstore.GetVersionMetadataInput{
				Context: ctx, Account: acc, App: app, VersionID: id, Platform: p,
			})
			if errors.Is(err, appstore.ErrPasswordTokenExpired) {
				return err // withAccount signs in again and starts over
			}
			label := "Version lookup failed"
			if err == nil {
				label = "Version " + out.DisplayVersion
				if !out.ReleaseDate.IsZero() {
					label += ", released " + out.ReleaseDate.Format("January 2, 2006")
				}
			}
			report(id, label)
		}
		return nil
	})
}

// ownedAppsAll returns the account's whole purchase history, newest first, in one
// request (about 12 seconds), and the account's store country (such as "US").
// Relies on the fork's raised MaxOwnedAppsLimit.
func (b *backend) ownedAppsAll() (apps []App, country string, err error) {
	err = b.withAccount(func(acc appstore.Account) error {
		out, err := b.store.OwnedApps(appstore.OwnedAppsInput{Account: acc, Page: 1, Limit: appstore.MaxOwnedAppsLimit})
		if err != nil {
			return err
		}
		apps = fromStoreApps(out.Results)
		country, _ = appstore.CountryCode(acc.StoreFront) // "" if unknown; the check then reports it
		return nil
	})
	return apps, country, err
}

// appDetails looks up one app in the account's App Store with Apple's public
// lookup service (no sign-in request); nil means the store doesn't have it.
func (b *backend) appDetails(ctx context.Context, id int64) (*storeDetails, error) {
	info, err := b.store.AccountInfo()
	if err != nil {
		return nil, err
	}
	country, err := appstore.CountryCode(info.Account.StoreFront)
	if err != nil {
		return nil, err
	}
	return lookupDetails(ctx, http.DefaultClient, country, id)
}

func fromStoreApps(apps []appstore.App) []App {
	out := make([]App, 0, len(apps))
	for _, a := range apps {
		app := App{ID: a.ID, BundleID: a.BundleID, Name: a.Name, Version: a.Version, Price: a.Price,
			PurchaseDate: a.PurchaseDate, Developer: a.ArtistName}
		app.Size, _ = strconv.ParseInt(a.FileSizeBytes, 10, 64) // blank or odd values stay 0
		for _, p := range a.Platforms {
			app.Platforms = append(app.Platforms, string(p))
		}
		out = append(out, app)
	}
	return out
}

// accountInfo returns the signed-in account (the equivalent of `ipatool auth info`).
func (b *backend) accountInfo() (appstore.Account, error) {
	out, err := b.store.AccountInfo()
	if err != nil {
		return appstore.Account{}, err
	}
	return out.Account, nil
}
