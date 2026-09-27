<p align="center">
  <a href="https://GitHub.com/majd/ipatool/releases/"><img src="https://img.shields.io/github/release/majd/ipatool.svg?label=Release" alt="Release"></a>
  <a href="https://github.com/majd/ipatool/blob/main/LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License"></a>
  <a href="https://github.com/sponsors/majd"><img src="https://img.shields.io/badge/Sponsor-%E2%9D%A4-pink.svg" alt="Sponsor"></a>
</p>

<p align="center">
  <code>ipatool</code> is a command line tool that allows you to search for iOS, iPadOS, tvOS, visionOS, and macOS apps on the <a href="https://apps.apple.com">App Store</a>, and download <code>.ipa</code> or macOS <code>.pkg</code> app packages.
</p>

<p align="center">
  <img src="./resources/demo.gif" alt="Demo">
</p>

## About this fork

This is an accessibility-focused fork of [ipatool](https://github.com/majd/ipatool) by Majd Alfhaily. The command line tool works exactly as upstream; this fork adds an accessible Windows interface that works with screen readers such as NVDA, JAWS and Narrator. See [Accessible GUI](#accessible-gui) for what it does and how it works.

**This fork supports Windows only.** On macOS, Linux or iOS, use upstream [ipatool](https://github.com/majd/ipatool): download it from its [releases](https://github.com/majd/ipatool/releases), or on macOS install it with `brew install ipatool`.

## Requirements

- Windows 10 or 11.
- An Apple Account already configured to use the App Store.
- [Go](https://go.dev/dl/), to build ipatool or run the GUI from source.

## Installation

This fork builds two programs with Go:

- `ipatool.exe`, the command line tool.
- `ipatool-gui.exe`, the accessible GUI, which runs `ipatool.exe` behind the scenes.

Run these commands from the repository folder to build both into the `releases` folder:

```shell
$ go build -o releases\ipatool.exe
$ cd gui
$ go build -ldflags=-H=windowsgui -o ..\releases\ipatool-gui.exe .
```

Keep both exes in the same folder so the GUI finds `ipatool.exe` automatically. You can also skip building and run either one from source; see [Running and building](#running-and-building).

## Usage

To get started, run the following command.

```text
$ ipatool --help
A cli tool for interacting with Apple's ipa files

Usage:
  ipatool [command]

Available Commands:
  auth                 Authenticate with the App Store
  completion           Generate the autocompletion script for the specified shell
  download             Download iOS, iPadOS, tvOS, visionOS, and macOS app packages from the App Store
  get-version-metadata Retrieves the metadata for a specific version of an app
  help                 Help about any command
  list-purchases       List apps owned by the authenticated App Store account
  list-versions        List the available versions of an App Store app
  purchase             Obtain a license for the app from the App Store
  search               Search for iOS, iPadOS, tvOS, visionOS, and macOS apps available on the App Store

Flags:
      --format format                sets output format for command; can be 'text', 'json' (default text)
  -h, --help                         help for ipatool
      --keychain-passphrase string   passphrase for unlocking keychain
      --non-interactive              run in non-interactive session
      --verbose                      enables verbose logs
  -v, --version                      version for ipatool

Use "ipatool [command] --help" for more information about a command.
```

**Note:** the tool runs in interactive mode by default. Use the `--non-interactive` flag
if running in an automated environment.

## Compiling

The tool can be compiled for Windows using the Go toolchain.

```shell
$ go build -o releases\ipatool.exe
```

Builds for other platforms aren't supported by this fork. ipatool's keychain support on macOS and Linux needs C code compiled for those systems (cgo), so it can't be cross-compiled from Windows; use upstream's releases there.

Unit tests can be executed with the following commands.

```shell
$ go generate ./...
$ go test -v ./...
```

## Accessible GUI

The `gui` folder contains a Windows front end for ipatool, written in Go with [walk](https://github.com/tailscale/walk) (Tailscale's maintained fork). walk builds windows from native Win32 controls, which screen readers can read through MSAA. The GUI is its own Go module (`ipatool-gui`), so ipatool's `go.mod` and code are untouched.

### Features

- Every field, button and list can be read by screen readers such as NVDA, JAWS and Narrator.
- Sign in with your Apple Account, including two-factor codes, check which account is signed in, and sign out.
- Search the App Store by name and platform, then send any result straight to the download page.
- Download the latest version of an app, or pick an older version from a list that shows each version number.
- Browse the apps your account owns, 25 at a time, filtered by platform.
- Works with iPhone, iPad, Apple TV, Apple Vision Pro and Mac apps.
- Full keyboard control: Ctrl+1 to Ctrl+5 to switch pages, Alt plus the underlined letter for any field or button, Enter to search or download, Escape to cancel, F5 to check your account, and F1 for a list of shortcuts.
- Switching pages keeps you on the page tabs, and moving between tabs announces just the tab's name.
- Clear, plain-language messages when something goes wrong, such as a wrong passphrase or an expired sign-in.
- A log page that shows everything the interface did, with passwords and codes hidden.
- Your settings are remembered between sessions. You can also choose to remember your keychain passphrase; it is then stored unencrypted on your computer.

### Running and building

Go 1.27 or newer is required (see `gui/go.mod`).

- **From source, no console window:** double-click `gui\run-gui.vbs`. It runs `go run -ldflags=-H=windowsgui .` in the `gui` folder.
- **From a terminal:** `cd gui` and then `go run .`
- **As an exe:** `cd gui` and then `go build -ldflags=-H=windowsgui -o ..\releases\ipatool-gui.exe .`

The Common Controls v6 manifest that walk needs is embedded through `gui/rsrc.syso`, which Go links automatically. If `gui/app.manifest` changes, regenerate it:

```shell
$ go run github.com/akavel/rsrc@latest -manifest app.manifest -o rsrc.syso
```

### How it runs ipatool

The GUI doesn't link ipatool's packages. Every action runs the ipatool CLI with `--format json --non-interactive --keychain-passphrase <passphrase>`, with no console window, and parses the JSON lines it prints. The last event that has a `success` field carries the result.

The ipatool to run is chosen in this order:

1. The path saved in the settings, if the file still exists.
2. The newest `ipatool*.exe` (by version number in the name) next to the GUI, in the working folder, or in `../releases`. Files with `gui` in the name are skipped.
3. The ipatool source folder: the nearest folder at or above the working folder whose `go.mod` declares `module github.com/majd/ipatool`. Each command is then run with `go run .` in that folder.

An exe takes priority over a saved source folder as soon as one appears.

Other behavior:

- **Two-factor codes:** in non-interactive mode, `auth login` exits successfully with the message "2FA code is required". The GUI detects that message, asks for the code, and runs the login again with `--auth-code`.
- **Cancelling:** Escape stops the whole process tree (`taskkill /T`), because with `go run` ipatool is a child of `go.exe`. A cancelled task's result is discarded.

### Settings

Settings are stored as JSON in `%APPDATA%\ipatool-gui\settings.json`:

| Key | Contents |
|---|---|
| `exe` | ipatool exe or source folder |
| `email` | Apple Account email |
| `output` | download folder |
| `passphrase` | keychain passphrase, in plain text, only if "Remember passphrase" is checked |

### Source layout

| File | Purpose |
|---|---|
| `main.go` | Main window, menus, shortcuts, and running tasks in the background |
| `account.go` | Account page and the two-factor code prompt |
| `search.go` | Search page and the app list model shared with My apps |
| `download.go` | Download page and the Choose older version dialog |
| `purchases.go` | My apps page |
| `runner.go` | Settings, finding ipatool, running it, parsing output, plain-language errors |
| `tabs.go` | Page switching and Ctrl+Tab |
| `winfix.go` | Accessibility workarounds for walk |
| `run-gui.vbs` | Double-click launcher |

### Accessibility workarounds for walk

walk needed several fixes to work well with screen readers:

- **Control IDs:** walk creates every control with ID 0. Containers look up the sender of a `WM_COMMAND` with `GetDlgItem`, which returns the first child with that ID, so most button clicks went to the wrong control. `assignControlIDs` gives every control in walk containers a unique ID.
- **Labels:** walk nests each label's STATIC control inside a wrapper window, so screen readers can't associate labels with fields. Every field gets an explicit accessible name and keyboard shortcut through `declarative.Accessibility`. Alt-key mnemonics on labels still work through the dialog manager.
- **TableView:** a TableView is two SysListView32 controls, one for frozen columns and the real one. Both were Tab stops, and the frozen one forwards focus, which trapped Shift+Tab. `fixTableView` removes the frozen list from the Tab order and sets the real list's name and shortcut with MSAA dynamic annotation (`IAccPropServices`).
- **Page changes:** after a page change, `TabWidget` moves focus into the new page if focus was anywhere inside it, including the tab strip. Moving focus back made screen readers announce "tab control" on every arrow key. `withoutPageAutofocus` clears `WS_TABSTOP` on the pages' controls during a page change, so walk finds nothing to focus. It covers arrow keys and clicks (by subclassing the TabWidget wrapper to intercept `TCN_SELCHANGE`), Ctrl+1 to Ctrl+5, and Ctrl+Tab (handled in a pre-translate handler that passes every other message to the main window's dialog-manager handling).
- **Keyboard navigation in the main window:** Tab and Alt mnemonics require the main window to be registered as a pre-translate handler, which walk only does automatically for dialogs.
- **Toolbar:** walk always creates a toolbar. The empty one is hidden so it isn't a Tab stop.
- **Controls avoided:** `NumberEdit` is avoided because its accessible name lands on a wrapper rather than the focused edit; a drop-down list is used instead.

## License

ipatool is released under the [MIT license](https://github.com/majd/ipatool/blob/main/LICENSE).
