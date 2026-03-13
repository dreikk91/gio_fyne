//go:build !windows

package ui

func (m *model) installCloseIntercept() {}
func (m *model) startTrayIfNeeded()    {}
func (m *model) shutdownPlatform()     {}
func (m *model) hideWindow()           {}
func (m *model) showWindow()           {}
func (m *model) notifyHiddenToTray()   {}
