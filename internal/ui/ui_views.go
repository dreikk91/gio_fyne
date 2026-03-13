package ui

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	appLog "cid_gio_gio/internal/logger"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

func (m *model) buildMainUI() {
	m.headerTitle = canvas.NewText("CID Retranslator", cText)
	m.headerTitle.TextSize = 18
	m.headerTitle.TextStyle = fyne.TextStyle{Bold: true}
	m.headerSubtitle = canvas.NewText("", cSoft)
	m.headerSubtitle.TextSize = 12

	headerLeft := container.NewVBox(m.headerTitle, m.headerSubtitle)

	m.chipStatus = newChip("Status", "Offline", cBad, cBadSoft)
	m.chipUptime = newChip("Uptime", "-", cText, cPanel2)
	m.chipClients = newChip("Clients", "0", cText, cPanel2)
	m.chipAccepted = newChip("Accepted", "0", cText, cPanel2)
	m.chipRejected = newChip("Rejected", "0", cText, cPanel2)
	headerRight := container.NewHBox(
		m.chipStatus.box(),
		layout.NewSpacer(),
		m.chipUptime.box(),
		m.chipClients.box(),
		m.chipAccepted.box(),
		m.chipRejected.box(),
	)

	headerRow := container.NewBorder(nil, nil, headerLeft, headerRight, layout.NewSpacer())

	m.statusBanner = newStatusBanner()

	tabs := container.NewAppTabs(
		container.NewTabItem("Objects", m.buildObjectsTab()),
		container.NewTabItem("Events", m.buildEventsTab()),
		container.NewTabItem("Settings", m.buildSettingsTab()),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	top := container.NewVBox(
		cardContainer(cPanel, headerRow),
		m.statusBanner.box,
	)
	root := container.NewBorder(top, nil, nil, nil, tabs)
	m.win.SetContent(root)
	m.win.Resize(fyne.NewSize(980, 700))
	m.refreshMainUI()
}

func (m *model) buildObjectsTab() fyne.CanvasObject {
	m.objMetricTotal = newMetricCard("Total objects", "0", cPanel2, cText)
	m.objMetricVisible = newMetricCard("Visible", "0", cAccent2, cAccent)
	m.objMetricActive = newMetricCard("Active", "0", cGoodSoft, cGood)
	m.objMetricInactive = newMetricCard("Inactive", "0", cBadSoft, cBad)
	metricsRow := container.NewGridWithColumns(4,
		m.objMetricTotal.box(),
		m.objMetricVisible.box(),
		m.objMetricActive.box(),
		m.objMetricInactive.box(),
	)

	m.objSearchEntry = widget.NewEntry()
	m.objSearchEntry.SetPlaceHolder("Search objects by ID, client or last event...")
	m.objSearchEntry.OnChanged = func(s string) {
		m.deviceFilter = strings.TrimSpace(s)
		m.applyFilters()
	}

	m.objShowingChip = newChip("Showing", "0 / 0", cText, cPanel2)
	toolbar := container.NewBorder(nil, nil, nil, m.objShowingChip.box(),
		cardContainer(cPanel2, m.objSearchEntry),
	)

	headers := []string{"State", "PPK", "Client", "Last Event", "Date/Time", "Actions"}
	m.objTable = widget.NewTable(
		func() (int, int) {
			m.mu.RLock()
			defer m.mu.RUnlock()
			return len(m.filteredDevices) + 1, len(headers)
		},
		func() fyne.CanvasObject {
			return newTableActionCell()
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			bg, lbl, btns, btnHist, btnDel := getActionCellParts(obj)
			if id.Row == 0 {
				lbl.SetText(headers[id.Col])
				lbl.TextStyle = fyne.TextStyle{Bold: true}
				lbl.Show()
				btns.Hide()
				bg.FillColor = cPanel3
				bg.Refresh()
				return
			}
			dataRow := id.Row - 1
			m.mu.RLock()
			if dataRow >= len(m.filteredDevices) {
				m.mu.RUnlock()
				return
			}
			d := m.filteredDevices[dataRow]
			m.mu.RUnlock()
			stale := isStale(d.LastEventTime, m.activityTO)

			rowBg := rowAltColor(dataRow)
			eventCat := m.getEventCategoryForDevice(d.ID)
			if !stale && eventCat != "" {
				rowBg = eventColor(eventCat, dataRow)
			}

			if stale {
				rowBg = cBadSoft
			}
			if m.selObjRow == id.Row {
				rowBg = cAccentSoft
			}
			ts := "-"
			if !d.LastEventTime.IsZero() {
				ts = d.LastEventTime.Format("2006-01-02 15:04:05")
			}
			values := []string{
				boolText(stale, "Inactive", "Active"),
				fmt.Sprintf("%03d", d.ID),
				firstNonEmpty(d.ClientAddr, "-"),
				d.LastEvent,
				ts,
				"",
			}
			if id.Col == 5 {
				lbl.Hide()
				btns.Show()
				btnHist.OnTapped = func() { m.openHistory(d) }
				btnDel.OnTapped = func() { m.openDeleteDialog(d) }
				bg.FillColor = rowBg
				bg.Refresh()
				btns.Refresh()
				return
			}
			btns.Hide()
			lbl.Show()
			lbl.TextStyle = fyne.TextStyle{}
			lbl.SetText(values[id.Col])
			bg.FillColor = rowBg
			bg.Refresh()
		},
	)
	m.objTable.SetColumnWidth(0, 90)
	m.objTable.SetColumnWidth(1, 70)
	m.objTable.SetColumnWidth(2, 180)
	m.objTable.SetColumnWidth(3, 220)
	m.objTable.SetColumnWidth(4, 160)
	m.objTable.SetColumnWidth(5, 160)
	m.objTable.OnSelected = func(id widget.TableCellID) {
		if id.Row <= 0 {
			return
		}
		m.selObjRow = id.Row
		dataRow := id.Row - 1
		m.mu.RLock()
		if dataRow < len(m.filteredDevices) {
			d := m.filteredDevices[dataRow]
			m.mu.RUnlock()
			m.openHistory(d)
		} else {
			m.mu.RUnlock()
		}
		m.objTable.Refresh()
	}
	m.objTable.OnUnselected = func(id widget.TableCellID) {
		if m.selObjRow == id.Row {
			m.selObjRow = -1
		}
		m.objTable.Refresh()
	}

	listWrap := cardContainer(cPanel, m.objTable)

	top := container.NewVBox(
		metricsRow,
		vGap(6),
		toolbar,
		vGap(6),
	)
	return container.NewBorder(top, nil, nil, nil, listWrap)
}

func (m *model) buildEventsTab() fyne.CanvasObject {
	m.evtMetricVisible = newMetricCard("Visible", "0", cPanel2, cText)
	m.evtMetricLoaded = newMetricCard("Loaded", "0", cAccent2, cAccent)
	m.evtMetricFilter = newMetricCard("Filter", "ALL", cAccent2, cAccent)
	m.evtMetricRate = newMetricCard("Msg/min", "0", cPanel2, cAccent)

	metricsRow := container.NewGridWithColumns(4,
		m.evtMetricVisible.box(),
		m.evtMetricLoaded.box(),
		m.evtMetricFilter.box(),
		m.evtMetricRate.box(),
	)

	filterRow := container.NewHBox()
	m.eventFilterBtns = map[string]*widget.Button{}
	for _, f := range eventFilters {
		ff := f
		btn := widget.NewButton(strings.ToUpper(f), func() {
			m.eventFilter = ff
			m.applyFilters()
			m.refreshEventFilterButtons()
		})
		btn.Importance = widget.LowImportance
		m.eventFilterBtns[f] = btn
		filterRow.Add(container.NewGridWrap(fyne.NewSize(78, 28), btn))
	}

	m.evtSearchEntry = widget.NewEntry()
	m.evtSearchEntry.SetPlaceHolder("Search events by code, description, zone...")
	m.evtSearchEntry.OnChanged = func(s string) {
		m.eventQuery = strings.TrimSpace(s)
		m.applyFilters()
	}

	m.hideTestsCheck = widget.NewCheck("Hide tests", func(v bool) {
		m.hideTests = v
		m.applyFilters()
	})
	m.hideBlockedCheck = widget.NewCheck("Only non-blocked", func(v bool) {
		m.hideBlocked = v
		m.applyFilters()
	})

	toolbar := container.NewBorder(nil, nil,
		filterRow,
		container.NewHBox(m.hideTestsCheck, m.hideBlockedCheck),
		cardContainer(cPanel2, m.evtSearchEntry),
	)

	headers := []string{"Time", "PPK", "Code", "Type", "Description", "Zone", "Relay"}
	m.evtTable = widget.NewTable(
		func() (int, int) {
			m.mu.RLock()
			defer m.mu.RUnlock()
			return len(m.filteredEvents) + 1, len(headers)
		},
		func() fyne.CanvasObject {
			return newEventCell()
		},
		func(id widget.TableCellID, obj fyne.CanvasObject) {
			bg, lbl := getEventCellParts(obj)
			if id.Row == 0 {
				lbl.SetText(headers[id.Col])
				lbl.TextStyle = fyne.TextStyle{Bold: true}
				bg.FillColor = cPanel3
				bg.Refresh()
				return
			}
			lbl.TextStyle = fyne.TextStyle{}
			dataRow := id.Row - 1
			m.mu.RLock()
			if dataRow >= len(m.filteredEvents) {
				m.mu.RUnlock()
				return
			}
			e := m.filteredEvents[dataRow]
			m.mu.RUnlock()
			relay := "OK"
			if e.RelayBlocked {
				relay = "Blocked"
			}
			values := []string{
				e.Time.Format("2006-01-02 15:04:05"),
				e.DeviceID,
				e.Code,
				e.Type,
				e.Desc,
				e.Zone,
				relay,
			}
			lbl.TextStyle = fyne.TextStyle{}
			lbl.SetText(values[id.Col])
			bg.FillColor = eventColor(e.Category, dataRow)
			bg.Refresh()
		},
	)
	m.evtTable.SetColumnWidth(0, 150)
	m.evtTable.SetColumnWidth(1, 70)
	m.evtTable.SetColumnWidth(2, 70)
	m.evtTable.SetColumnWidth(3, 120)
	m.evtTable.SetColumnWidth(4, 280)
	m.evtTable.SetColumnWidth(5, 60)
	m.evtTable.SetColumnWidth(6, 80)
	m.evtTable.OnSelected = func(id widget.TableCellID) {
		if id.Row <= 0 {
			return
		}
		m.selEvtRow = id.Row
		m.evtTable.Refresh()
	}
	m.evtTable.OnUnselected = func(id widget.TableCellID) {
		if m.selEvtRow == id.Row {
			m.selEvtRow = -1
		}
		m.evtTable.Refresh()
	}

	loadMoreBtn := widget.NewButton("Load more", func() { m.loadMoreEvents() })
	loadMoreBtn.Importance = widget.LowImportance

	top := container.NewVBox(
		metricsRow,
		vGap(6),
		toolbar,
		vGap(6),
	)
	center := cardContainer(cPanel, m.evtTable)
	m.refreshEventFilterButtons()
	return container.NewBorder(top, loadMoreBtn, nil, nil, center)
}

func (m *model) buildSettingsTab() fyne.CanvasObject {
	m.logLevelSelect = widget.NewSelect(logLevels, func(v string) {
		m.cfgEntries["Logging.Level"].SetText(v)
		m.cfg.Logging.Level = v
		if err := appLog.SetLevel(v); err != nil {
			m.statusErr = "Invalid log level: " + v
		} else {
			m.statusErr = ""
			m.statusMsg = "Log level applied: " + strings.ToUpper(v)
		}
		m.refreshMainUI()
	})
	m.fontSizeSlider = widget.NewSlider(7, 30)
	m.fontSizeSlider.Step = 1
	m.fontSizeSlider.OnChanged = func(v float64) {
		size := clamp(int(v+0.5), 7, 30)
		m.cfgEntries["UI.FontSize"].SetText(strconv.Itoa(size))
		m.applyFontSize(size)
		m.fontSizeLabel.Text = fmt.Sprintf("Current font size: %d", size)
		m.fontSizeLabel.Refresh()
	}
	m.fontSizeLabel = canvas.NewText("Current font size: 12", cSoft)
	m.fontSizeLabel.TextSize = 11

	network := newSettingsSection("Network & Queue", container.NewVBox(
		m.fieldRow("Server host", "Server.Host"),
		m.fieldRow("Server port", "Server.Port"),
		m.fieldRow("Client host", "Client.Host"),
		m.fieldRow("Client port", "Client.Port"),
		m.fieldRow("Queue buffer", "Queue.BufferSize"),
		m.fieldRow("PPK timeout", "Monitoring.PpkTimeout"),
	))

	history := newSettingsSection("History & Interface", container.NewVBox(
		m.fieldRow("History global", "History.GlobalLimit"),
		m.fieldRow("History log", "History.LogLimit"),
		m.fontSliderRow(),
		m.flagRow("UI.StartMinimized", "Start minimized"),
		m.flagRow("UI.MinimizeToTray", "Minimize to tray"),
		m.flagRow("UI.CloseToTray", "Close to tray"),
	))

	rules := newSettingsSection("CID Rules", container.NewVBox(
		m.fieldRow("Required prefix", "CidRules.RequiredPrefix"),
		m.fieldRow("Valid length", "CidRules.ValidLength"),
		m.fieldRow("Default acc add", "CidRules.AccNumAdd"),
		m.multiLineRow("Account ranges (From-To:Delta)", "CidRules.AccountRanges", 120),
	))
	m.rfOpenBtn = widget.NewButton("Configure Relay Filter", func() { m.openRelayFilter() })

	logging := newSettingsSection("Logging & Actions", container.NewVBox(
		m.logLevelRow(),
		m.flagRow("Logging.EnableConsole", "Enable console logs"),
		m.flagRow("Logging.EnableFile", "Enable file logs"),
		m.fontSizeLabel,
		container.NewHBox(
			widget.NewButton("Save", func() { m.onSaveConfig() }),
			widget.NewButton("Reset", func() { m.onResetConfig() }),
		),
	))

	left := container.NewVBox(network, history)
	right := container.NewVBox(rules, m.rfOpenBtn, logging)

	grid := container.NewGridWithColumns(2, left, right)
	m.loadCfgEditors(m.cfg)
	return grid
}

func (m *model) onSaveConfig() {
	cfg, err := m.collectCfg()
	if err != nil {
		m.statusErr = "Save failed: " + err.Error()
		m.refreshMainUI()
		return
	}
	go m.saveConfigRemote(cfg)
}

func (m *model) onResetConfig() {
	m.applyFontSize(m.cfg.UI.FontSize)
	m.loadCfgEditors(m.cfg)
	m.refreshMainUI()
}

func (m *model) refreshMainUI() {
	m.runOnUI(func() {
		m.updateHeaderStats()
		m.updateStatusBanner()
		if m.objTable != nil {
			m.objTable.Refresh()
		}
		if m.evtTable != nil {
			m.evtTable.Refresh()
		}
		m.refreshMetrics()
	})
}

func (m *model) refreshHistoryUI() {
	m.runOnUI(func() {
		m.mu.RLock()
		count := len(m.hRows)
		m.mu.RUnlock()
		if m.hHeaderTitle != nil {
			m.hHeaderTitle.Text = fmt.Sprintf("Device Event Journal - %03d", m.hDevice.ID)
			m.hHeaderTitle.Refresh()
		}
		if m.hHeaderSubtitle != nil {
			m.hHeaderSubtitle.Text = fmt.Sprintf("Records: %d", count)
			m.hHeaderSubtitle.Refresh()
		}
		if m.hTable != nil {
			m.hTable.Refresh()
		}
	})
}

func (m *model) refreshRelayUI() {
	m.runOnUI(func() {
		if m.rfObjList != nil {
			m.rfObjList.Refresh()
		}
		if m.rfCodeList != nil {
			m.rfCodeList.Refresh()
		}
		if m.rfSumList != nil {
			m.rfSumList.Refresh()
		}
		if m.rfStatusLabel != nil {
			m.rfStatusLabel.SetText(m.rfStatusDesc())
		}
	})
}

func (m *model) updateHeaderStats() {
	if m.headerSubtitle == nil {
		return
	}
	m.mu.RLock()
	total := len(m.devices)
	visible := len(m.filteredDevices)
	eventsLoaded := len(m.events)
	m.mu.RUnlock()
	m.headerSubtitle.Text = fmt.Sprintf("Objects: %d (visible %d) | Events loaded: %d", total, visible, eventsLoaded)
	m.headerSubtitle.Refresh()

	m.mu.RLock()
	stats := m.stats
	m.mu.RUnlock()
	if m.chipStatus != nil {
		status := boolText(stats.Connected, "Online", "Offline")
		bg := firstColor(stats.Connected, cGoodSoft, cBadSoft)
		fg := firstColor(stats.Connected, cGood, cBad)
		m.chipStatus.set(status, fg, bg)
	}
	if m.chipUptime != nil {
		m.chipUptime.set(stats.Uptime, cText, cPanel2)
	}
	if m.chipClients != nil {
		m.chipClients.set(strconv.Itoa(stats.Clients), cText, cPanel2)
	}
	if m.chipAccepted != nil {
		m.chipAccepted.set(strconv.FormatInt(stats.Accepted, 10), cText, cPanel2)
	}
	if m.chipRejected != nil {
		m.chipRejected.set(strconv.FormatInt(stats.Rejected, 10), cText, cPanel2)
	}
}

func (m *model) refreshMetrics() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.objMetricTotal != nil {
		m.objMetricTotal.set(strconv.Itoa(len(m.devices)), cPanel2, cText)
	}
	if m.objMetricVisible != nil {
		m.objMetricVisible.set(strconv.Itoa(len(m.filteredDevices)), cAccent2, cAccent)
	}
	if m.objMetricActive != nil {
		m.objMetricActive.set(strconv.Itoa(m.activeDevices), cGoodSoft, cGood)
	}
	if m.objMetricInactive != nil {
		m.objMetricInactive.set(strconv.Itoa(m.inactiveDevices), cBadSoft, cBad)
	}
	if m.objShowingChip != nil {
		m.objShowingChip.set(fmt.Sprintf("%d / %d", len(m.filteredDevices), len(m.devices)), cText, cPanel2)
	}

	if m.evtMetricVisible != nil {
		m.evtMetricVisible.set(strconv.Itoa(len(m.filteredEvents)), cPanel2, cText)
	}
	if m.evtMetricLoaded != nil {
		m.evtMetricLoaded.set(strconv.Itoa(len(m.events)), cAccent2, cAccent)
	}
	if m.evtMetricFilter != nil {
		bg, fg := filterTone(m.eventFilter)
		m.evtMetricFilter.set(strings.ToUpper(m.eventFilter), bg, fg)
	}
	if m.evtMetricRate != nil {
		m.evtMetricRate.set(strconv.FormatInt(m.stats.ReceivedPS*60, 10), cPanel2, cAccent)
	}
}

func (m *model) refreshEventFilterButtons() {
	for f, btn := range m.eventFilterBtns {
		if f == m.eventFilter {
			btn.Importance = widget.MediumImportance
		} else {
			btn.Importance = widget.LowImportance
		}
		btn.Refresh()
	}
}

func newSettingsSection(title string, body fyne.CanvasObject) fyne.CanvasObject {
	titleText := canvas.NewText(title, cText)
	titleText.TextStyle = fyne.TextStyle{Bold: true}
	titleText.TextSize = 14
	return cardContainer(cPanel, container.NewVBox(titleText, body))
}

func (m *model) fieldRow(label, key string) fyne.CanvasObject {
	entry := m.cfgEntries[key]
	if entry == nil {
		entry = widget.NewEntry()
		m.cfgEntries[key] = entry
	}
	lbl := canvas.NewText(label, cSoft)
	lbl.TextSize = 12
	row := container.NewBorder(nil, nil, lbl, nil, entry)
	return row
}

func (m *model) multiLineRow(label, key string, height int) fyne.CanvasObject {
	entry := m.cfgEntries[key]
	if entry == nil {
		entry = widget.NewMultiLineEntry()
		m.cfgEntries[key] = entry
	}
	entry.SetMinRowsVisible(4)
	lbl := canvas.NewText(label, cSoft)
	lbl.TextSize = 12
	return container.NewVBox(lbl, entry)
}

func (m *model) flagRow(key, label string) fyne.CanvasObject {
	chk := m.cfgChecks[key]
	if chk == nil {
		chk = widget.NewCheck(label, nil)
		m.cfgChecks[key] = chk
	} else {
		chk.SetText(label)
	}
	return chk
}

func (m *model) logLevelRow() fyne.CanvasObject {
	lbl := canvas.NewText("Log level", cSoft)
	lbl.TextSize = 12
	return container.NewBorder(nil, nil, lbl, nil, m.logLevelSelect)
}

func (m *model) fontSliderRow() fyne.CanvasObject {
	lbl := canvas.NewText("Font size", cSoft)
	lbl.TextSize = 12
	return container.NewVBox(lbl, m.fontSizeSlider)
}

func (m *model) applyFontSize(size int) {
	size = clamp(size, 7, 30)
	m.theme.SetFontSize(size)
	m.app.Settings().SetTheme(m.theme)
	if m.fontSizeSlider != nil {
		m.fontSizeSlider.SetValue(float64(size))
	}
}

func filterTone(filter string) (color.NRGBA, color.NRGBA) {
	switch strings.ToLower(strings.TrimSpace(filter)) {
	case "alarm":
		return cBadSoft, cBad
	case "test":
		return cWarnSoft, cWarn
	case "fault":
		return color.NRGBA{R: 255, G: 238, B: 214, A: 255}, color.NRGBA{R: 168, G: 95, B: 0, A: 255}
	case "guard":
		return cGoodSoft, cGood
	case "disguard":
		return cAccentSoft, cAccent
	case "other":
		return cPanel3, cSoft
	default:
		return cAccent2, cAccent
	}
}
