package main

import (
	"strings"
	"syscall"
	"unsafe"

	"github.com/tailscale/walk"
	"github.com/tailscale/win"
)

// First-letter navigation (type-ahead) for walk's TableView.
//
// A TableView is a virtual list view (LVS_OWNERDATA): it asks the program for
// each row's text instead of storing it, so it can't search its own text. When
// a letter is typed, the list view keeps track of the typing (several quick
// letters form one prefix; the same letter repeated moves to the next match)
// and sends LVN_ODFINDITEM to its parent to ask which row matches. walk never
// answers, so typing letters did nothing useful. enableTypeAhead answers it.

// The Win32 LVFINDINFOW and NMLVFINDITEMW structures (not in tailscale/win).
type lvFindInfo struct {
	flags       uint32
	psz         *uint16
	lParam      uintptr
	pt          win.POINT
	vkDirection uint32
}

type nmLVFindItem struct {
	hdr    win.NMHDR
	iStart int32
	lvfi   lvFindInfo
}

const (
	lvfiString  = 0x0002
	lvfiPartial = 0x0008
	lvfiWrap    = 0x0020
)

// enableTypeAhead makes typed letters jump to the next row whose textAt (the
// first column, such as the app name) starts with them, among the count() rows
// the list currently shows (one page at a time in My apps). Every TableView the
// GUI creates should get this.
func enableTypeAhead(tv *walk.TableView, count func() int, textAt func(row int) string) {
	var lists []win.HWND
	for child := win.GetWindow(tv.Handle(), win.GW_CHILD); child != 0; child = win.GetWindow(child, win.GW_HWNDNEXT) {
		if className(child) == "SysListView32" {
			lists = append(lists, child)
		}
	}
	if len(lists) != 2 {
		return
	}
	normal := lists[1] // the real list; lists[0] holds the unused frozen columns

	wrapper := tv.Handle()
	var original uintptr
	original = win.SetWindowLongPtr(wrapper, win.GWLP_WNDPROC, syscall.NewCallback(
		func(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
			if msg == win.WM_NOTIFY {
				// lParam points to a structure owned by the sender (read without a uintptr-to-pointer conversion).
				nmh := *(**win.NMHDR)(unsafe.Pointer(&lParam))
				if nmh.HwndFrom == normal && nmh.Code == win.LVN_ODFINDITEM {
					find := *(**nmLVFindItem)(unsafe.Pointer(&lParam))
					if find.lvfi.flags&(lvfiString|lvfiPartial) == 0 || find.lvfi.psz == nil {
						return ^uintptr(0) // -1: other kinds of search aren't used
					}
					row := findByPrefix(count(), textAt, utf16PtrToString(find.lvfi.psz),
						int(find.iStart), find.lvfi.flags&lvfiWrap != 0)
					return uintptr(row) // -1 (no match) converts to the all-ones value Windows expects
				}
			}
			return win.CallWindowProc(original, hwnd, msg, wParam, lParam)
		}))
}

// findByPrefix returns the first row from start onwards whose text starts with
// prefix, ignoring capitals, continuing from the top if wrap is set; -1 if none.
func findByPrefix(count int, textAt func(row int) string, prefix string, start int, wrap bool) int {
	prefix = strings.ToLower(prefix)
	if prefix == "" || count <= 0 {
		return -1
	}
	if start < 0 || start >= count {
		if !wrap {
			return -1
		}
		start = 0
	}
	limit := count - start
	if wrap {
		limit = count
	}
	for i := 0; i < limit; i++ {
		row := (start + i) % count
		if strings.HasPrefix(strings.ToLower(textAt(row)), prefix) {
			return row
		}
	}
	return -1
}

// utf16PtrToString reads a zero-terminated UTF-16 string.
func utf16PtrToString(p *uint16) string {
	n := 0
	for ptr := unsafe.Pointer(p); *(*uint16)(ptr) != 0; ptr = unsafe.Add(ptr, 2) {
		n++
	}
	return syscall.UTF16ToString(unsafe.Slice(p, n))
}
