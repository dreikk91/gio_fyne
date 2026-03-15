package ui

import (
	"strconv"
	"strings"

	"cid_fyne/internal/config"

	"fyne.io/fyne/v2/widget"
)

func (m *model) loadCfgEditors(cfg config.AppConfig) {
	setText := func(k, v string) {
		e, ok := m.cfgEntries[k]
		if !ok {
			e = widget.NewEntry()
			m.cfgEntries[k] = e
		}
		e.SetText(v)
	}
	setFlag := func(k string, v bool) {
		b, ok := m.cfgChecks[k]
		if !ok {
			b = widget.NewCheck("", nil)
			m.cfgChecks[k] = b
		}
		b.SetChecked(v)
	}
	setText("Server.Host", cfg.Server.Host)
	setText("Server.Port", cfg.Server.Port)
	setText("Client.Host", cfg.Client.Host)
	setText("Client.Port", cfg.Client.Port)
	setText("Queue.BufferSize", strconv.Itoa(cfg.Queue.BufferSize))
	setText("Monitoring.PpkTimeout", cfg.Monitoring.PpkTimeout)
	setText("Logging.Level", cfg.Logging.Level)
	setText("Profiling.Host", cfg.Profiling.Host)
	setText("Profiling.Port", cfg.Profiling.Port)
	setText("History.GlobalLimit", strconv.Itoa(cfg.History.GlobalLimit))
	setText("History.LogLimit", strconv.Itoa(cfg.History.LogLimit))
	setText("UI.FontSize", strconv.Itoa(cfg.UI.FontSize))
	if m.fontSizeSlider != nil {
		m.fontSizeSlider.SetValue(float64(cfg.UI.FontSize))
	}
	setText("CidRules.RequiredPrefix", cfg.CidRules.RequiredPrefix)
	setText("CidRules.ValidLength", strconv.Itoa(cfg.CidRules.ValidLength))
	setText("CidRules.AccNumAdd", strconv.Itoa(cfg.CidRules.AccNumAdd))
	setText("CidRules.AccountRanges", formatAccountRanges(cfg.CidRules.AccountRanges))
	setFlag("UI.StartMinimized", cfg.UI.StartMinimized)
	setFlag("UI.MinimizeToTray", cfg.UI.MinimizeToTray)
	setFlag("UI.CloseToTray", cfg.UI.CloseToTray)
	setFlag("Logging.EnableConsole", cfg.Logging.EnableConsole)
	setFlag("Logging.EnableFile", cfg.Logging.EnableFile)
	setFlag("Profiling.Enabled", cfg.Profiling.Enabled)

	if m.logLevelSelect != nil {
		m.logLevelSelect.SetSelected(cfg.Logging.Level)
	}
	if m.fontSizeLabel != nil {
		m.fontSizeLabel.Text = "Current font size: " + strconv.Itoa(cfg.UI.FontSize)
		m.fontSizeLabel.Refresh()
	}
}

func (m *model) collectCfg() (config.AppConfig, error) {
	cfg := m.cfg
	cfg.Server.Host = m.cfgEntries["Server.Host"].Text
	cfg.Server.Port = m.cfgEntries["Server.Port"].Text
	cfg.Client.Host = m.cfgEntries["Client.Host"].Text
	cfg.Client.Port = m.cfgEntries["Client.Port"].Text
	cfg.Queue.BufferSize = atoiOr(cfg.Queue.BufferSize, m.cfgEntries["Queue.BufferSize"].Text)
	cfg.Monitoring.PpkTimeout = m.cfgEntries["Monitoring.PpkTimeout"].Text
	cfg.Logging.Level = strings.TrimSpace(m.cfgEntries["Logging.Level"].Text)
	cfg.Profiling.Host = strings.TrimSpace(m.cfgEntries["Profiling.Host"].Text)
	cfg.Profiling.Port = strings.TrimSpace(m.cfgEntries["Profiling.Port"].Text)
	cfg.History.GlobalLimit = atoiOr(cfg.History.GlobalLimit, m.cfgEntries["History.GlobalLimit"].Text)
	cfg.History.LogLimit = atoiOr(cfg.History.LogLimit, m.cfgEntries["History.LogLimit"].Text)
	cfg.CidRules.RequiredPrefix = strings.TrimSpace(m.cfgEntries["CidRules.RequiredPrefix"].Text)
	cfg.CidRules.ValidLength = atoiOr(cfg.CidRules.ValidLength, m.cfgEntries["CidRules.ValidLength"].Text)
	cfg.CidRules.AccNumAdd = atoiOr(cfg.CidRules.AccNumAdd, m.cfgEntries["CidRules.AccNumAdd"].Text)
	ranges, err := parseAccountRanges(m.cfgEntries["CidRules.AccountRanges"].Text)
	if err != nil {
		return cfg, err
	}
	if len(ranges) > 0 {
		cfg.CidRules.AccountRanges = ranges
	}
	cfg.UI.FontSize = clamp(atoiOr(cfg.UI.FontSize, m.cfgEntries["UI.FontSize"].Text), 7, 30)
	cfg.UI.StartMinimized = m.cfgChecks["UI.StartMinimized"].Checked
	cfg.UI.MinimizeToTray = m.cfgChecks["UI.MinimizeToTray"].Checked
	cfg.UI.CloseToTray = m.cfgChecks["UI.CloseToTray"].Checked
	cfg.Logging.EnableConsole = m.cfgChecks["Logging.EnableConsole"].Checked
	cfg.Logging.EnableFile = m.cfgChecks["Logging.EnableFile"].Checked
	cfg.Profiling.Enabled = m.cfgChecks["Profiling.Enabled"].Checked
	config.Normalize(&cfg)
	return cfg, nil
}
