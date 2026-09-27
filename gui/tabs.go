package main

import (
	"syscall"
	"unsafe"

	"github.com/tailscale/walk"
	"github.com/tailscale/win"
)

// Page switching.
//
// When walk's TabWidget changes page while focus is anywhere inside it (the tab
// strip included), it moves focus to the new page's first control. Moving focus
// back to the tab strip afterwards makes screen readers announce "tab control"
// again on every arrow press. Instead, page changes run with the pages'
// controls briefly removed from the tab order, so walk finds nothing to focus
// and focus never leaves where it was.

// setupTabs finds the native tab strip and routes every way of changing page
// (arrow keys, clicks, Ctrl+Tab) through withoutPageAutofocus.
func (g *gui) setupTabs() {
	g.tabList = findDescendant(g.tabs.Handle(), "SysTabControl32")

	// Arrow keys and clicks: the tab strip tells its parent (walk's TabWidget
	// wrapper) with TCN_SELCHANGE. Intercept that message on the wrapper.
	wrapper := g.tabs.Handle()
	var original uintptr
	original = win.SetWindowLongPtr(wrapper, win.GWLP_WNDPROC, syscall.NewCallback(
		func(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
			if msg == win.WM_NOTIFY {
				// lParam points to an NMHDR owned by the sender (read without a uintptr-to-pointer conversion).
				if nmh := *(**win.NMHDR)(unsafe.Pointer(&lParam)); nmh.HwndFrom == g.tabList && int32(nmh.Code) == win.TCN_SELCHANGE {
					var result uintptr
					g.withoutPageAutofocus(func() { result = win.CallWindowProc(original, hwnd, msg, wParam, lParam) })
					return result
				}
			}
			return win.CallWindowProc(original, hwnd, msg, wParam, lParam)
		}))

	// Ctrl+Tab: walk handles it in the main window's pre-translate step; take it
	// over there and pass every other message on to walk.
	g.app.AddPreTranslateHandlerForHWND(g.mw.Handle(), keyRouter{g})
}

type keyRouter struct{ g *gui }

func (k keyRouter) OnPreTranslate(msg *win.MSG) bool {
	g := k.g
	if msg.Message == win.WM_KEYDOWN && msg.WParam == win.VK_TAB && win.GetKeyState(win.VK_CONTROL) < 0 &&
		(msg.HWnd == g.mw.Handle() || win.IsChild(g.mw.Handle(), msg.HWnd)) {
		count := g.tabs.Pages().Len()
		step := 1
		if win.GetKeyState(win.VK_SHIFT) < 0 {
			step = count - 1
		}
		g.goToPage((g.tabs.CurrentIndex() + step) % count)
		return true
	}
	// Enter in My apps' platform filter or Sort by: walk only claims Enter for
	// editable combo boxes, so the dialog manager would swallow it. An open
	// drop-down closes as usual.
	if msg.Message == win.WM_KEYDOWN && msg.WParam == win.VK_RETURN {
		for _, combo := range []*walk.ComboBox{g.purchases.filter, g.purchases.sort} {
			if combo != nil && msg.HWnd == combo.Handle() &&
				win.SendMessage(combo.Handle(), win.CB_GETDROPPEDSTATE, 0, 0) == 0 {
				g.myAppsComboEnter()
				return true
			}
		}
	}
	// Route keyboard messages through the dialog manager so Tab and Alt+letter
	// work in the main window, not just in dialogs.
	return g.mw.OnPreTranslate(msg)
}

// withoutPageAutofocus runs f with the pages' controls temporarily not Tab stops,
// which is how walk chooses what to focus after a page change.
func (g *gui) withoutPageAutofocus(f func()) {
	var stripped []win.HWND
	for i := 0; i < g.tabs.Pages().Len(); i++ {
		forEachDescendant(g.tabs.Pages().At(i).Handle(), func(hwnd win.HWND) {
			if style := win.GetWindowLong(hwnd, win.GWL_STYLE); style&win.WS_TABSTOP != 0 {
				win.SetWindowLong(hwnd, win.GWL_STYLE, style&^win.WS_TABSTOP)
				stripped = append(stripped, hwnd)
			}
		})
	}
	defer func() {
		for _, hwnd := range stripped {
			win.SetWindowLong(hwnd, win.GWL_STYLE, win.GetWindowLong(hwnd, win.GWL_STYLE)|win.WS_TABSTOP)
		}
	}()
	f()
}

// goToPage switches page and puts focus on the tab strip, so screen readers
// announce the page. If focus is already there, only the selected tab changes.
func (g *gui) goToPage(index int) {
	g.showPage(index)
	if win.GetFocus() != g.tabList {
		win.SetFocus(g.tabList)
	}
}

// showPage switches page without moving focus; callers that want a specific
// control focused (e.g. after Enter on a search result) set it afterwards.
func (g *gui) showPage(index int) {
	g.withoutPageAutofocus(func() { _ = g.tabs.SetCurrentIndex(index) })
}

// focusWidget switches to the page and focuses one of its controls.
func (g *gui) focusWidget(page int, w walk.Widget) {
	g.showPage(page)
	_ = w.SetFocus()
}

func forEachDescendant(root win.HWND, f func(win.HWND)) {
	for child := win.GetWindow(root, win.GW_CHILD); child != 0; child = win.GetWindow(child, win.GW_HWNDNEXT) {
		f(child)
		forEachDescendant(child, f)
	}
}
