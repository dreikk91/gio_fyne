package ui

import (
	"fmt"
	"strconv"
	"strings"

	"cid_gio_gio/internal/config"
	appLog "cid_gio_gio/internal/logger"

	"github.com/rs/zerolog/log"

	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func (m *model) handleInputs(gtx layout.Context) {
	m.handleSearchInputs(gtx)
	m.handleDeleteInputs(gtx)
	m.handleLogLevelInputs(gtx)
	m.handleTabInputs(gtx)
	m.handleSettingsInputs(gtx)
	m.handleHistoryInputs(gtx)
	m.handleRfInteractions(gtx)
	m.handleCtxMenuInputs(gtx)
}

func (m *model) handleSearchInputs(gtx layout.Context) {
	if q := strings.TrimSpace(m.objSearch.Text()); q != m.deviceFilter {
		m.deviceFilter = q
		m.applyFilters()
	}
	if q := strings.TrimSpace(m.evtSearch.Text()); q != m.eventQuery {
		m.eventQuery = q
		m.applyFilters()
	}
	if m.hideTestsBox.Update(gtx) {
		m.hideTests = m.hideTestsBox.Value
		m.applyFilters()
	}
	if m.hideBlockedBox.Update(gtx) {
		m.hideBlocked = m.hideBlockedBox.Value
		m.applyFilters()
	}
}

func (m *model) handleDeleteInputs(gtx layout.Context) {
	if !m.delOpen {
		return
	}
	for m.delBackdrop.Clicked(gtx) {
		m.delOpen = false
	}
	for m.delCancel.Clicked(gtx) {
		m.delOpen = false
	}
	for m.delConfirm.Clicked(gtx) {
		if m.delBusy.CompareAndSwap(false, true) {
			go m.deleteDeviceRemote(m.delDevice.ID)
		}
	}
}

func (m *model) handleLogLevelInputs(gtx layout.Context) {
	for m.logLevelBtn.Clicked(gtx) {
		m.logLevelOpen = !m.logLevelOpen
	}
	if !m.logLevelOpen {
		return
	}
	for _, lvl := range logLevels {
		for m.logLevelBtnMap[lvl].Clicked(gtx) {
			m.cfgFields["Logging.Level"].SetText(lvl)
			m.cfg.Logging.Level = lvl
			if err := appLog.SetLevel(lvl); err != nil {
				m.statusErr = "Invalid log level: " + lvl
				log.Error().Err(err).Str("level", lvl).Msg("update log level failed")
			} else {
				m.statusErr = ""
				m.statusMsg = "Log level applied: " + strings.ToUpper(lvl)
				log.Info().Str("level", lvl).Msg("log level applied from ui")
			}
			m.logLevelOpen = false
		}
	}
}

func (m *model) handleTabInputs(gtx layout.Context) {
	for m.tabBtn[0].Clicked(gtx) {
		m.tab = 0
	}
	for m.tabBtn[1].Clicked(gtx) {
		m.tab = 1
	}
	for m.tabBtn[2].Clicked(gtx) {
		m.tab = 2
	}
	for _, f := range eventFilters {
		for m.filterBtns[f].Clicked(gtx) {
			m.eventFilter = f
			m.applyFilters()
		}
	}
}

func (m *model) handleSettingsInputs(gtx layout.Context) {
	for m.saveCfg.Clicked(gtx) {
		cfg, err := m.collectCfg()
		if err != nil {
			m.statusErr = "Save failed: " + err.Error()
			log.Error().Err(err).Msg("collect config failed")
			continue
		}
		go m.saveConfigRemote(cfg)
	}
	for m.resetCfg.Clicked(gtx) {
		m.applyFontSize(m.cfg.UI.FontSize)
		m.loadCfgEditors(m.cfg)
	}
	for m.rfOpenBtn.Clicked(gtx) {
		m.openRelayFilter()
	}
}

func (m *model) handleHistoryInputs(gtx layout.Context) {
	if m.hOpen {
		if q := strings.TrimSpace(m.hSearch.Text()); q != m.hQueryCache {
			m.hQueryCache = q
			m.hLimit = m.initialDeviceHistoryLimit()
			m.requestHistoryReload()
		}
		if m.hHideBox.Update(gtx) {
			m.hHideTests = m.hHideBox.Value
			m.hLimit = m.initialDeviceHistoryLimit()
			m.requestHistoryReload()
		}
		if m.hHideBlockedBox.Update(gtx) {
			m.hHideBlocked = m.hHideBlockedBox.Value
			m.hLimit = m.initialDeviceHistoryLimit()
			m.requestHistoryReload()
		}
	}
	for m.hReload.Clicked(gtx) {
		m.requestHistoryReloadNow()
	}
	for m.hClose.Clicked(gtx) {
		m.hOpen = false
		m.historyBusy.Store(false)
	}
	for _, f := range eventFilters {
		for m.hFilterBtns[f].Clicked(gtx) {
			m.hEventType = f
			m.hLimit = m.initialDeviceHistoryLimit()
			m.requestHistoryReload()
		}
	}
}

func (m *model) settingsTab(gtx layout.Context) layout.Dimensions {
	return panel(gtx, cPanel, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, m.networkSection)
					}),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, m.historySection)
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Bottom: unit.Dp(8)}.Layout(gtx, m.rulesSection)
					}),
					layout.Rigid(m.loggingSection),
				)
			}),
		)
	})
}

func (m *model) networkSection(gtx layout.Context) layout.Dimensions {
	return settingsSection(m.th, gtx, "Network & Queue", func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(field(m, "Server host", "Server.Host")),
			layout.Rigid(field(m, "Server port", "Server.Port")),
			layout.Rigid(field(m, "Client host", "Client.Host")),
			layout.Rigid(field(m, "Client port", "Client.Port")),
			layout.Rigid(field(m, "Queue buffer", "Queue.BufferSize")),
			layout.Rigid(field(m, "PPK timeout", "Monitoring.PpkTimeout")),
		)
	})
}

func (m *model) historySection(gtx layout.Context) layout.Dimensions {
	return settingsSection(m.th, gtx, "History & Interface", func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(field(m, "History global", "History.GlobalLimit")),
			layout.Rigid(field(m, "History log", "History.LogLimit")),
			layout.Rigid(fontSlider(m)),
			layout.Rigid(flag(m, "UI.StartMinimized", "Start minimized")),
			layout.Rigid(flag(m, "UI.MinimizeToTray", "Minimize to tray")),
			layout.Rigid(flag(m, "UI.CloseToTray", "Close to tray")),
		)
	})
}

func (m *model) rulesSection(gtx layout.Context) layout.Dimensions {
	return settingsSection(m.th, gtx, "CID Rules", func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(field(m, "Required prefix", "CidRules.RequiredPrefix")),
			layout.Rigid(field(m, "Valid length", "CidRules.ValidLength")),
			layout.Rigid(field(m, "Default acc add", "CidRules.AccNumAdd")),
			layout.Rigid(multilineField(m, "Account ranges (From-To:Delta)", "CidRules.AccountRanges", 110)),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return secondaryButton(gtx, m.th, &m.rfOpenBtn, "Configure Relay Filter")
			}),
		)
	})
}

func (m *model) loggingSection(gtx layout.Context) layout.Dimensions {
	return settingsSection(m.th, gtx, "Logging & Actions", func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(logLevelField(m)),
			layout.Rigid(flag(m, "Logging.EnableConsole", "Enable console logs")),
			layout.Rigid(flag(m, "Logging.EnableFile", "Enable file logs")),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				lbl := material.Body2(m.th, fmt.Sprintf("Current font size: %d", clamp(atoiOr(m.cfg.UI.FontSize, m.cfgFields["UI.FontSize"].Text()), 7, 30)))
				lbl.Color = cSoft
				return lbl.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: unit.Dp(8)}.Layout),
			layout.Rigid(m.settingsActions),
		)
	})
}

func (m *model) settingsActions(gtx layout.Context) layout.Dimensions {
	return layout.Flex{}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return accentButton(gtx, m.th, &m.saveCfg, "Save")
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(8)}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return secondaryButton(gtx, m.th, &m.resetCfg, "Reset")
		}),
	)
}

func (m *model) ensureRows(n int) {
	if cap(m.objRows) < n {
		m.objRows = make([]widget.Clickable, n)
	} else {
		m.objRows = m.objRows[:n]
	}
	if cap(m.objRCTags) < n {
		m.objRCTags = make([]bool, n)
	} else {
		m.objRCTags = m.objRCTags[:n]
	}
}

func (m *model) handleCtxMenuInputs(gtx layout.Context) {
	if !m.ctxMenuOpen {
		return
	}
	for m.ctxMenuDel.Clicked(gtx) {
		m.delDevice = m.ctxMenuDevice
		m.delOpen = true
		m.delBusy.Store(false)
		m.ctxMenuOpen = false
	}
	for m.ctxMenuHist.Clicked(gtx) {
		m.openHistory(m.ctxMenuDevice)
		m.ctxMenuOpen = false
	}
	for m.ctxMenuClose.Clicked(gtx) {
		m.ctxMenuOpen = false
	}
}

func (m *model) loadCfgEditors(cfg config.AppConfig) {
	setText := func(k, v string) {
		e, ok := m.cfgFields[k]
		if !ok {
			e = &widget.Editor{SingleLine: true}
			m.cfgFields[k] = e
		}
		if k == "CidRules.AccountRanges" {
			e.SingleLine = false
			e.Submit = false
		} else {
			e.SingleLine = true
		}
		e.SetText(v)
	}
	setFlag := func(k string, v bool) {
		b, ok := m.cfgFlags[k]
		if !ok {
			b = &widget.Bool{}
			m.cfgFlags[k] = b
		}
		b.Value = v
	}
	setText("Server.Host", cfg.Server.Host)
	setText("Server.Port", cfg.Server.Port)
	setText("Client.Host", cfg.Client.Host)
	setText("Client.Port", cfg.Client.Port)
	setText("Queue.BufferSize", strconv.Itoa(cfg.Queue.BufferSize))
	setText("Monitoring.PpkTimeout", cfg.Monitoring.PpkTimeout)
	setText("Logging.Level", cfg.Logging.Level)
	setText("History.GlobalLimit", strconv.Itoa(cfg.History.GlobalLimit))
	setText("History.LogLimit", strconv.Itoa(cfg.History.LogLimit))
	setText("UI.FontSize", strconv.Itoa(cfg.UI.FontSize))
	m.fontSize.Value = sliderValueFromFontSize(cfg.UI.FontSize)
	setText("CidRules.RequiredPrefix", cfg.CidRules.RequiredPrefix)
	setText("CidRules.ValidLength", strconv.Itoa(cfg.CidRules.ValidLength))
	setText("CidRules.AccNumAdd", strconv.Itoa(cfg.CidRules.AccNumAdd))
	setText("CidRules.AccountRanges", formatAccountRanges(cfg.CidRules.AccountRanges))
	setFlag("UI.StartMinimized", cfg.UI.StartMinimized)
	setFlag("UI.MinimizeToTray", cfg.UI.MinimizeToTray)
	setFlag("UI.CloseToTray", cfg.UI.CloseToTray)
	setFlag("Logging.EnableConsole", cfg.Logging.EnableConsole)
	setFlag("Logging.EnableFile", cfg.Logging.EnableFile)
}

func (m *model) collectCfg() (config.AppConfig, error) {
	cfg := m.cfg
	cfg.Server.Host = m.cfgFields["Server.Host"].Text()
	cfg.Server.Port = m.cfgFields["Server.Port"].Text()
	cfg.Client.Host = m.cfgFields["Client.Host"].Text()
	cfg.Client.Port = m.cfgFields["Client.Port"].Text()
	cfg.Queue.BufferSize = atoiOr(cfg.Queue.BufferSize, m.cfgFields["Queue.BufferSize"].Text())
	cfg.Monitoring.PpkTimeout = m.cfgFields["Monitoring.PpkTimeout"].Text()
	cfg.Logging.Level = strings.TrimSpace(m.cfgFields["Logging.Level"].Text())
	cfg.History.GlobalLimit = atoiOr(cfg.History.GlobalLimit, m.cfgFields["History.GlobalLimit"].Text())
	cfg.History.LogLimit = atoiOr(cfg.History.LogLimit, m.cfgFields["History.LogLimit"].Text())
	cfg.CidRules.RequiredPrefix = strings.TrimSpace(m.cfgFields["CidRules.RequiredPrefix"].Text())
	cfg.CidRules.ValidLength = atoiOr(cfg.CidRules.ValidLength, m.cfgFields["CidRules.ValidLength"].Text())
	cfg.CidRules.AccNumAdd = atoiOr(cfg.CidRules.AccNumAdd, m.cfgFields["CidRules.AccNumAdd"].Text())
	ranges, err := parseAccountRanges(m.cfgFields["CidRules.AccountRanges"].Text())
	if err != nil {
		return cfg, err
	}
	if len(ranges) > 0 {
		cfg.CidRules.AccountRanges = ranges
	}
	cfg.UI.FontSize = clamp(atoiOr(cfg.UI.FontSize, m.cfgFields["UI.FontSize"].Text()), 7, 30)
	cfg.UI.StartMinimized = m.cfgFlags["UI.StartMinimized"].Value
	cfg.UI.MinimizeToTray = m.cfgFlags["UI.MinimizeToTray"].Value
	cfg.UI.CloseToTray = m.cfgFlags["UI.CloseToTray"].Value
	cfg.Logging.EnableConsole = m.cfgFlags["Logging.EnableConsole"].Value
	cfg.Logging.EnableFile = m.cfgFlags["Logging.EnableFile"].Value
	config.Normalize(&cfg)
	return cfg, nil
}

func (m *model) applyFontSize(size int) {
	size = clamp(size, 7, 30)
	m.th.TextSize = unit.Sp(size)
}
