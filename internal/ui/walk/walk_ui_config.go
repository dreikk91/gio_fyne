//go:build windows

package walk

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"cid_fyne/internal/config"
	"cid_fyne/internal/core"
	"github.com/lxn/walk"
	"github.com/lxn/win"
)

func (a *walkApp) loadConfigEditors() {
	if a.serverHost == nil {
		return
	}
	a.serverHost.SetText(a.cfg.Server.Host)
	a.serverPort.SetText(a.cfg.Server.Port)
	a.clientHost.SetText(a.cfg.Client.Host)
	a.clientPort.SetText(a.cfg.Client.Port)
	a.reconnectInit.SetText(a.cfg.Client.ReconnectInitial)
	a.reconnectMax.SetText(a.cfg.Client.ReconnectMax)
	a.queueBuffer.SetText(fmt.Sprintf("%d", a.cfg.Queue.BufferSize))
	a.ppkTimeout.SetText(a.cfg.Monitoring.PpkTimeout)
	a.logDir.SetText(a.cfg.Logging.LogDir)
	a.logFilename.SetText(a.cfg.Logging.Filename)
	a.logMaxSize.SetText(fmt.Sprintf("%d", a.cfg.Logging.MaxSize))
	a.logMaxBackups.SetText(fmt.Sprintf("%d", a.cfg.Logging.MaxBackups))
	a.logMaxAge.SetText(fmt.Sprintf("%d", a.cfg.Logging.MaxAge))
	a.logConsole.SetChecked(a.cfg.Logging.EnableConsole)
	a.logFile.SetChecked(a.cfg.Logging.EnableFile)
	a.logPretty.SetChecked(a.cfg.Logging.PrettyConsole)
	a.logSampling.SetChecked(a.cfg.Logging.SamplingEnabled)
	a.setComboValue(a.logLevel, logLevels, a.cfg.Logging.Level)
	a.historyGlobal.SetText(fmt.Sprintf("%d", a.cfg.History.GlobalLimit))
	a.historyLog.SetText(fmt.Sprintf("%d", a.cfg.History.LogLimit))
	a.historyRetention.SetText(fmt.Sprintf("%d", a.cfg.History.RetentionDays))
	a.historyCleanup.SetText(fmt.Sprintf("%d", a.cfg.History.CleanupIntervalHours))
	a.historyArchivePath.SetText(a.cfg.History.ArchiveDBPath)
	a.historyBatch.SetText(fmt.Sprintf("%d", a.cfg.History.MaintenanceBatch))
	a.historyArchive.SetChecked(a.cfg.History.ArchiveEnabled)
	a.uiStartMin.SetChecked(a.cfg.UI.StartMinimized)
	a.uiMinTray.SetChecked(a.cfg.UI.MinimizeToTray)
	a.uiCloseTray.SetChecked(a.cfg.UI.CloseToTray)
	if a.uiFontCombo != nil {
		a.uiFontCombo.SetText(strconv.Itoa(a.cfg.UI.FontSize))
	}
	if a.uiFontValue != nil {
		a.uiFontValue.SetText(fmt.Sprintf("%d", a.cfg.UI.FontSize))
	}
	a.requiredPrefix.SetText(a.cfg.CidRules.RequiredPrefix)
	a.validLength.SetText(fmt.Sprintf("%d", a.cfg.CidRules.ValidLength))
	a.accountRanges.SetText(formatAccountRanges(a.cfg.CidRules.AccountRanges))
}

func (a *walkApp) collectConfigFromEditors() (config.AppConfig, error) {
	cfg := a.cfg
	cfg.Server.Host = strings.TrimSpace(a.serverHost.Text())
	cfg.Server.Port = strings.TrimSpace(a.serverPort.Text())
	cfg.Client.Host = strings.TrimSpace(a.clientHost.Text())
	cfg.Client.Port = strings.TrimSpace(a.clientPort.Text())
	cfg.Client.ReconnectInitial = strings.TrimSpace(a.reconnectInit.Text())
	cfg.Client.ReconnectMax = strings.TrimSpace(a.reconnectMax.Text())
	cfg.Queue.BufferSize = atoiOr(cfg.Queue.BufferSize, a.queueBuffer.Text())
	cfg.Monitoring.PpkTimeout = strings.TrimSpace(a.ppkTimeout.Text())
	cfg.Logging.LogDir = strings.TrimSpace(a.logDir.Text())
	cfg.Logging.Filename = strings.TrimSpace(a.logFilename.Text())
	cfg.Logging.MaxSize = atoiOr(cfg.Logging.MaxSize, a.logMaxSize.Text())
	cfg.Logging.MaxBackups = atoiOr(cfg.Logging.MaxBackups, a.logMaxBackups.Text())
	cfg.Logging.MaxAge = atoiOr(cfg.Logging.MaxAge, a.logMaxAge.Text())
	cfg.Logging.EnableConsole = a.logConsole.Checked()
	cfg.Logging.EnableFile = a.logFile.Checked()
	cfg.Logging.PrettyConsole = a.logPretty.Checked()
	cfg.Logging.SamplingEnabled = a.logSampling.Checked()
	cfg.History.GlobalLimit = atoiOr(cfg.History.GlobalLimit, a.historyGlobal.Text())
	cfg.History.LogLimit = atoiOr(cfg.History.LogLimit, a.historyLog.Text())
	cfg.History.RetentionDays = atoiOr(cfg.History.RetentionDays, a.historyRetention.Text())
	cfg.History.CleanupIntervalHours = atoiOr(cfg.History.CleanupIntervalHours, a.historyCleanup.Text())
	cfg.History.ArchiveDBPath = strings.TrimSpace(a.historyArchivePath.Text())
	cfg.History.MaintenanceBatch = atoiOr(cfg.History.MaintenanceBatch, a.historyBatch.Text())
	cfg.History.ArchiveEnabled = a.historyArchive.Checked()
	cfg.UI.StartMinimized = a.uiStartMin.Checked()
	cfg.UI.MinimizeToTray = a.uiMinTray.Checked()
	cfg.UI.CloseToTray = a.uiCloseTray.Checked()
	if a.uiFontCombo != nil {
		val, _ := strconv.Atoi(a.uiFontCombo.Text())
		if val >= 7 && val <= 30 {
			cfg.UI.FontSize = val
		}
	}
	cfg.CidRules.RequiredPrefix = strings.TrimSpace(a.requiredPrefix.Text())
	cfg.CidRules.ValidLength = atoiOr(cfg.CidRules.ValidLength, a.validLength.Text())
	if a.logLevel.CurrentIndex() >= 0 && a.logLevel.CurrentIndex() < len(logLevels) {
		cfg.Logging.Level = logLevels[a.logLevel.CurrentIndex()]
	}
	ranges, err := parseAccountRanges(a.accountRanges.Text())
	if err != nil {
		return cfg, err
	}
	if len(ranges) > 0 {
		cfg.CidRules.AccountRanges = ranges
	}
	config.Normalize(&cfg)
	return cfg, nil
}

func (a *walkApp) saveConfig() {
	cfg, err := a.collectConfigFromEditors()
	if err != nil {
		walk.MsgBox(a.mw, "Налаштування", err.Error(), walk.MsgBoxIconError)
		return
	}
	reqID := a.saveReqID.Add(1)
	go func(cfg config.AppConfig) {
		err := a.rt.SaveConfig(a.ctx, cfg)
		if err == nil {
			cfg = a.rt.GetConfig()
		}
		if a.mw == nil {
			return
		}
		a.mw.Synchronize(func() {
			if reqID != a.saveReqID.Load() {
				return
			}
			if err != nil {
				a.statusErr = "Save failed: " + err.Error()
				a.updateStatusBar()
				walk.MsgBox(a.mw, "Налаштування", err.Error(), walk.MsgBoxIconError)
				return
			}
			a.cfg = cfg
			a.activityTO = core.ParseDuration(a.cfg.Monitoring.PpkTimeout, 15*time.Minute)
			a.historyLimit = a.initialGlobalLimit()
			a.eventsLimit = a.historyLimit
			a.eventsAllShown.Store(false)
			a.loadConfigEditors()
			a.status = "Налаштування збережено"
			a.statusErr = ""
			a.updateStatusBar()
			a.reloadAll()
		})
	}(cfg)
}

func (a *walkApp) setComboValue(box *walk.ComboBox, values []string, value string) {
	if box == nil {
		return
	}
	value = strings.ToLower(strings.TrimSpace(value))
	for i, item := range values {
		if strings.EqualFold(item, value) {
			box.SetCurrentIndex(i)
			return
		}
	}
	if len(values) > 0 {
		box.SetCurrentIndex(0)
	}
}

func (a *walkApp) setupNotifyIcon() error {
	icon, _ := walk.Resources.Icon("APPICON")
	if icon == nil {
		if fileIcon, err := walk.NewIconFromFile(resolveIconPath()); err == nil {
			icon = fileIcon
		}
	}
	notifyIcon, err := walk.NewNotifyIcon(a.mw)
	if err != nil {
		return err
	}
	a.notifyIcon = notifyIcon
	if icon != nil {
		_ = notifyIcon.SetIcon(icon)
	}
	_ = notifyIcon.SetToolTip("CID Ретранслятор")
	_ = notifyIcon.SetVisible(true)
	notifyIcon.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button == walk.LeftButton {
			a.toggleWindowVisibility()
		}
	})
	showHide := walk.NewAction()
	showHide.SetText("Показати/Сховати")
	showHide.Triggered().Attach(a.toggleWindowVisibility)
	exitAction := walk.NewAction()
	exitAction.SetText("Вихід")
	exitAction.Triggered().Attach(func() {
		_ = notifyIcon.SetVisible(false)
		a.mw.Dispose()
	})
	notifyIcon.ContextMenu().Actions().Add(showHide)
	notifyIcon.ContextMenu().Actions().Add(exitAction)
	return nil
}

func (a *walkApp) toggleWindowVisibility() {
	if a.mw.Visible() {
		a.mw.Hide()
		return
	}
	a.mw.Show()
	win.ShowWindow(a.mw.Handle(), win.SW_RESTORE)
	win.SetForegroundWindow(a.mw.Handle())
}

func (a *walkApp) applyStartVisibility() {
	if a.cfg.UI.StartMinimized && a.mw != nil {
		a.mw.Hide()
	}
}

func (a *walkApp) initialGlobalLimit() int {
	return maxInt(1, a.cfg.History.GlobalLimit)
}

func (a *walkApp) initialDeviceHistoryLimit() int {
	base := maxInt(1, a.cfg.History.LogLimit)
	if base < historyLoadChunk {
		base = historyLoadChunk
	}
	return minInt(a.deviceHistoryCap(), base)
}

func (a *walkApp) deviceHistoryCap() int {
	return maxInt(1, int(^uint(0)>>1))
}

func (a *walkApp) applyHistoryTableScale(state *historyDialog) {
	if state == nil || state.table == nil {
		return
	}
	base := clampInt(a.cfg.UI.FontSize, 7, 30)
	font, err := walk.NewFont("Segoe UI", base, 0)
	if err == nil {
		state.table.SetFont(font)
	}
	rowScale := clampInt(base-10, 0, 16)
	_ = state.table.SetMinMaxSize(walk.Size{Height: 260 + rowScale*6}, walk.Size{})
}

func addLimitSafe(current, delta int) int {
	if delta <= 0 {
		delta = 1
	}
	maxIntValue := int(^uint(0) >> 1)
	if current > maxIntValue-delta {
		return maxIntValue
	}
	return current + delta
}

func resolveIconPath() string {
	if wd, err := os.Getwd(); err == nil {
		p := filepath.Join(wd, "icon.ico")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "icon.ico")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "icon.ico"
}
