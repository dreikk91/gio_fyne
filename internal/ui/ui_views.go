package ui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	appLog "github.com/dreikk91/gio_fyne/internal/logger"

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
	m.chipRate = newChip("Msg/min", "0", cAccent, cAccentSoft)
	headerRight := container.NewHBox(
		m.chipStatus.box(),
		layout.NewSpacer(),
		m.chipUptime.box(),
		m.chipClients.box(),
		m.chipAccepted.box(),
		m.chipRejected.box(),
		m.chipRate.box(),
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
	m.win.Resize(fyne.NewSize(1024, 720))
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
	metricsRow = container.NewPadded(metricsRow)

	m.objSearchEntry = widget.NewEntry()
	m.objSearchEntry.SetPlaceHolder("Search objects by ID, client or last event...")
	m.objSearchEntry.OnChanged = func(s string) {
		m.deviceFilter = strings.TrimSpace(s)
		m.applyDeviceFilters()
	}

	m.objShowingChip = newChip("Showing", "0 / 0", cText, cPanel2)
	toolbar := container.NewBorder(nil, nil, nil, m.objShowingChip.box(),
		m.objSearchEntry,
	)

	headers := []string{"State", "PPK", "Client", "Last Event", "Date/Time", "Actions"}
	m.objTable = m.newObjectsTable(headers)
	m.objTable.OnSelected = func(id widget.TableCellID) {
		if id.Row <= 0 {
			return
		}
		dataRow := id.Row - 1
		m.mu.RLock()
		idx := m.objStart + dataRow
		if idx >= 0 && idx < len(m.filteredDevices) {
			d := m.filteredDevices[idx]
			m.mu.RUnlock()
			m.selObjRow = idx
			m.openHistory(d)
		} else {
			m.mu.RUnlock()
		}
		m.objTable.Refresh()
	}
	m.objTable.OnUnselected = func(id widget.TableCellID) {
		idx := m.objStart + (id.Row - 1)
		if m.selObjRow == idx {
			m.selObjRow = -1
		}
		m.objTable.Refresh()
	}

	m.objScroll = container.NewScroll(m.objTable)
	m.objScroll.OnScrolled = func(pos fyne.Position) {
		m.onObjScrolled(pos.Y)
	}
	listWrap := cardContainer(cPanel, container.New(&objectTableLayout{table: m.objTable}, m.objScroll))

	top := container.NewVBox(
		metricsRow,
		vGap(6),
		toolbar,
		vGap(6),
	)
	return container.NewBorder(top, nil, nil, nil, listWrap)
}

type objectTableLayout struct {
	table           *widget.Table
	lastClientWidth float32
	lastEventWidth  float32
}

type refreshOnResizeLayout struct {
	onResize func(size fyne.Size)
	lastSize fyne.Size
}

func (l *refreshOnResizeLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Resize(size)
		o.Move(fyne.NewPos(0, 0))
	}
	if l.onResize != nil && (l.lastSize.Width != size.Width || l.lastSize.Height != size.Height) {
		l.lastSize = size
		l.onResize(size)
	}
}

func (l *refreshOnResizeLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.NewSize(0, 0)
	}
	return objects[0].MinSize()
}

func (l *objectTableLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if l.table != nil {
		widths := fitObjectColumnWidths(size.Width)
		clientWidth := widths[2]
		eventWidth := widths[3]
		if l.lastClientWidth != clientWidth {
			l.table.SetColumnWidth(2, clientWidth)
			l.lastClientWidth = clientWidth
		}
		if l.lastEventWidth != eventWidth {
			l.table.SetColumnWidth(3, eventWidth)
			l.lastEventWidth = eventWidth
		}
		l.table.SetColumnWidth(0, widths[0])
		l.table.SetColumnWidth(1, widths[1])
		l.table.SetColumnWidth(4, widths[4])
		l.table.SetColumnWidth(5, widths[5])
	}
	for _, o := range objects {
		o.Resize(size)
		o.Move(fyne.NewPos(0, 0))
	}
}

func (l *objectTableLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(600, 300)
}

func fitObjectColumnWidths(totalWidth float32) [6]float32 {
	minW := [6]float32{74, 58, 130, 190, 136, 126}
	base := [6]float32{86, 64, 180, 360, 160, 150}

	available := totalWidth - 12
	if available < 480 {
		available = 480
	}

	var baseSum float32
	for _, w := range base {
		baseSum += w
	}
	var minSum float32
	for _, w := range minW {
		minSum += w
	}
	if available <= minSum {
		return minW
	}
	if available >= baseSum {
		extra := available - baseSum
		base[2] += extra * 0.35
		base[3] += extra * 0.65
		return base
	}

	needShrink := baseSum - available
	shrinkCap := [6]float32{}
	var capSum float32
	for i := range base {
		shrinkCap[i] = base[i] - minW[i]
		if shrinkCap[i] > 0 {
			capSum += shrinkCap[i]
		}
	}
	if capSum <= 0 {
		return minW
	}
	for i := range base {
		if shrinkCap[i] <= 0 {
			continue
		}
		delta := needShrink * (shrinkCap[i] / capSum)
		base[i] -= delta
		if base[i] < minW[i] {
			base[i] = minW[i]
		}
	}
	return base
}

func (m *model) buildEventsTab() fyne.CanvasObject {
	metricsRow := container.NewGridWithColumns(4,
		m.objMetricTotal.box(),
		m.objMetricVisible.box(),
		m.objMetricActive.box(),
		m.objMetricInactive.box(),
	)
	metricsRow = container.NewPadded(metricsRow)

	filterRow := container.NewHBox()
	m.eventFilterBtns = map[string]*filterButtonRef{}
	for i, f := range eventFilters {
		ff := f
		btn := newFilterButton(strings.ToUpper(f), func() {
			m.eventFilter = ff
			m.reloadEventsWithReset()
			m.refreshEventFilterButtons()
		})
		m.eventFilterBtns[f] = btn
		if i > 0 {
			filterRow.Add(hGap(4))
		}
		filterRow.Add(container.NewGridWrap(filterButtonSize(60), container.NewCenter(btn.box())))
	}

	m.evtSearchEntry = widget.NewEntry()
	m.evtSearchEntry.SetPlaceHolder("Search events by code, description, zone...")
	m.evtSearchEntry.OnChanged = func(s string) {
		m.eventQuery = strings.TrimSpace(s)
		m.reloadEventsWithReset()
	}

	m.hideTestsCheck = widget.NewCheck("Hide tests", func(v bool) {
		m.hideTests = v
		m.reloadEventsWithReset()
	})
	m.hideBlockedCheck = widget.NewCheck("Only non-blocked", func(v bool) {
		m.hideBlocked = v
		m.reloadEventsWithReset()
	})

	rowHeight := filterButtonSize(60).Height
	filterWidth := filterRow.MinSize().Width
	if filterWidth < 320 {
		filterWidth = 320
	}
	filterWrap := container.NewGridWrap(fyne.NewSize(filterWidth, rowHeight), container.NewCenter(filterRow))

	m.evtSearchEntry.SetPlaceHolder("Search events...")

	toolbar := container.NewBorder(nil, nil, filterWrap, container.NewHBox(m.hideTestsCheck, m.hideBlockedCheck),
		m.evtSearchEntry,
	)

	headers := []string{"Time", "PPK", "Code", "Type", "Description", "Zone", "Relay"}
	m.evtList = m.newEventsList()
	m.evtList.OnSelected = func(id widget.ListItemID) {
		m.selEvtRow = m.evtStart + int(id)
		if m.evtList != nil {
			m.evtList.Unselect(id)
		}
	}

	top := container.NewVBox(
		vGap(2),
		toolbar,
		vGap(2),
	)
	m.evtScroll = container.NewScroll(m.evtList)
	m.evtScroll.OnScrolled = func(pos fyne.Position) {
		m.onEvtScrolled(pos.Y)
	}
	centerContent := container.NewBorder(newTableHeader(headers), nil, nil, nil, m.evtScroll)
	responsiveCenter := container.New(&refreshOnResizeLayout{
		onResize: func(_ fyne.Size) {
			if m.evtList != nil {
				m.evtList.Refresh()
			}
		},
	}, centerContent)
	center := cardContainer(cPanel, responsiveCenter)
	m.refreshEventFilterButtons()
	return container.NewBorder(top, nil, nil, nil, center)
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
	m.colorsOpenBtn = widget.NewButton("Event Colors Customization", func() { m.openColorSettings() })

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

	pprofOpenBtn := widget.NewButton("Open pprof", func() {
		url := m.pprofURL()
		if url == "" {
			m.statusErr = "pprof URL is empty"
			m.refreshMainUI()
			return
		}
		if err := openURL(url); err != nil {
			m.statusErr = "Failed to open pprof: " + err.Error()
		} else {
			m.statusErr = ""
			m.statusMsg = "Opened pprof in browser"
		}
		m.refreshMainUI()
	})
	pprofCopyBtn := widget.NewButton("Copy URL", func() {
		url := m.pprofURL()
		if url == "" {
			m.statusErr = "pprof URL is empty"
			m.refreshMainUI()
			return
		}
		if m.win != nil {
			m.win.Clipboard().SetContent(url)
		}
		m.statusErr = ""
		m.statusMsg = "pprof URL copied"
		m.refreshMainUI()
	})

	profiling := newSettingsSection("Profiling (pprof)", container.NewVBox(
		m.flagRow("Profiling.Enabled", "Enable pprof server"),
		m.fieldRow("pprof host", "Profiling.Host"),
		m.fieldRow("pprof port", "Profiling.Port"),
		container.NewHBox(pprofOpenBtn, pprofCopyBtn),
	))

	pprofHeapBtn := widget.NewButton("Heap graph", func() { m.openPprofWeb("heap") })
	pprofCPUBtn := widget.NewButton("CPU graph (30s)", func() { m.openPprofWeb("profile?seconds=30") })
	pprofGorBtn := widget.NewButton("Goroutines", func() { m.openPprofWeb("goroutine") })
	pprofAllocsBtn := widget.NewButton("Allocs", func() { m.openPprofWeb("allocs") })
	profilingTools := newSettingsSection("Profiling Views", container.NewVBox(
		widget.NewLabel("Opens pprof web UI in your browser (requires Go installed)."),
		container.NewHBox(pprofHeapBtn, pprofCPUBtn),
		container.NewHBox(pprofGorBtn, pprofAllocsBtn),
	))

	left := container.NewVBox(network, history, profiling, profilingTools)
	right := container.NewVBox(rules, container.NewHBox(m.rfOpenBtn, m.colorsOpenBtn), logging)

	grid := container.NewGridWithColumns(2, left, right)
	m.loadCfgEditors(m.cfg)
	scroll := container.NewScroll(grid)
	scroll.SetMinSize(fyne.NewSize(900, 600))
	return scroll
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
		if m.evtList != nil {
			m.evtList.Refresh()
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
		if m.hList != nil {
			m.hList.Refresh()
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
	if m.chipRate != nil {
		m.chipRate.set(strconv.FormatInt(stats.ReceivedPM, 10), cAccent, cAccentSoft)
	}
}

func (m *model) refreshMetrics() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.objMetricTotal != nil {
		m.objMetricTotal.set(strconv.Itoa(len(m.devices)), cPanel2, cText)
	}
	if m.objShowingChip != nil {
		m.objShowingChip.set(fmt.Sprintf("%d / %d", len(m.filteredDevices), len(m.devices)), cText, cPanel2)
	}
}
func (m *model) refreshEventFilterButtons() {
	for f, btn := range m.eventFilterBtns {
		btn.set(f == m.eventFilter, f)
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

func (m *model) pprofURL() string {
	host := strings.TrimSpace(m.cfgEntries["Profiling.Host"].Text)
	if host == "" {
		host = "127.0.0.1"
	}
	port := strings.TrimSpace(m.cfgEntries["Profiling.Port"].Text)
	if port == "" {
		port = "6060"
	}
	return "http://" + host + ":" + port + "/debug/pprof/"
}

func openURL(url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

func (m *model) openPprofWeb(path string) {
	base := m.pprofURL()
	if base == "" {
		m.statusErr = "pprof URL is empty"
		m.refreshMainUI()
		return
	}
	target := base + strings.TrimPrefix(path, "/")
	viewAddr := "127.0.0.1:6061"
	viewURL := "http://" + viewAddr + "/"
	cmd := exec.Command("go", "tool", "pprof", "-http="+viewAddr, target)
	if err := cmd.Start(); err != nil {
		m.statusErr = "Failed to start pprof web UI: " + err.Error()
		m.refreshMainUI()
		return
	}
	m.statusErr = ""
	m.statusMsg = "pprof web UI started at " + viewURL
	_ = openURL(viewURL)
	m.refreshMainUI()
}

