// Workarounds for walk behaviour that breaks keyboard and screen-reader use.
package main

import (
	"strings"
	"syscall"
	"unsafe"

	"github.com/tailscale/walk"
	"github.com/tailscale/win"
)

var nextControlID uintptr = 1000 // above IDOK, IDCANCEL and the other predefined IDs

// assignControlIDs gives every control inside root its own control ID.
//
// walk creates all controls with ID 0. When a button is pressed, its container
// looks the sender up with GetDlgItem(id), which returns the first child with
// ID 0 (often the label next to the button), so the click is delivered to the
// wrong window and silently lost. Unique IDs make that lookup find the button.
func assignControlIDs(root win.HWND) {
	for child := win.GetWindow(root, win.GW_CHILD); child != 0; child = win.GetWindow(child, win.GW_HWNDNEXT) {
		// Only children of walk containers; native controls such as list views
		// manage the IDs of their own internal children.
		if strings.HasPrefix(className(root), `\o/ Walk_`) && win.GetWindowLongPtr(child, win.GWLP_ID) == 0 {
			win.SetWindowLongPtr(child, win.GWLP_ID, nextControlID)
			nextControlID++
		}
		assignControlIDs(child)
	}
}

// fixTableView makes walk's TableView behave like one list for keyboard and
// screen-reader users.
//
// A TableView is two native list views inside a wrapper: one for frozen columns
// (unused here, zero width) and the real one. Both are Tab stops, and the frozen
// one forwards focus to the real one, which traps Shift+Tab. Neither has an
// accessible name, because walk annotates only the wrapper window.
func fixTableView(tv *walk.TableView, name, shortcut string) {
	var lists []win.HWND
	for child := win.GetWindow(tv.Handle(), win.GW_CHILD); child != 0; child = win.GetWindow(child, win.GW_HWNDNEXT) {
		if className(child) == "SysListView32" {
			lists = append(lists, child)
		}
	}
	if len(lists) != 2 {
		return
	}
	frozen, normal := lists[0], lists[1]

	style := win.GetWindowLong(frozen, win.GWL_STYLE)
	win.SetWindowLong(frozen, win.GWL_STYLE, style&^win.WS_TABSTOP)

	setAccessibleProp(normal, &win.PROPID_ACC_NAME, name)
	setAccessibleProp(normal, &win.PROPID_ACC_KEYBOARDSHORTCUT, shortcut)
}

// findDescendant returns the first window of the given class inside root.
func findDescendant(root win.HWND, class string) win.HWND {
	for child := win.GetWindow(root, win.GW_CHILD); child != 0; child = win.GetWindow(child, win.GW_HWNDNEXT) {
		if className(child) == class {
			return child
		}
		if found := findDescendant(child, class); found != 0 {
			return found
		}
	}
	return 0
}

func className(hwnd win.HWND) string {
	buf := make([]uint16, 256)
	n, _ := win.GetClassName(hwnd, &buf[0], len(buf))
	return syscall.UTF16ToString(buf[:n])
}

var accServices *win.IAccPropServices

// setAccessibleProp sets a property screen readers announce (such as the name) on
// a native control, using MSAA dynamic annotation, the same mechanism walk uses
// for its own widgets.
func setAccessibleProp(hwnd win.HWND, prop *win.MSAAPROPID, value string) {
	if accServices == nil {
		hr := win.CoCreateInstance(&win.CLSID_AccPropServices, nil, win.CLSCTX_INPROC_SERVER,
			&win.IID_IAccPropServices, (*unsafe.Pointer)(unsafe.Pointer(&accServices)))
		if win.FAILED(hr) {
			accServices = nil
			return
		}
	}
	accServices.SetHwndPropStr(hwnd, win.OBJID_CLIENT, win.CHILDID_SELF, prop, value)
}
