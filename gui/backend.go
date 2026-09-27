package main

import (
	"context"
	"errors"
	"fmt"
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

// ownedPage is one page of `ipatool list-purchases`.
type ownedPage struct {
	apps  []App
	total int
}

// ownedApps is `ipatool list-purchases`; platform "" means all platforms.
func (b *backend) ownedApps(page, limit int, platform string) (ownedPage, error) {
	p, err := appstore.ParsePlatform(platform)
	if err != nil {
		return ownedPage{}, err
	}
	var result ownedPage
	err = b.withAccount(func(acc appstore.Account) error {
		out, err := b.store.OwnedApps(appstore.OwnedAppsInput{Account: acc, Page: page, Limit: limit, Platform: p})
		if err != nil {
			return err
		}
		result = ownedPage{apps: fromStoreApps(out.Results), total: out.TotalCount}
		return nil
	})
	return result, err
}

type downloadRequest struct {
	target     string // bundle ID, or a numeric app ID
	platform   string
	versionID  string // external version ID; "" for the latest version
	output     string // folder or file path
	getLicense bool   // obtain a free license if the account doesn't own the app
}

type downloadResult struct {
	path      string
	purchased bool
}

// download is `ipatool download`, following cmd/download.go: up to 3 attempts,
// signing in again when the password token expired and obtaining a license when
// one is required (if allowed), then copying the license data (sinf) into the
// package. progress receives the download progress; ctx cancels it.
func (b *backend) download(ctx context.Context, req downloadRequest, progress *progressbar.ProgressBar) (downloadResult, error) {
	platform, err := appstore.ParsePlatform(req.platform)
	if err != nil {
		return downloadResult{}, err
	}
	var appID int64
	bundleID := req.target
	if id, err := strconv.ParseInt(req.target, 10, 64); err == nil {
		appID, bundleID = id, ""
	}

	var lastErr error
	var app appstore.App // kept outside the attempt so its price can be checked on failure
	purchaseRequired, purchased := false, false
	for attempt := 1; ; attempt++ {
		path, err := func() (string, error) {
			info, err := b.store.AccountInfo()
			if err != nil {
				return "", err
			}
			acc := info.Account
			if errors.Is(lastErr, appstore.ErrPasswordTokenExpired) {
				login, err := b.store.Login(appstore.LoginInput{Email: acc.Email, Password: acc.Password})
				if err != nil {
					return "", err
				}
				acc = login.Account
			}

			app = appstore.App{ID: appID}
			if bundleID != "" {
				lookup, err := b.store.Lookup(appstore.LookupInput{Account: acc, BundleID: bundleID, Platform: platform})
				if err != nil {
					return "", err
				}
				app = lookup.App
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
			return out.DestinationPath, nil
		}()
		if err == nil {
			return downloadResult{path: path, purchased: purchased}, nil
		}
		// ipatool can only get licenses for free apps, so a paid app the account
		// hasn't bought can't be downloaded; say so instead of retrying.
		if app.Price > 0 && (errors.Is(err, appstore.ErrLicenseRequired) || isPaidPurchaseRefusal(err)) {
			return downloadResult{}, errPaidApp
		}
		// A free app (price known from the lookup) with the license box unticked.
		if errors.Is(err, appstore.ErrLicenseRequired) && !req.getLicense && bundleID != "" {
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
// up in the account's store as cmd/ does.
func (b *backend) resolveApp(acc appstore.Account, target string, platform appstore.Platform) (appstore.App, error) {
	if id, err := strconv.ParseInt(target, 10, 64); err == nil {
		return appstore.App{ID: id}, nil
	}
	lookup, err := b.store.Lookup(appstore.LookupInput{Account: acc, BundleID: target, Platform: platform})
	if err != nil {
		return appstore.App{}, err
	}
	return lookup.App, nil
}

// listVersions is `ipatool list-versions`: the app's external version IDs, oldest first.
func (b *backend) listVersions(target, platform string) ([]string, error) {
	p, err := appstore.ParsePlatform(platform)
	if err != nil {
		return nil, err
	}
	var ids []string
	err = b.withAccount(func(acc appstore.Account) error {
		app, err := b.resolveApp(acc, target, p)
		if err != nil {
			return err
		}
		out, err := b.store.ListVersions(appstore.ListVersionsInput{Account: acc, App: app, Platform: p})
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
func (b *backend) versionDetails(ctx context.Context, target, platform string, ids []string, report func(id, label string)) error {
	p, err := appstore.ParsePlatform(platform)
	if err != nil {
		return err
	}
	return b.withAccount(func(acc appstore.Account) error {
		app, err := b.resolveApp(acc, target, p)
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
// request (about 12 seconds). Relies on the fork's raised MaxOwnedAppsLimit.
func (b *backend) ownedAppsAll() ([]App, error) {
	var apps []App
	err := b.withAccount(func(acc appstore.Account) error {
		out, err := b.store.OwnedApps(appstore.OwnedAppsInput{Account: acc, Page: 1, Limit: appstore.MaxOwnedAppsLimit})
		if err != nil {
			return err
		}
		apps = fromStoreApps(out.Results)
		return nil
	})
	return apps, err
}

func fromStoreApps(apps []appstore.App) []App {
	out := make([]App, 0, len(apps))
	for _, a := range apps {
		app := App{ID: a.ID, BundleID: a.BundleID, Name: a.Name, Version: a.Version, Price: a.Price,
			PurchaseDate: a.PurchaseDate}
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
