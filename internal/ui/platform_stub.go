//go:build !windows

package ui

import "gioui.org/app"

type platformState struct{}

func (m *model) onWin32ViewEvent(_ app.Win32ViewEvent) {}
func (m *model) shutdownPlatform()                     {}
