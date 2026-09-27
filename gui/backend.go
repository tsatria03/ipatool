package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/byteness/keyring"
	cookiejar "github.com/juju/persistent-cookiejar"
	"github.com/majd/ipatool/v2/pkg/appstore"
	"github.com/majd/ipatool/v2/pkg/keychain"
	"github.com/majd/ipatool/v2/pkg/util/machine"
	"github.com/majd/ipatool/v2/pkg/util/operatingsystem"
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
)

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

// accountInfo returns the signed-in account (the equivalent of `ipatool auth info`).
func (b *backend) accountInfo() (appstore.Account, error) {
	out, err := b.store.AccountInfo()
	if err != nil {
		return appstore.Account{}, err
	}
	return out.Account, nil
}
