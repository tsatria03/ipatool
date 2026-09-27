package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// These tests guard against the GUI's copy of ipatool's setup drifting from the
// command line tool's, which would split the saved login between the two.

// funcSource returns the source of the named top-level function in a Go file.
func funcSource(t *testing.T, path, name string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	src := strings.ReplaceAll(string(data), "\r\n", "\n")
	start := strings.Index(src, "\nfunc "+name+"(")
	if start < 0 {
		t.Fatalf("func %s not found in %s", name, path)
	}
	end := strings.Index(src[start:], "\n}\n")
	if end < 0 {
		t.Fatalf("end of func %s not found in %s", name, path)
	}
	return src[start : start+end+3]
}

func TestStateDirectoryMatchesCLI(t *testing.T) {
	cli := funcSource(t, "../cmd/state_directory.go", "prepareStateDirectory")
	gui := funcSource(t, "backend.go", "prepareStateDirectory")
	// The only intended difference is the (private) constant's name.
	cli = strings.ReplaceAll(cli, "ConfigDirectoryName", "configDirectoryName")
	if cli != gui {
		t.Errorf("gui/backend.go prepareStateDirectory differs from cmd/state_directory.go; copy it again.\nCLI:\n%s\nGUI:\n%s", cli, gui)
	}
}

func TestConstantsMatchCLI(t *testing.T) {
	data, err := os.ReadFile("../cmd/constants.go")
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"ConfigDirectoryName": configDirectoryName,
		"CookieJarFileName":   cookieJarFileName,
		"KeychainServiceName": keychainServiceName,
	} {
		m := regexp.MustCompile(name + `\s*=\s*"([^"]*)"`).FindStringSubmatch(string(data))
		if m == nil {
			t.Errorf("%s not found in cmd/constants.go", name)
		} else if m[1] != want {
			t.Errorf("%s: CLI uses %q, GUI uses %q", name, m[1], want)
		}
	}
}

// TestLiveAccountInfo reads the real saved login. It only runs when
// IPATOOL_GUI_LIVE=1, using the passphrase remembered in the GUI's settings.
func TestLiveAccountInfo(t *testing.T) {
	if os.Getenv("IPATOOL_GUI_LIVE") != "1" {
		t.Skip("set IPATOOL_GUI_LIVE=1 to test against the real saved login")
	}
	passphrase := loadSettings().Passphrase
	if passphrase == "" {
		t.Skip("no remembered passphrase in the GUI settings")
	}
	b, err := newBackend(passphrase)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("state directory: %s", b.dir)
	acc, err := b.accountInfo()
	if err != nil {
		t.Fatalf("accountInfo: %v", err)
	}
	if acc.Email == "" {
		t.Fatal("accountInfo returned no email")
	}
	t.Logf("signed in as %s (%s)", acc.Name, acc.Email)
}
