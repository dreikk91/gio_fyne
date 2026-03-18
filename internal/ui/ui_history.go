package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
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
	m.hFilterBtns = map[string]*filterButtonRef{}
	for i, f := range eventFilters {
		ff := f
		btn := newFilterButton(strings.ToUpper(f), func() {
			m.hEventType = ff
			m.hLimit = m.initialDeviceHistoryLimit()
			m.requestHistoryReload()
			m.refreshHistoryFilters()
		})
		m.hFilterBtns[f] = btn
		if i > 0 {
			filterRow.Add(hGap(4))
		}
		filterRow.Add(container.NewGridWrap(filterButtonSize(48), container.NewCenter(btn.box())))
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

	rowHeight := filterButtonSize(48).Height
	filterWidth := filterRow.MinSize().Width
	if filterWidth < 260 {
		filterWidth = 260
	}
	filterWrap := container.NewGridWrap(fyne.NewSize(filterWidth, rowHeight), container.NewCenter(filterRow))
	toolbar := container.NewBorder(nil, nil, filterWrap, container.NewHBox(m.hHideTestsCheck, m.hHideBlockedCheck),
		m.hSearchEntry,
	)

	headers := []string{"Time", "PPK", "Code", "Type", "Description", "Zone", "Relay"}
	m.hList = widget.NewList(
		func() int {
			m.mu.RLock()
			defer m.mu.RUnlock()
			return m.hCount
		},
		func() fyne.CanvasObject { return newEventListItem() },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			m.mu.RLock()
			idx := m.hStart + int(id)
			if idx < 0 || idx >= len(m.hRows) {
				m.mu.RUnlock()
				return
			}
			e := m.hRows[idx]
			m.mu.RUnlock()
			bg, txt := getEventListItemParts(obj)
			rowBg, rowFg := m.eventRowColors(e.Category, idx)
			bg.FillColor = rowBg
			bg.Refresh()
			txt.Color = rowFg
			contentWidth := float32(0)
			if m.hScroll != nil {
				contentWidth = m.hScroll.Size().Width
			}
			txt.Text = m.formatEventLineAdaptive(e, contentWidth-12)
			txt.TextSize = fyne.CurrentApp().Settings().Theme().Size(theme.SizeNameText)
			txt.Refresh()
		},
	)
	m.hList.OnSelected = func(id widget.ListItemID) {
		m.selHistRow = m.hStart + int(id)
		if m.hList != nil {
			m.hList.Unselect(id)
		}
	}

	top := container.NewVBox(
		header,
		toolbar,
		vGap(6),
	)
	m.hScroll = container.NewScroll(m.hList)
	m.hScroll.OnScrolled = func(pos fyne.Position) {
		m.onHistScrolled(pos.Y)
	}
	centerContent := container.NewBorder(newTableHeader(headers), nil, nil, nil, m.hScroll)
	responsiveCenter := container.New(&refreshOnResizeLayout{
		onResize: func(_ fyne.Size) {
			if m.hList != nil {
				m.hList.Refresh()
			}
		},
	}, centerContent)
	center := cardContainer(cModal, responsiveCenter)
	content := container.NewBorder(top, nil, nil, nil, center)
	m.hWin.SetContent(content)
	m.hWin.Resize(fyne.NewSize(1024, 660))
	m.refreshHistoryFilters()
	m.refreshHistoryUI()
}

func (m *model) refreshHistoryFilters() {
	for f, btn := range m.hFilterBtns {
		btn.set(f == m.hEventType, f)
	}
}
