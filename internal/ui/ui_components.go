package ui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func cardContainer(bg color.NRGBA, content fyne.CanvasObject) fyne.CanvasObject {
	rect := canvas.NewRectangle(bg)
	rect.StrokeColor = cBorder
	rect.StrokeWidth = 1
	rect.CornerRadius = 10
	return container.NewMax(rect, container.NewPadded(content))
}

func newMetricCard(label, value string, bg, fg color.NRGBA) *metricCardRef {
	bgRect := canvas.NewRectangle(bg)
	bgRect.StrokeColor = cBorder
	bgRect.StrokeWidth = 1
	bgRect.CornerRadius = 10
	lbl := canvas.NewText(label, cSoft)
	lbl.TextSize = 10
	lbl.TextStyle = fyne.TextStyle{Bold: true}
	val := canvas.NewText(value, fg)
	val.TextSize = 16
	val.TextStyle = fyne.TextStyle{Bold: true}
	col := container.NewVBox(lbl, val)
	return &metricCardRef{bg: bgRect, label: lbl, value: val, container: container.NewMax(bgRect, container.NewPadded(col))}
}

func (m *metricCardRef) box() fyne.CanvasObject { return m.container }

func (m *metricCardRef) set(value string, bg, fg color.NRGBA) {
	m.value.Text = value
	m.value.Color = fg
	m.bg.FillColor = bg
	m.value.Refresh()
	m.bg.Refresh()
}

func newChip(label, value string, fg, bg color.NRGBA) *chipRef {
	bgRect := canvas.NewRectangle(bg)
	bgRect.StrokeColor = cBorder
	bgRect.StrokeWidth = 1
	bgRect.CornerRadius = 12
	lbl := canvas.NewText(label, cSoft)
	lbl.TextSize = 9
	val := canvas.NewText(" "+value, fg)
	val.TextSize = 10
	val.TextStyle = fyne.TextStyle{Bold: true}
	row := container.NewHBox(lbl, val)
	return &chipRef{bg: bgRect, label: lbl, value: val, container: container.NewMax(bgRect, container.NewPadded(row))}
}

func (c *chipRef) box() fyne.CanvasObject { return c.container }

func (c *chipRef) set(value string, fg, bg color.NRGBA) {
	c.value.Text = value
	c.value.Color = fg
	c.bg.FillColor = bg
	c.value.Refresh()
	c.bg.Refresh()
}

type filterButtonRef struct {
	bg        *canvas.Rectangle
	btn       *widget.Button
	container fyne.CanvasObject
}

func newFilterButton(label string, onTap func()) *filterButtonRef {
	bg := canvas.NewRectangle(color.Transparent)
	bg.StrokeColor = color.Transparent
	bg.StrokeWidth = 1
	bg.CornerRadius = 4
	btn := widget.NewButton(label, onTap)
	btn.Importance = widget.LowImportance
	box := container.NewMax(bg, btn)
	return &filterButtonRef{bg: bg, btn: btn, container: box}
}

func (f *filterButtonRef) box() fyne.CanvasObject { return f.container }

func (f *filterButtonRef) set(active bool, filter string) {
	if active {
		bg, _ := filterTone(filter)
		f.bg.FillColor = bg
		f.bg.StrokeColor = cBorder
		f.btn.Importance = widget.HighImportance
	} else {
		f.bg.FillColor = color.Transparent
		f.bg.StrokeColor = color.Transparent
		f.btn.Importance = widget.LowImportance
	}
	f.bg.Refresh()
	f.btn.Refresh()
}

func newStatusBanner() *statusBannerRef {
	bg := canvas.NewRectangle(cAccentSoft)
	bg.CornerRadius = 8
	bg.StrokeColor = cBorder
	bg.StrokeWidth = 1
	text := canvas.NewText("", cAccent)
	text.TextSize = 12
	box := container.NewMax(bg, container.NewPadded(text))
	box.Hide()
	return &statusBannerRef{bg: bg, text: text, box: box}
}

func newTableHeader(cols []string) fyne.CanvasObject {
	items := make([]fyne.CanvasObject, 0, len(cols))
	for _, c := range cols {
		lbl := canvas.NewText(c, cSoft)
		lbl.TextSize = 11
		items = append(items, lbl)
	}
	grid := container.NewGridWithColumns(len(cols), items...)
	return cardContainer(cPanel3, grid)
}

func wrapTableWithScroll(tbl *widget.Table) fyne.CanvasObject {
	return container.NewScroll(tbl)
}

func (m *model) newObjectsTable(headers []string) *widget.Table {
	t := widget.NewTable(
		func() (int, int) {
			m.mu.RLock()
			defer m.mu.RUnlock()
			return m.objCount + 1, len(headers)
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
			idx := m.objStart + dataRow
			if idx < 0 || idx >= len(m.filteredDevices) {
				m.mu.RUnlock()
				return
			}
			d := m.filteredDevices[idx]
			m.mu.RUnlock()
			stale := isStale(d.LastEventTime, m.activityTO)

			rowBg := rowAltColor(dataRow)
			eventCat := m.getEventCategoryForDevice(d.ID)
			if !stale && eventCat != "" {
				rowBg, _ = m.eventRowColors(eventCat, dataRow)
			}

			if stale {
				rowBg = cBadSoft
			}
			if m.selObjRow == idx {
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
	t.SetColumnWidth(0, 85)
	t.SetColumnWidth(1, 65)
	t.SetColumnWidth(2, 160)
	t.SetColumnWidth(3, 320)
	t.SetColumnWidth(4, 160)
	t.SetColumnWidth(5, 150)
	return t
}

func (m *model) newEventsList() *widget.List {
	return widget.NewList(
		func() int {
			m.mu.RLock()
			defer m.mu.RUnlock()
			return m.evtCount
		},
		func() fyne.CanvasObject {
			return newEventListItem()
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			m.mu.RLock()
			idx := m.evtStart + int(id)
			if idx < 0 || idx >= len(m.filteredEvents) {
				m.mu.RUnlock()
				return
			}
			e := m.filteredEvents[idx]
			m.mu.RUnlock()
			bg, txt := getEventListItemParts(obj)
			rowBg, rowFg := m.eventRowColors(e.Category, idx)
			bg.FillColor = rowBg
			bg.Refresh()
			txt.Color = rowFg
			contentWidth := float32(0)
			if m.evtScroll != nil {
				contentWidth = m.evtScroll.Size().Width
			}
			txt.Text = m.formatEventLineAdaptive(e, contentWidth-12)
			txt.TextSize = fyne.CurrentApp().Settings().Theme().Size(theme.SizeNameText)
			txt.Refresh()
		},
	)
}

func newTableTextCell() fyne.CanvasObject {
	lbl := widget.NewLabel("")
	lbl.Truncation = fyne.TextTruncateEllipsis
	lbl.Wrapping = fyne.TextWrapOff
	return lbl
}

func newEventCell() fyne.CanvasObject {
	bg := canvas.NewRectangle(cPanel)
	lbl := widget.NewLabel("")
	lbl.Truncation = fyne.TextTruncateEllipsis
	lbl.Wrapping = fyne.TextWrapOff
	return container.NewMax(bg, lbl)
}

func getEventCellParts(obj fyne.CanvasObject) (*canvas.Rectangle, *widget.Label) {
	cell := obj.(*fyne.Container)
	bg := cell.Objects[0].(*canvas.Rectangle)
	lbl := cell.Objects[1].(*widget.Label)
	return bg, lbl
}

func newEventListItem() fyne.CanvasObject {
	bg := canvas.NewRectangle(cPanel)
	txt := canvas.NewText("", cText)
	txt.TextSize = 12
	return container.NewStack(bg, container.NewPadded(txt))
}

func getEventListItemParts(obj fyne.CanvasObject) (*canvas.Rectangle, *canvas.Text) {
	stack := obj.(*fyne.Container)
	bg := stack.Objects[0].(*canvas.Rectangle)
	txtContainer := stack.Objects[1].(*fyne.Container)
	txt := txtContainer.Objects[0].(*canvas.Text)
	return bg, txt
}

func newTableActionCell() fyne.CanvasObject {
	bg := canvas.NewRectangle(cPanel)
	lbl := widget.NewLabel("")
	lbl.Truncation = fyne.TextTruncateEllipsis
	lbl.Wrapping = fyne.TextWrapOff
	h := widget.NewButton("History", nil)
	d := widget.NewButton("Delete", nil)
	h.Importance = widget.LowImportance
	d.Importance = widget.DangerImportance
	btns := container.NewHBox(h, d)
	btns.Hide()
	return container.NewMax(bg, lbl, btns)
}

func getActionCellParts(obj fyne.CanvasObject) (*canvas.Rectangle, *widget.Label, *fyne.Container, *widget.Button, *widget.Button) {
	c := obj.(*fyne.Container)
	bg := c.Objects[0].(*canvas.Rectangle)
	lbl := c.Objects[1].(*widget.Label)
	btns := c.Objects[2].(*fyne.Container)
	btnBox := btns.Objects[0].(*widget.Button)
	btnBox2 := btns.Objects[1].(*widget.Button)
	return bg, lbl, btns, btnBox, btnBox2
}

func (m *model) runOnUI(fn func()) {
	fyne.Do(fn)
}

func vGap(h float32) fyne.CanvasObject {
	sp := canvas.NewRectangle(color.NRGBA{A: 0})
	sp.SetMinSize(fyne.NewSize(0, h))
	return sp
}

func newRelayObjCell() fyne.CanvasObject {
	chk := widget.NewCheck("", nil)
	lbl := widget.NewLabel("")
	lbl.Alignment = fyne.TextAlignLeading
	return container.NewHBox(chk, lbl)
}

func getRelayObjCellParts(obj fyne.CanvasObject) (*widget.Check, *widget.Label) {
	cell := obj.(*fyne.Container)
	chk := cell.Objects[0].(*widget.Check)
	lbl := cell.Objects[1].(*widget.Label)
	return chk, lbl
}

func newRelayCodeCell() fyne.CanvasObject {
	chk := widget.NewCheck("", nil)
	lbl := widget.NewLabel("")
	lbl.Alignment = fyne.TextAlignLeading
	cfg := widget.NewButton("Config", nil)
	cfg.Importance = widget.LowImportance
	return container.NewHBox(chk, lbl, layout.NewSpacer(), cfg)
}

func getRelayCdCellParts(obj fyne.CanvasObject) (*widget.Check, *widget.Label, *widget.Button) {
	cell := obj.(*fyne.Container)
	chk := cell.Objects[0].(*widget.Check)
	lbl := cell.Objects[1].(*widget.Label)
	cfg := cell.Objects[3].(*widget.Button)
	return chk, lbl, cfg
}

func getTableCellParts(obj fyne.CanvasObject) (*canvas.Rectangle, *widget.Label) {
	cell := obj.(*fyne.Container)
	bg := cell.Objects[0].(*canvas.Rectangle)
	lbl := cell.Objects[1].(*widget.Label)
	return bg, lbl
}

func filterButtonSize(width float32) fyne.Size {
	h := fyne.CurrentApp().Settings().Theme().Size(theme.SizeNameText) + 4
	if h < 18 {
		h = 18
	}
	return fyne.NewSize(width, h)
}

func hGap(w float32) fyne.CanvasObject {
	sp := canvas.NewRectangle(color.NRGBA{A: 0})
	sp.SetMinSize(fyne.NewSize(w, 0))
	return sp
}
