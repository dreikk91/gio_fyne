//go:build windows

package ui

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"gioui.org/app"
	"gioui.org/io/system"
	"github.com/go-toast/toast"
	"github.com/getlantern/systray"
	"github.com/rs/zerolog/log"
	"golang.org/x/sys/windows"
)

const (
	wmClose       = 0x0010
	wmSize        = 0x0005
	sizeMinimized = 1
	swHide        = 0
	swShow        = 5
	swRestore     = 9
)

const gwlWndProc = ^uintptr(3) // -4 as uintptr

var (
	user32                = windows.NewLazySystemDLL("user32.dll")
	procSetWindowLongPtrW = user32.NewProc("SetWindowLongPtrW")
	procCallWindowProcW   = user32.NewProc("CallWindowProcW")
	procDefWindowProcW    = user32.NewProc("DefWindowProcW")
	procShowWindow        = user32.NewProc("ShowWindow")
	hookWndProc           = syscall.NewCallback(windowProcHook)
	hookInstalledByHWND   sync.Map // hwnd uintptr -> uintptr (old wndproc)
	hookModelByHWND       sync.Map // hwnd uintptr -> *model
)

func (m *model) onWin32ViewEvent(e app.Win32ViewEvent) {
	if !e.Valid() || e.HWND == 0 {
		return
	}
	if e.HWND == m.platformHWND().Load() {
		return
	}
	m.platformHWND().Store(e.HWND)
	if m.cfg.UI.MinimizeToTray || m.cfg.UI.CloseToTray {
		installCloseHook(e.HWND, m)
		m.startTray()
	} else {
		uninstallCloseHook(e.HWND)
	}
	if m.cfg.UI.StartMinimized {
		m.hideWindow()
	}
}

func (m *model) shutdownPlatform() {
	if m.trayStarted().Load() {
		systray.Quit()
		m.trayStarted().Store(false)
		m.trayReady().Store(false)
	}
	hwnd := m.platformHWND().Load()
	if hwnd != 0 {
		uninstallCloseHook(hwnd)
	}
}

func (m *model) startTray() {
	if !m.trayStarted().CompareAndSwap(false, true) {
		return
	}
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		systray.Run(func() {
			m.trayReady().Store(true)
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
					case <-exitItem.ClickedCh:
						m.requestExit()
						return
					}
				}
			}()
		}, func() {
			m.trayReady().Store(false)
			m.trayStarted().Store(false)
		})
	}()
}

func (m *model) requestExit() {
	log.Warn().Msg("exit requested from tray menu")
	m.allowClose().Store(true)
	m.w.Perform(system.ActionClose)
}

func (m *model) hideWindow() {
	hwnd := m.platformHWND().Load()
	if hwnd == 0 {
		return
	}
	_, _, _ = procShowWindow.Call(hwnd, swHide)
}

func (m *model) showWindow() {
	hwnd := m.platformHWND().Load()
	if hwnd == 0 {
		return
	}
	_, _, _ = procShowWindow.Call(hwnd, swShow)
	_, _, _ = procShowWindow.Call(hwnd, swRestore)
	m.w.Perform(system.ActionRaise)
}

func installCloseHook(hwnd uintptr, m *model) {
	hookModelByHWND.Store(hwnd, m)
	if _, exists := hookInstalledByHWND.Load(hwnd); exists {
		return
	}
	old, _, _ := procSetWindowLongPtrW.Call(hwnd, gwlWndProc, hookWndProc)
	if old != 0 {
		hookInstalledByHWND.Store(hwnd, old)
		return
	}
	// Hook installation failed, do not keep dangling model binding.
	hookModelByHWND.Delete(hwnd)
	log.Warn().Uint64("hwnd", uint64(hwnd)).Msg("failed to install close hook")
}

func uninstallCloseHook(hwnd uintptr) {
	if old, ok := hookInstalledByHWND.Load(hwnd); ok {
		_, _, _ = procSetWindowLongPtrW.Call(hwnd, gwlWndProc, old.(uintptr))
		hookInstalledByHWND.Delete(hwnd)
	}
	hookModelByHWND.Delete(hwnd)
}

func windowProcHook(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	if msg == wmClose {
		if v, ok := hookModelByHWND.Load(hwnd); ok {
			m := v.(*model)
			if m.cfg.UI.CloseToTray && !m.allowClose().Load() && m.trayReady().Load() {
				log.Info().Msg("window close intercepted, hiding to tray")
				go m.hideWindow()
				go m.notifyHiddenToTray()
				return 0
			}
			log.Warn().Bool("allow_exit", m.allowClose().Load()).Msg("window close forwarded to app")
		}
	}
	if msg == wmSize {
		if v, ok := hookModelByHWND.Load(hwnd); ok {
			m := v.(*model)
			if m.cfg.UI.MinimizeToTray && wparam == sizeMinimized && m.trayReady().Load() {
				go m.hideWindow()
				go m.notifyHiddenToTray()
				return 0
			}
		}
	}
	if old, ok := hookInstalledByHWND.Load(hwnd); ok {
		ret, _, _ := procCallWindowProcW.Call(old.(uintptr), hwnd, uintptr(msg), wparam, lparam)
		return ret
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wparam, lparam)
	return ret
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
	if !m.trayReady().Load() {
		return
	}
	now := time.Now().Unix()
	last := m.trayNotice().Load()
	if last > 0 && now-last < 10 {
		return
	}
	m.trayNotice().Store(now)
	n := toast.Notification{
		AppID:   "CID Retranslator",
		Title:   "CID Retranslator",
		Message: "Hidden to tray. Use the tray icon to restore.",
		Icon:    findTrayIconPath(),
	}
	if err := n.Push(); err != nil {
		log.Warn().Err(err).Msg("tray notification failed")
	}
}

type platformState struct {
	hwnd      atomic.Uintptr
	tray      atomic.Bool
	ready     atomic.Bool
	allowExit atomic.Bool
	noticeAt  atomic.Int64
}

func (m *model) ensurePlatformState() *platformState {
	if m.platform == nil {
		m.platform = &platformState{}
	}
	return m.platform
}

func (m *model) platformHWND() *atomic.Uintptr { return &m.ensurePlatformState().hwnd }
func (m *model) trayStarted() *atomic.Bool     { return &m.ensurePlatformState().tray }
func (m *model) trayReady() *atomic.Bool       { return &m.ensurePlatformState().ready }
func (m *model) allowClose() *atomic.Bool      { return &m.ensurePlatformState().allowExit }
func (m *model) trayNotice() *atomic.Int64     { return &m.ensurePlatformState().noticeAt }
