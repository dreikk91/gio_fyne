package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

func cardContainer(bg color.NRGBA, content fyne.CanvasObject) fyne.CanvasObject {
	rect := canvas.NewRectangle(bg)
	rect.StrokeColor = cBorder
	rect.StrokeWidth = 1
	rect.CornerRadius = 8
	return container.NewMax(rect, container.NewPadded(content))
}

func newMetricCard(label, value string, bg, fg color.NRGBA) *metricCardRef {
	bgRect := canvas.NewRectangle(bg)
	bgRect.StrokeColor = cBorder
	bgRect.StrokeWidth = 1
	bgRect.CornerRadius = 8
	lbl := canvas.NewText(label+":", cSoft)
	lbl.TextSize = 11
	val := canvas.NewText(value, fg)
	val.TextSize = 12
	row := container.NewHBox(lbl, layout.NewSpacer(), val)
	return &metricCardRef{bg: bgRect, label: lbl, value: val, container: container.NewMax(bgRect, container.NewPadded(row))}
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
	bgRect.CornerRadius = 8
	lbl := canvas.NewText(label+":", cSoft)
	lbl.TextSize = 11
	val := canvas.NewText(value, fg)
	val.TextSize = 11
	row := container.NewHBox(lbl, layout.NewSpacer(), val)
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

type tableTextCell struct {
	*fyne.Container
	bg   *canvas.Rectangle
	text *canvas.Text
}

func newTableTextCell() fyne.CanvasObject {
	lbl := widget.NewLabel("")
	lbl.Truncation = fyne.TextTruncateEllipsis
	lbl.Wrapping = fyne.TextWrapOff
	return lbl
}

type eventCell struct {
	*fyne.Container
	bg   *canvas.Rectangle
	text *widget.Label
}

func newEventCell() fyne.CanvasObject {
	cell := &eventCell{
		bg:   canvas.NewRectangle(cPanel),
		text: widget.NewLabel(""),
	}
	cell.text.Truncation = fyne.TextTruncateEllipsis
	cell.text.Wrapping = fyne.TextWrapOff
	cell.Container = container.NewMax(cell.bg, container.NewPadded(cell.text))
	return cell
}

func getEventCellParts(obj fyne.CanvasObject) (*canvas.Rectangle, *widget.Label) {
	c := obj.(*eventCell)
	return c.bg, c.text
}

type tableActionCell struct {
	*fyne.Container
	bg        *canvas.Rectangle
	label     *canvas.Text
	history   *widget.Button
	deleteBtn *widget.Button
	buttons   *fyne.Container
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

type objectRow struct {
	*fyne.Container
	bg        *canvas.Rectangle
	state     *widget.Label
	ppk       *widget.Label
	client    *widget.Label
	lastEvent *widget.Label
	timeText  *widget.Label
	history   *widget.Button
	deleteBtn *widget.Button

	onHistory func()
	onDelete  func()
}

func newObjectRow() fyne.CanvasObject {
	row := &objectRow{
		bg:        canvas.NewRectangle(cPanel),
		state:     widget.NewLabel("-"),
		ppk:       widget.NewLabel("-"),
		client:    widget.NewLabel("-"),
		lastEvent: widget.NewLabel("-"),
		timeText:  widget.NewLabel("-"),
		history:   widget.NewButton("History", nil),
		deleteBtn: widget.NewButton("Delete", nil),
	}
	row.state.Truncation = fyne.TextTruncateEllipsis
	row.ppk.Truncation = fyne.TextTruncateEllipsis
	row.client.Truncation = fyne.TextTruncateEllipsis
	row.lastEvent.Truncation = fyne.TextTruncateEllipsis
	row.timeText.Truncation = fyne.TextTruncateEllipsis
	row.history.Importance = widget.LowImportance
	row.deleteBtn.Importance = widget.DangerImportance
	row.bg.StrokeColor = cBorder
	row.bg.StrokeWidth = 1
	row.bg.CornerRadius = 8
	cols := container.NewGridWithColumns(6,
		row.state, row.ppk, row.client, row.lastEvent, row.timeText,
		container.NewHBox(row.history, row.deleteBtn),
	)
	wrap := container.NewMax(row.bg, container.NewPadded(cols))
	row.Container = wrap
	row.history.OnTapped = func() {
		if row.onHistory != nil {
			row.onHistory()
		}
	}
	row.deleteBtn.OnTapped = func() {
		if row.onDelete != nil {
			row.onDelete()
		}
	}
	return row
}

func (r *objectRow) setData(state, ppk, client, lastEvent, time string, bg color.NRGBA) {
	r.state.SetText(state)
	r.ppk.SetText(ppk)
	r.client.SetText(client)
	r.lastEvent.SetText(lastEvent)
	r.timeText.SetText(time)
	r.bg.FillColor = bg
	r.bg.Refresh()
}

func (m *model) runOnUI(fn func()) {
	fn()
}

func vGap(h float32) fyne.CanvasObject {
	sp := canvas.NewRectangle(color.NRGBA{A: 0})
	sp.SetMinSize(fyne.NewSize(0, h))
	return sp
}

type relayObjCell struct {
	*fyne.Container
	check *widget.Check
	label *widget.Label
}

func newRelayObjCell() fyne.CanvasObject {
	chk := widget.NewCheck("", nil)
	lbl := widget.NewLabel("")
	lbl.Alignment = fyne.TextAlignLeading
	return &relayObjCell{
		Container: container.NewHBox(chk, lbl),
		check:     chk,
		label:     lbl,
	}
}

type relayCodeCell struct {
	*fyne.Container
	check  *widget.Check
	label  *widget.Label
	config *widget.Button
}

func newRelayCodeCell() fyne.CanvasObject {
	chk := widget.NewCheck("", nil)
	lbl := widget.NewLabel("")
	lbl.Alignment = fyne.TextAlignLeading
	cfg := widget.NewButton("Config", nil)
	cfg.Importance = widget.LowImportance
	return &relayCodeCell{
		Container: container.NewBorder(nil, nil, chk, cfg, lbl),
		check:     chk,
		label:     lbl,
		config:    cfg,
	}
}
