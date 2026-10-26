//go:build !windows

package ui

import "github.com/egoist/mygo"

// dropFloatingFrame does nothing outside Windows: only DWM rounds, and so
// outlines, the control's window. macOS and Linux draw no frame around it.
func dropFloatingFrame(w *mygo.Window) {}
