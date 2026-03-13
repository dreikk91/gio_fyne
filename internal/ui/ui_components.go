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

func (m *model) runOnUI(fn func()) {
	fn()
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
