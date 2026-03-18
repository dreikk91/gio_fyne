//go:build windows

package walk

import (
	"syscall"

	"github.com/lxn/walk"
	"github.com/lxn/win"
)

func applyTableGridlineColor(tv *walk.TableView, color win.COLORREF) {
	if tv == nil || tv.Handle() == 0 {
		return
	}

	cb := syscall.NewCallback(func(hwnd win.HWND, lparam uintptr) uintptr {
		var className [64]uint16
		n, _ := win.GetClassName(hwnd, &className[0], len(className))
		if n > 0 && syscall.UTF16ToString(className[:n]) == "SysListView32" {
			win.SendMessage(hwnd, win.LVM_SETBKCOLOR, 0, uintptr(color))
			win.SendMessage(hwnd, win.LVM_SETTEXTBKCOLOR, 0, uintptr(color))
			win.InvalidateRect(hwnd, nil, true)
		}
		return 1
	})

	win.EnumChildWindows(tv.Handle(), cb, 0)
}
