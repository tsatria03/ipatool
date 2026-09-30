## About this fork

This is an accessibility-focused fork of [ipatool](https://github.com/majd/ipatool) by Majd Alfhaily. The command line tool works exactly as upstream; this fork adds an accessible Windows interface that works with screen readers such as NVDA, JAWS and Narrator. See [Accessible GUI](#accessible-gui) for what it does and how it works.

**This fork supports Windows only.** On macOS, Linux or iOS, use upstream [ipatool](https://github.com/majd/ipatool): download it from its [releases](https://github.com/majd/ipatool/releases), or on macOS install it with `brew install ipatool`.

## Requirements

- Windows 10 or 11.
- An Apple Account already configured to use the App Store.
- [Go](https://go.dev/dl/), to build the programs or run them from source.

## Installation

This fork builds two programs with Go:

- `ipatool.exe`, the command line tool.
- `ipatool-gui.exe`, the accessible GUI. It has ipatool built in, so it works on its own; you don't need `ipatool.exe` to use it.

Run these commands from the repository folder to build both into the `releases\ipatool` folder:

```shell
$ go build -o releases\ipatool\ipatool.exe
$ cd gui
$ go build -ldflags=-H=windowsgui -o ..\releases\ipatool\ipatool-gui.exe .
```

Build only the one you want, or both. They share the saved login, so signing in with one also signs in the other. You can also skip building and run the GUI from source; see [Running and building](#running-and-building).

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
$ go build -o releases\ipatool\ipatool.exe
```

Builds for other platforms aren't supported by this fork. ipatool's keychain support on macOS and Linux needs C code compiled for those systems (cgo), so it can't be cross-compiled from Windows; use upstream's releases there.

Unit tests can be executed with the following commands.

```shell
$ go generate ./...
$ go test -v ./...
```

## Accessible GUI

The `gui` folder contains a Windows front end for ipatool, written in Go with [walk](https://github.com/tailscale/walk) (Tailscale's maintained fork). walk builds windows from native Win32 controls, which screen readers can read through MSAA. The GUI is its own Go module (`ipatool-gui`), so ipatool's `go.mod` and code are untouched, and it uses ipatool's engine directly, so `ipatool-gui.exe` is a single standalone program (about 42 MB).

### Features

- Every field, button and list can be read by screen readers such as NVDA, JAWS and Narrator.
- Sign in with your Apple Account, including two-factor codes, check which account is signed in, and sign out. The Account page only shows what applies: while you're signed in, the Apple ID email and password fields and Log in are hidden; once you're signed out, Log out is. Before the first check (for example when your passphrase isn't saved), everything is shown.
- Show passphrase (Alt+P) and Show password (Alt+H) check boxes reveal what you typed in the keychain passphrase and Apple ID password fields, so your screen reader can read it back. They start unchecked every time.
- Global search: search the whole App Store by name and platform, with each result read as its name, version, developer, price and size (in KB, MB or GB, as Apple lists it), for example "Dice Only; Version: 1.12; Developer: Unboxing Solutions B.V.; Price: Free; Size: 40 MB". Send any result straight to the download page, or press App info for the rest.
- App info: press Alt+O (the App info button) or Alt+Enter on an app in either list to read what the App Store lists about it, such as the developer, size, age rating, rating, release dates, what's new and the description, in a window you can read line by line and copy. See [App info](#app-info).
- Download the latest version of an app, or pick an older version from a list that shows each version number and release date.
- Downloads are named like iTunes 12.6.5.3 named them, with the app's name and version, such as `Dice Only 1.9.ipa`, instead of ipatool's `bundle.id_appID_version.ipa`. Characters Windows doesn't allow in file names are replaced (`Battle Prime: Modern War` becomes `Battle Prime - Modern War`), very long names are shortened, and downloading the same version again replaces the file.
- See download progress as a percentage, and cancel a download at any time. Pressing Download moves you to the Result field, so your screen reader reads the progress and the outcome.
- Browse the apps your account owns, 100 at a time, each read as its name, version, developer, purchase date and size. They load by themselves the first time you move into the My apps page, and then searching them by name or bundle ID (with Enter or the Search button), filtering by platform, sorting and turning pages are instant.
- Sort your apps by purchase date, name, bundle ID, developer (A to Z or Z to A) or size (largest or smallest first), or by how long they've been on the App Store.
- See which of your apps are still on the App Store and which have been removed, with the availability filter.
- Copy all your apps to the clipboard as text, or export them to a JSON file, following the current filters, search and sort. See [The My apps page](#the-my-apps-page).
- Works with iPhone, iPad, Apple TV, Apple Vision Pro and Mac apps.
- First-letter navigation in every list: type a letter to jump to the next app whose name starts with it, press it again for the next one, or type several letters quickly to match more of the name. In My apps it searches the page on screen.
- Full keyboard control: Ctrl+1 to Ctrl+5 to switch pages, Alt plus the underlined letter for any field or button, Enter to search or download, Escape to cancel, F5 to check your account, and F1 for a list of shortcuts in a read-only text box you can read line by line (one shortcut per line) and copy.
- Switching pages keeps you on the page tabs, and moving between tabs announces just the tab's name.
- Clear, plain-language messages when something goes wrong, such as a wrong passphrase or an expired sign-in.
- A log page that shows everything the interface did, with passwords and codes hidden.
- Your settings are remembered between sessions. You can also choose to remember your keychain passphrase; it is then stored unencrypted on your computer.

### The My apps page

My apps lists every app your Apple Account owns. The first time you move into the page, it fetches your whole purchase history from Apple and checks which of those apps are still on your account's App Store (about 15 seconds together for a few thousand apps). It keeps the result for the session, so everything below is instant.

| Control | Shortcut | What it does |
|---|---|---|
| Load | Alt+L | Fetches a fresh list from Apple and checks availability again. |
| Previous page / Next page | Alt+P / Alt+N | Turns the page. The list shows 100 apps at a time. |
| Availability filter | Alt+I | All apps, Available (still on the App Store), or Unavailable (no longer on the App Store). |
| Platform filter | Alt+T | Shows only apps for one platform (iphone, ipad, appletv, visionos or macos), or all platforms. |
| Page | Alt+G | Read-only. For example "Page 2 of 36, 3554 apps", or "Page 1 of 1, 13 of 3554 apps match" while searching or filtering. |
| Search my apps | Alt+S | Type part of an app's name or bundle ID, then press Enter. |
| Search | Alt+R | Does the same as pressing Enter in Search my apps. |
| Sort by | Alt+B | Newest or oldest purchase first; name A to Z or Z to A; bundle ID A to Z or Z to A; newest or oldest to the App Store first; largest or smallest first; developer A to Z or Z to A. Apps that left the App Store have no size or developer, so those four orders put them last. Apps by the same developer are sorted by name. |
| Apps you own | Alt+W | The list: name, version, developer, purchase date and size, for example "Game-board; Version: 1.0.5; Developer: Muamel Aljanahi; Purchase date: September 5, 2026; Size: 217 MB". Apps that left the App Store have no developer or size. App info has the rest, such as the bundle ID, platforms and App ID. Press Enter on an app to send it to the Download page. |
| Download selected | Alt+D | Sends the selected app to the Download page. |
| Copy bundle ID | Alt+C | Copies the selected app's bundle ID. |
| App info | Alt+O | Shows what the App Store lists about the selected app. Alt+Enter in the list does the same. See [App info](#app-info). |
| Copy all apps | Alt+A | Copies every matching app as text. |
| Export to JSON | Alt+E | Saves every matching app, with details about the export, as a JSON file. |

The availability filter, the platform filter and Sort by apply as you arrow through them, and Enter in any of them, or in Search my apps, moves to the list. They all work together: for example, Unavailable with Name, A to Z and the search "radio" lists the radio apps you own that have left the App Store, alphabetically.

**Availability** comes from Apple's public lookup service, which only returns apps currently for sale in your account's country. An app that's unavailable may have been removed by its developer or by Apple, or only be sold in other countries. Your purchase history keeps it either way, and you can often still download it.

**Downloading apps that left the App Store:** select the app in My apps and press Download selected (or Enter), then Download. Like iTunes and your iPhone, the GUI then downloads it by its App ID from your purchase history. If you type a bundle ID on the Download page yourself and Apple can't find it, type the app's App ID instead; App info (Alt+O) shows it for every app in My apps. Apple may no longer have the files for some removed apps, so not every one can be downloaded. If the check can't reach Apple, your apps still load; All apps works, and the Page field explains that availability couldn't be checked until you press Load again.

**Copy all apps and Export to JSON** include every page, not just the 100 apps on screen. They follow the availability and platform filters, Search my apps and Sort by, so with nothing narrowed you get all your apps in the chosen order. If your apps haven't loaded yet, or are still loading, they wait for the load and then continue. The copied text never includes availability.

Copy all apps puts one line per app on the clipboard:

```text
Game-board; Bundle ID: net.muamal.gameboard; Version: 1.0.5; Platforms: iphone; App ID: 6786885206
```

Export to JSON suggests `My apps.json` in your download folder and asks before replacing a file. The file describes the export, then lists the apps with the same field names as `ipatool list-purchases --format json`, plus the purchase date and whether the app is still on the App Store (`available` is left out if availability couldn't be checked):

```json
{
  "exportedFrom": "ipatool GUI, My apps",
  "exported": "2026-09-27T14:05:00-07:00",
  "totalApps": 3554,
  "exportedApps": 14,
  "availability": "Unavailable",
  "search": "radio",
  "platform": "all platforms",
  "sortedBy": "Name, A to Z",
  "apps": [
    {
      "name": "100.9 Cherry FM",
      "bundleID": "com.kary.radioplayer",
      "version": "6.0.1",
      "platforms": ["iphone", "ipad"],
      "id": 891764481,
      "purchaseDate": "2017-11-22T19:14:16Z",
      "available": false
    }
  ]
}
```

The file is UTF-8, so app names in any language are kept as they are.

### App info

Select an app in Global search or My apps and press App info (Alt+O), or press Alt+Enter in the list. A window opens with a read-only Details box (Alt+D), where your screen reader reads one fact per line:

```text
Name: Dice Only
Developer: Unboxing Solutions B.V.
Version: 1.12
Size: 40 MB
Price: Free
Category: Games
Age rating: 4+
Minimum OS version: 17.6
Languages: NL, EN, HI
Released: January 15, 2023
Updated: September 19, 2026
Platforms: iphone, ipad
Bundle ID: com.unboxingsolutions.DiceOnly
App ID: 1665283082
App Store link: https://apps.apple.com/us/app/dice-only/id1665283082?uo=4

What's new:
...

Description:
...
```

Lines Apple has nothing for are left out (this app has no ratings yet; others show a line like `Rating: 4.8 out of 5, 13 ratings`), and My apps adds the date you got the app. Copy all (Alt+C) copies the whole text, and Close or Escape returns you to the list.

- **My apps** already has these details for every app still on the App Store, from the availability check, so App info opens instantly and asks Apple nothing.
- **Apps that left the App Store** have no details at Apple; the window says so and shows what your purchase history knows (name, version, platforms, purchase date, bundle ID and App ID).
- **Global search** looks the app up once with Apple's public lookup service (no sign-in), and remembers it until you close the GUI.

### Running and building

Go 1.27 or newer is required (see `gui/go.mod`).

- **From source, no console window:** double-click `gui\run-gui.vbs`. It runs `go run -ldflags=-H=windowsgui .` in the `gui` folder.
- **From a terminal:** `cd gui` and then `go run .`
- **As an exe:** `cd gui` and then `go build -ldflags=-H=windowsgui -o ..\releases\ipatool\ipatool-gui.exe .`

The Common Controls v6 manifest that walk needs is embedded through `gui/rsrc.syso`, which Go links automatically. If `gui/app.manifest` changes, regenerate it:

```shell
$ go run github.com/akavel/rsrc@latest -manifest app.manifest -o rsrc.syso
```

### How it works

The GUI uses ipatool's engine (`pkg/appstore` and the packages around it) directly. `gui/go.mod` requires `github.com/majd/ipatool/v2` and replaces it with `../`, so the GUI compiles the repository's own `pkg` folder in place; nothing is copied, and changes to `pkg` are picked up on the next GUI build.

- **Shared login:** `cmd` keeps its setup helpers private, so `gui/backend.go` repeats them: the state directory (`~/.ipatool`, or `$XDG_STATE_HOME/ipatool` / `$XDG_DATA_HOME/ipatool`, including the legacy migration), the `cookies` file and the `ipatool-auth.service` keyring with the file backend. This must stay identical to `cmd/common.go`, `cmd/constants.go` and `cmd/state_directory.go`, or the GUI and the command line tool would stop sharing the saved login. `gui/backend_test.go` fails if the copied code or constants drift. Shared dependencies (such as `byteness/keyring` and `jose2go`) are pinned to the versions in ipatool's `go.mod`; after changing dependencies, compare `go list -m all` in both modules.
- **Same behavior as the CLI:** each action follows its `cmd` counterpart, including signing in again and retrying when Apple reports an expired password token, getting a free license when allowed, and copying the license data (sinf) into downloaded packages.
- **Apps removed from the App Store:** a bundle ID is turned into an app with the public lookup service (as `cmd` does), which answers "app not found" for apps no longer for sale. Download selected remembers the app ID of the app it sends, and `backend.resolveApp` falls back to it when the lookup says "app not found", so downloads and Choose older version work for owned apps that left the store. The app ID goes straight to the purchase-history download endpoint, like iTunes and iOS re-downloads.
- **Two-factor codes:** login asks for the code in the middle of the same session, like `ipatool auth login` in interactive mode.
- **Tasks:** each action runs in the background with a fresh engine session (so a corrected passphrase takes effect immediately), and updates the window on the UI thread.
- **My apps:** every owned-apps request makes the engine download the whole purchase history (about 12 seconds) and then cut out one page. The GUI therefore fetches the whole list once per session (`backend.ownedAppsAll`), keeps it in memory, and searches, filters and pages through it locally (`myapps.go`). The first load starts when focus first enters the page's controls; Load fetches a fresh list.
- **Changes to ipatool's own code:** keep these in mind when merging upstream changes to those files.
  - To fetch everything in one request, this fork raises `MaxOwnedAppsLimit` in `pkg/appstore/appstore_owned_apps.go` from 100 to 100000 (the related tests in `pkg/appstore` and `cmd` use the constant). The command line tool's `list-purchases --max-results` accepts the larger value too.
  - `appstore.App` in `pkg/appstore/app.go` has `FileSizeBytes` and `ArtistName` fields, which keep the size and developer Apple lists in search and lookup results, for Global search's Size and Developer columns. The command line tool's output is unchanged, because it lists its fields explicitly.
  - `appstore.DownloadOutput` in `pkg/appstore/appstore_download.go` has `Name` and `Version`, the app's store name (`itemName`) and version from Apple's download answer (falling back to `bundleVersion` for older apps without `bundleShortVersionString`). The GUI renames finished downloads to `Name Version.ipa` with them (`gui/filenames.go`), after the license data is copied in, so resuming and licensing work as before. The command line tool's file names are unchanged.
  - `pkg/appstore/storefront.go` exports `CountryCode`, a wrapper around the private store front to country mapping, so My apps can check availability in the account's country.
- **Availability check:** after the purchase history loads, `gui/availability.go` sends the owned app IDs to Apple's public lookup service (`itunes.apple.com/lookup`, 150 IDs per request, 4 requests at a time, no sign-in) for the account's country. Apps it doesn't return are marked unavailable. If any request fails, every app stays unknown, so a failed check never makes apps look unavailable. The lookup answers include each app's full store record, which the check keeps (`storeDetails` in `gui/details.go`) for App info; a field of an unexpected type is left empty instead of failing the check.
- **Progress and cancelling:** downloads report progress through a `progressbar` that isn't drawn, read once a second for the status bar and the Result field. Escape cancels the task's context, which stops a download; a cancelled task's result is discarded.

After merging upstream changes, build the GUI and run `go test .` in the `gui` folder, since changes in `pkg/appstore` or `cmd` can affect it.

### Settings

Settings are stored as JSON in `%APPDATA%\ipatool-gui\settings.json`:

| Key | Contents |
|---|---|
| `email` | Apple Account email |
| `output` | download folder |
| `passphrase` | keychain passphrase, in plain text, only if "Remember passphrase" is checked |

### Source layout

| File | Purpose |
|---|---|
| `main.go` | Main window, menus and shortcuts |
| `backend.go` | ipatool engine setup (matching `cmd`) and every App Store action |
| `backend_test.go` | Checks that the setup copied from `cmd` hasn't drifted; a live account check runs with `IPATOOL_GUI_LIVE=1` |
| `tasks.go` | Running actions in the background, the busy timer and progress |
| `account.go` | Account page and the two-factor code prompt |
| `search.go` | Global search page and the app list model shared with My apps |
| `download.go` | Download page and the Choose older version dialog |
| `filenames.go` | iTunes-style file names for downloads, made safe for Windows |
| `filenames_test.go` | Tests for the file names, shortening and replacing |
| `purchases.go` | My apps page |
| `myapps.go` | Local search, filters, sorting, paging, and the copy and JSON export formats for My apps |
| `availability.go` | Checks which owned apps are still on the account's App Store |
| `availability_test.go` | Tests the availability check and the store details against a fake lookup service, including failures |
| `details.go` | App info: the store details, their text and the App info window |
| `details_test.go` | Tests for the App info text |
| `myapps_test.go` | Tests for the search, filter, sorting, paging, sizes and export formats |
| `messages.go` | Plain-language error messages, such as for paid apps the account hasn't bought |
| `errors_test.go` | Checks the error messages and the download license rules against a fake App Store |
| `settings.go` | Saved settings and the app type |
| `tabs.go` | Page switching and Ctrl+Tab |
| `winfix.go` | Accessibility workarounds for walk |
| `typeahead.go` | First-letter navigation for the app lists |
| `typeahead_test.go` | Tests for the first-letter matching |
| `run-gui.vbs` | Double-click launcher |

### Accessibility workarounds for walk

walk needed several fixes to work well with screen readers:

- **Control IDs:** walk creates every control with ID 0. Containers look up the sender of a `WM_COMMAND` with `GetDlgItem`, which returns the first child with that ID, so most button clicks went to the wrong control. `assignControlIDs` gives every control in walk containers a unique ID.
- **Labels:** walk nests each label's STATIC control inside a wrapper window, so screen readers can't associate labels with fields. Every field gets an explicit accessible name and keyboard shortcut through `declarative.Accessibility`. Alt-key mnemonics on labels still work through the dialog manager.
- **First-letter navigation:** a TableView's list views are virtual (`LVS_OWNERDATA`): they ask for each row's text instead of storing it, so they can't search it. When a letter is typed, the list view asks its parent which row matches with `LVN_ODFINDITEM`, which walk doesn't answer, so letters did nothing. `enableTypeAhead` (`typeahead.go`) subclasses the TableView wrapper and answers with the next row whose first column starts with the typed text. The list view still handles the typing itself (quick letters form a prefix, a repeated letter moves on), so it behaves like any Windows list. Every TableView should get it; plain list boxes and combo boxes already do this natively.
- **TableView:** a TableView is two SysListView32 controls, one for frozen columns and the real one. Both were Tab stops, and the frozen one forwards focus, which trapped Shift+Tab. `fixTableView` removes the frozen list from the Tab order and sets the real list's name and shortcut with MSAA dynamic annotation (`IAccPropServices`).
- **Page changes:** after a page change, `TabWidget` moves focus into the new page if focus was anywhere inside it, including the tab strip. Moving focus back made screen readers announce "tab control" on every arrow key. `withoutPageAutofocus` clears `WS_TABSTOP` on the pages' controls during a page change, so walk finds nothing to focus. It covers arrow keys and clicks (by subclassing the TabWidget wrapper to intercept `TCN_SELCHANGE`), Ctrl+1 to Ctrl+5, and Ctrl+Tab (handled in a pre-translate handler that passes every other message to the main window's dialog-manager handling).
- **Keyboard navigation in the main window:** Tab and Alt mnemonics require the main window to be registered as a pre-translate handler, which walk only does automatically for dialogs.
- **Toolbar:** walk always creates a toolbar. The empty one is hidden so it isn't a Tab stop.
- **Controls avoided:** `NumberEdit` is avoided because its accessible name lands on a wrapper rather than the focused edit; a drop-down list is used instead.

## License

ipatool is released under the [MIT license](https://github.com/majd/ipatool/blob/main/LICENSE).
