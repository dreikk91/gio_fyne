package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func (m *model) openHistoryWindow() {
	if m.hWin == nil {
		m.hWin = m.app.NewWindow("Device History")
		m.buildHistoryUI()
		m.hWin.SetCloseIntercept(func() {
			m.hOpen = false
			m.historyBusy.Store(false)
			m.hWin.Hide()
		})
	}
	m.hWin.Show()
}

func (m *model) buildHistoryUI() {
	m.hHeaderTitle = canvas.NewText("", cText)
	m.hHeaderTitle.TextSize = 16
	m.hHeaderTitle.TextStyle = fyne.TextStyle{Bold: true}
	m.hHeaderSubtitle = canvas.NewText("", cSoft)
	m.hHeaderSubtitle.TextSize = 11

	header := cardContainer(cModalH, container.NewVBox(m.hHeaderTitle, m.hHeaderSubtitle))

	filterRow := container.NewHBox()
	m.hFilterBtns = map[string]*widget.Button{}
	for _, f := range eventFilters {
		ff := f
		btn := widget.NewButton(strings.ToUpper(f), func() {
			m.hEventType = ff
			m.hLimit = m.initialDeviceHistoryLimit()
			m.requestHistoryReload()
			m.refreshHistoryFilters()
		})
		btn.Importance = widget.LowImportance
		m.hFilterBtns[f] = btn
		filterRow.Add(container.NewGridWrap(fyne.NewSize(78, 28), btn))
	}

	m.hSearchEntry = widget.NewEntry()
	m.hSearchEntry.SetPlaceHolder("History search...")
	m.hSearchEntry.OnChanged = func(s string) {
		q := strings.TrimSpace(s)
		if q != m.hQueryCache {
			m.hQueryCache = q
			m.hLimit = m.initialDeviceHistoryLimit()
			m.requestHistoryReload()
		}
	}
	m.hHideTestsCheck = widget.NewCheck("Hide tests", func(bool) {
		m.hLimit = m.initialDeviceHistoryLimit()
		m.requestHistoryReload()
	})
	m.hHideBlockedCheck = widget.NewCheck("Only non-blocked", func(bool) {
		m.hLimit = m.initialDeviceHistoryLimit()
		m.requestHistoryReload()
	})

	toolbar := container.NewBorder(nil, nil, nil, container.NewHBox(m.hHideTestsCheck, m.hHideBlockedCheck),
		cardContainer(cPanel2, m.hSearchEntry),
	)

	headers := []string{"Time", "PPK", "Code", "Type", "Description", "Zone", "Relay"}
	m.hTable = widget.NewTable(
		func() (int, int) {
			m.mu.RLock()
			defer m.mu.RUnlock()
			return len(m.hRows) + 1, len(headers)
		},
		func() fyne.CanvasObject { return newEventCell() },
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
			if dataRow >= len(m.hRows) {
				m.mu.RUnlock()
				return
			}
			e := m.hRows[dataRow]
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
			
			lbl.SetText(values[id.Col])
			bg.FillColor = eventColor(e.Category, dataRow)
			bg.Refresh()
		},
	)
	m.hTable.SetColumnWidth(0, 150)
	m.hTable.SetColumnWidth(1, 70)
	m.hTable.SetColumnWidth(2, 70)
	m.hTable.SetColumnWidth(3, 120)
	m.hTable.SetColumnWidth(4, 280)
	m.hTable.SetColumnWidth(5, 60)
	m.hTable.SetColumnWidth(6, 80)
	m.hTable.OnSelected = func(id widget.TableCellID) {
		if id.Row <= 0 {
			return
		}
		m.selHistRow = id.Row
		m.hTable.Refresh()
	}
	m.hTable.OnUnselected = func(id widget.TableCellID) {
		if m.selHistRow == id.Row {
			m.selHistRow = -1
		}
		m.hTable.Refresh()
	}

	loadMoreBtn := widget.NewButton("Load more", func() { m.loadMoreHistory() })
	loadMoreBtn.Importance = widget.LowImportance

	top := container.NewVBox(
		header,
		filterRow,
		vGap(6),
		toolbar,
		vGap(6),
	)
	center := cardContainer(cModal, m.hTable)
	content := container.NewBorder(top, loadMoreBtn, nil, nil, center)
	m.hWin.SetContent(content)
	m.hWin.Resize(fyne.NewSize(980, 640))
	m.refreshHistoryFilters()
	m.refreshHistoryUI()
}

func (m *model) refreshHistoryFilters() {
	for f, btn := range m.hFilterBtns {
		if f == m.hEventType {
			btn.Importance = widget.MediumImportance
		} else {
			btn.Importance = widget.LowImportance
		}
		btn.Refresh()
	}
}
