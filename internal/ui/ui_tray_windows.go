//go:build windows

package ui

import (
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/getlantern/systray"
	"github.com/go-toast/toast"
	"github.com/rs/zerolog/log"
)

func (m *model) installCloseIntercept() {
	m.win.SetCloseIntercept(func() {
		if m.cfg.UI.CloseToTray && !m.allowClose.Load() && m.trayReady.Load() {
			log.Info().Msg("window close intercepted, hiding to tray")
			m.hideWindow()
			m.notifyHiddenToTray()
			return
		}
		m.allowClose.Store(true)
		m.app.Quit()
	})
}

func (m *model) startTrayIfNeeded() {
	if !(m.cfg.UI.MinimizeToTray || m.cfg.UI.CloseToTray) {
		return
	}
	m.startTray()
}

func (m *model) startTray() {
	if !m.trayStarted.CompareAndSwap(false, true) {
		return
	}
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		systray.Run(func() {
			m.trayReady.Store(true)
			if icon := loadTrayIcon(); len(icon) > 0 {
				systray.SetIcon(icon)
			}
			systray.SetTitle("CID")
			systray.SetTooltip("CID Retranslator")
			showItem := systray.AddMenuItem("Show", "Show window")
			hideItem := systray.AddMenuItem("Hide", "Hide window")
			exitItem := systray.AddMenuItem("Exit", "Exit app")
			go func() {
				for {
					select {
					case <-showItem.ClickedCh:
						m.showWindow()
					case <-hideItem.ClickedCh:
						m.hideWindow()
						m.notifyHiddenToTray()
					case <-exitItem.ClickedCh:
						m.requestExit()
						return
					}
				}
			}()
		}, func() {
			m.trayReady.Store(false)
			m.trayStarted.Store(false)
		})
	}()
}

func (m *model) shutdownPlatform() {
	if m.trayStarted.Load() {
		systray.Quit()
		m.trayStarted.Store(false)
		m.trayReady.Store(false)
	}
}

func (m *model) requestExit() {
	log.Warn().Msg("exit requested from tray menu")
	m.allowClose.Store(true)
	m.app.Quit()
}

func (m *model) hideWindow() {
	if m.win != nil {
		m.win.Hide()
	}
}

func (m *model) showWindow() {
	if m.win != nil {
		m.win.Show()
		m.win.RequestFocus()
	}
}

func loadTrayIcon() []byte {
	wd, _ := os.Getwd()
	candidates := []string{
		filepath.Join(wd, "icon.ico"),
		filepath.Join(filepath.Dir(wd), "icon.ico"),
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "icon.ico"))
	}
	for _, p := range candidates {
		b, err := os.ReadFile(p)
		if err == nil && len(b) > 0 {
			return b
		}
	}
	return nil
}

func findTrayIconPath() string {
	wd, _ := os.Getwd()
	candidates := []string{
		filepath.Join(wd, "icon.ico"),
		filepath.Join(filepath.Dir(wd), "icon.ico"),
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "icon.ico"))
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func (m *model) notifyHiddenToTray() {
	if !m.trayReady.Load() {
		return
	}
	now := time.Now().Unix()
	last := m.trayNoticeAt.Load()
	if last > 0 && now-last < 10 {
		return
	}
	m.trayNoticeAt.Store(now)

	// Skip notification if icon can't be found to prevent COM errors
	iconPath := findTrayIconPath()
	if iconPath == "" {
		log.Debug().Msg("skipping tray notification: icon not found")
		return
	}

	n := toast.Notification{
		AppID:   "CID Retranslator",
		Title:   "CID Retranslator",
		Message: "Hidden to tray. Use the tray icon to restore.",
		Icon:    iconPath,
	}
	if err := n.Push(); err != nil {
		log.Warn().Err(err).Msg("tray notification failed")
	}
}
