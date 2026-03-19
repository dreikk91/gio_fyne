//go:build windows

package walk

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
	"github.com/rs/zerolog/log"
)

var (
	comdlg32           = syscall.NewLazyDLL("comdlg32.dll")
	chooseColorW       = comdlg32.NewProc("ChooseColorW")
	customColors [16]uint32
)

type CHOOSECOLOR struct {
	LStructSize    uint32
	HwndOwner      win.HWND
	HInstance      win.HWND
	RgbResult      uint32
	LpCustColors   *uint32
	Flags          uint32
	LCustData      uintptr
	LpfnHook       uintptr
	LpTemplateName *uint16
}

const (
	CC_ANYCOLOR           = 0x00000100
	CC_FULLOPEN           = 0x00000002
	CC_RGBINIT            = 0x00000001
	CC_ENABLEHOOK         = 0x00000010
	CC_ENABLETEMPLATE     = 0x00000020
	CC_ENABLETEMPLATEHANDLE = 0x00000040
	CC_SOLIDCOLOR         = 0x00000080
)

func runColorDialog(owner walk.Form, color walk.Color) (walk.Color, bool) {
	var cc CHOOSECOLOR
	cc.LStructSize = uint32(unsafe.Sizeof(cc))
	if owner != nil {
		cc.HwndOwner = owner.Handle()
	}
	cc.RgbResult = uint32(color.R()) | (uint32(color.G()) << 8) | (uint32(color.B()) << 16)
	cc.LpCustColors = &customColors[0]
	cc.Flags = CC_ANYCOLOR | CC_FULLOPEN | CC_RGBINIT

	ret, _, _ := chooseColorW.Call(uintptr(unsafe.Pointer(&cc)))
	if ret == 0 {
		return 0, false
	}

	r := uint8(cc.RgbResult & 0xFF)
	g := uint8((cc.RgbResult >> 8) & 0xFF)
	b := uint8((cc.RgbResult >> 16) & 0xFF)

	return walk.RGB(r, g, b), true
}

type colorSettingsDialog struct {
	app *walkApp
	dlg *walk.Dialog
}

func (a *walkApp) openColorSettings() {
	if a.mw == nil {
		return
	}

	d := &colorSettingsDialog{
		app: a,
	}

	if _, err := d.run(); err != nil {
		log.Error().Err(err).Msg("failed to run color settings dialog")
		walk.MsgBox(a.mw, "Кольори подій", "Помилка створення вікна: "+err.Error(), walk.MsgBoxIconError)
	}
}

func (d *colorSettingsDialog) run() (int, error) {
	var db *walk.DataBinder

	// Pre-load types to ensure we have colors
	d.app.loadCategoryColors()

	return Dialog{
		AssignTo:   &d.dlg,
		Title:      "Кольори подій",
		MinSize:    Size{Width: 700, Height: 620},
		Layout:     VBox{Margins: Margins{Left: 12, Top: 12, Right: 12, Bottom: 12}, Spacing: 10},
		DataBinder: DataBinder{AssignTo: &db, DataSource: d},
		Children: []Widget{
			Composite{
				Background: SolidColorBrush{Color: colorSurface},
				Layout:     VBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}, Spacing: 2},
				Children: []Widget{
					Label{Text: "Палітра категорій подій", Font: Font{Family: "Segoe UI Semibold", PointSize: 11}},
					Label{Text: "Налаштуйте фон і колір тексту для кожної категорії. Зміни застосовуються після натискання \"Застосувати\".", TextColor: colorSoft},
				},
			},
			ScrollView{
				Background: SolidColorBrush{Color: colorWindow},
				Layout:     VBox{MarginsZero: true, Spacing: 8},
				Children: d.createColorPickers(),
			},
			Composite{
				Background: SolidColorBrush{Color: colorSurface},
				Layout:     HBox{Margins: Margins{Left: 10, Top: 8, Right: 10, Bottom: 8}, Spacing: 10},
				Children: []Widget{
					Label{Text: "Порада: комбінуйте високий контраст для кращої читабельності.", TextColor: colorSoft},
					HSpacer{},
					HSpacer{},
					PushButton{
						Text: "Закрити",
						OnClicked: func() {
							d.dlg.Close(walk.DlgCmdOK)
						},
					},
				},
			},
		},
	}.Run(d.app.mw)
}

func (d *colorSettingsDialog) createColorPickers() []Widget {
	var widgets []Widget
	for _, et := range d.app.eventTypes {
		etCopy := et // capture for closure

		title := strings.TrimSpace(etCopy.Title)
		if title == "" {
			title = etCopy.Key
		}
		subtitle := fmt.Sprintf("Категорія: %s", etCopy.Key)

		var preview *walk.Label
		var bgValueLabel *walk.Label
		var fgValueLabel *walk.Label

		// Ensure we have correct defaults based on hex parsing
		bgCol := hexToColor(etCopy.Color)
		if etCopy.Color == "" {
			bgCol = walk.RGB(255, 255, 255)
		}
		fgCol := hexToColor(etCopy.FontColor)
		if etCopy.FontColor == "" {
			fgCol = colorText
		}

		currentBg := bgCol
		currentFg := fgCol

		updatePreview := func() {
			if preview == nil {
				return
			}
			brush, _ := walk.NewSolidColorBrush(currentBg)
			preview.SetBackground(brush)
			preview.SetTextColor(currentFg)
			if bgValueLabel != nil {
				bgValueLabel.SetText("Фон: " + colorHex(currentBg))
			}
			if fgValueLabel != nil {
				fgValueLabel.SetText("Текст: " + colorHex(currentFg))
			}
		}

		widgets = append(widgets, Composite{
			Background: SolidColorBrush{Color: colorSurface},
			Layout:     HBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}, Spacing: 10},
			Children: []Widget{
				Composite{
					MinSize: Size{Width: 220},
					Layout:  VBox{MarginsZero: true, Spacing: 1},
					Children: []Widget{
						Label{Text: title, Font: Font{Family: "Segoe UI Semibold", PointSize: 10}},
						Label{Text: subtitle, TextColor: colorSoft},
					},
				},
				Label{
					AssignTo:      &preview,
					StretchFactor: 1,
					Text:          " SAMPLE EVENT 1301 ALARM ",
					TextColor:     currentFg,
					MinSize:       Size{Width: 250, Height: 34},
					Background:    SolidColorBrush{Color: currentBg},
				},
				Composite{
					MinSize: Size{Width: 140},
					Layout:  VBox{MarginsZero: true, Spacing: 2},
					Children: []Widget{
						Label{AssignTo: &bgValueLabel, Text: "Фон: " + colorHex(currentBg), TextColor: colorSoft},
						Label{AssignTo: &fgValueLabel, Text: "Текст: " + colorHex(currentFg), TextColor: colorSoft},
					},
				},
				PushButton{
					Text: "Фон...",
					OnClicked: func() {
						if newColor, ok := runColorDialog(d.dlg, currentBg); ok {
							currentBg = newColor
							updatePreview()
						}
					},
				},
				PushButton{
					Text: "Текст...",
					OnClicked: func() {
						if newColor, ok := runColorDialog(d.dlg, currentFg); ok {
							currentFg = newColor
							updatePreview()
						}
					},
				},
				PushButton{
					Text: "Скинути",
					OnClicked: func() {
						currentBg = bgCol
						currentFg = fgCol
						updatePreview()
					},
				},
				PushButton{
					Text: "Застосувати",
					OnClicked: func() {
						bgHex := colorHex(currentBg)
						fgHex := colorHex(currentFg)

						log.Info().Str("category", etCopy.Key).Str("bg", bgHex).Str("fg", fgHex).Msg("updating category color")

						go func() {
							if err := d.app.rt.SaveEventTypeColors(d.app.ctx, etCopy.Key, bgHex, fgHex); err != nil {
								log.Error().Err(err).Msg("failed to save category color")
								d.dlg.Synchronize(func() {
									walk.MsgBox(d.dlg, "Помилка", "Не вдалося зберегти колір: "+err.Error(), walk.MsgBoxIconError)
								})
								return
							}

							d.dlg.Synchronize(func() {
								d.app.loadCategoryColors()
								d.app.repaintTables()
							})
						}()
					},
				},
			},
		})
	}
	return widgets
}

func colorHex(c walk.Color) string {
	return fmt.Sprintf("#%02X%02X%02X", c.R(), c.G(), c.B())
}
