//go:build windows

package ui

import (
	"syscall"
	"unsafe"

	"github.com/egoist/mygo"
)

// Windows 11 window attributes and values this file needs. They are DWM's
// own numbers, not MyGo's: MyGo keeps its copies unexported.
const (
	// dwmwaWindowCornerPreference is DWMWA_WINDOW_CORNER_PREFERENCE.
	dwmwaWindowCornerPreference = 33
	// dwmwcpDoNotRound is DWMWCP_DONOTROUND: square corners, no outline.
	dwmwcpDoNotRound = 1
)

var (
	dwmapi               = syscall.NewLazyDLL("dwmapi.dll")
	procDwmSetWindowAttr = dwmapi.NewProc("DwmSetWindowAttribute")
)

// dropFloatingFrame tells Windows not to round, and so not to outline, the
// control's window: MyGo rounds a Panel as it makes it, and Windows 11 draws
// a hairline round any window it rounds, a square well outside the pie.
// Square corners keep the Panel, and what it brings (out of Alt-Tab, no
// taskbar button, on top).
//
// Best effort: Windows 10 ignores the attribute, and without DWM the call
// fails; either way only the hairline is left.
func dropFloatingFrame(w *mygo.Window) {
	if w == nil || procDwmSetWindowAttr.Find() != nil {
		return
	}
	hwnd := w.NativeHandle()
	if hwnd == 0 {
		return
	}
	pref := int32(dwmwcpDoNotRound)
	// The HRESULT is deliberately ignored: on Windows 10 and without DWM
	// this fails harmlessly, and there is nothing to fall back to.
	procDwmSetWindowAttr.Call(hwnd, dwmwaWindowCornerPreference, uintptr(unsafe.Pointer(&pref)), unsafe.Sizeof(pref))
}
