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
		AssignTo:      &d.dlg,
		Title:         "Налаштування кольорів подій",
		MinSize:       Size{Width: 400, Height: 500},
		Layout:        VBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}, Spacing: 10},
		DataBinder:    DataBinder{AssignTo: &db, DataSource: d},
		Children: []Widget{
			Label{Text: "Виберіть колір фону для кожної категорії подій:", Font: Font{PointSize: 10, Bold: true}},
			ScrollView{
				Layout: VBox{MarginsZero: true, Spacing: 5},
				Children: d.createColorPickers(),
			},
			Composite{
				Layout: HBox{MarginsZero: true, Spacing: 10},
				Children: []Widget{
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

		colorLabel := strings.TrimSpace(etCopy.Title)
		if colorLabel == "" {
			colorLabel = etCopy.Key
		}

		var preview *walk.Label
		
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

		widgets = append(widgets, Composite{
			Layout: HBox{MarginsZero: true, Spacing: 10},
			Children: []Widget{
				Label{Text: colorLabel, MinSize: Size{Width: 100}},
				Label{
					AssignTo:   &preview,
					Text:       " Text ",
					TextColor:  currentFg,
					MinSize:    Size{Width: 50, Height: 20},
					Background: SolidColorBrush{Color: currentBg},
				},
				PushButton{
					Text: "Фон",
					OnClicked: func() {
						if newColor, ok := runColorDialog(d.dlg, currentBg); ok {
							currentBg = newColor
							brush, _ := walk.NewSolidColorBrush(currentBg)
							preview.SetBackground(brush)
						}
					},
				},
				PushButton{
					Text: "Текст",
					OnClicked: func() {
						if newColor, ok := runColorDialog(d.dlg, currentFg); ok {
							currentFg = newColor
							preview.SetTextColor(currentFg)
						}
					},
				},
				PushButton{
					Text: "Зберегти",
					OnClicked: func() {
						// Convert walk.Color back to Hex
						bgHex := fmt.Sprintf("#%02X%02X%02X", currentBg.R(), currentBg.G(), currentBg.B())
						fgHex := fmt.Sprintf("#%02X%02X%02X", currentFg.R(), currentFg.G(), currentFg.B())

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
				HSpacer{},
			},
		})
	}
	return widgets
}
