package ui

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"github.com/rs/zerolog/log"
)

func (m *model) loadCategoryColors() {
	types, err := m.rt.GetEventTypes(m.ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to load event type colors")
		return
	}

	bgMap := make(map[string]color.NRGBA, len(types))
	fgMap := make(map[string]color.NRGBA, len(types))
	for _, et := range types {
		key := strings.ToLower(strings.TrimSpace(et.Key))
		if key == "" {
			continue
		}
		if clr, ok := parseHexColor(et.Color); ok {
			bgMap[key] = clr
		}
		if clr, ok := parseHexColor(et.FontColor); ok {
			fgMap[key] = clr
		}
	}

	m.mu.Lock()
	m.eventTypes = types
	m.categoryColors = bgMap
	m.categoryFontColors = fgMap
	m.mu.Unlock()
}

func (m *model) openColorSettings() {
	types, err := m.rt.GetEventTypes(m.ctx)
	if err != nil {
		dialog.ShowError(err, m.win)
		return
	}
	m.eventTypes = types

	form := widget.NewForm()
	for _, et := range types {
		etCopy := et
		label := strings.TrimSpace(etCopy.Title)
		if label == "" {
			label = etCopy.Key
		}

		entry := widget.NewEntry()
		entry.SetText(etCopy.Color)
		entry.SetPlaceHolder("#RRGGBB")

		preview := canvas.NewRectangle(hexToColor(etCopy.Color))
		preview.SetMinSize(fyne.NewSize(24, 24))

		colorPickerBtn := widget.NewButton("Pick Color", func() {
			cp := dialog.NewColorPicker("Pick color for "+label, "Choose a color", func(c color.Color) {
				r, g, b, _ := c.RGBA()
				newHex := fmt.Sprintf("#%02X%02X%02X", uint8(r>>8), uint8(g>>8), uint8(b>>8))
				entry.SetText(newHex)
				preview.FillColor = c
				preview.Refresh()
			}, m.win)
			cp.Show()
		})

		entry.OnChanged = func(s string) {
			if len(s) == 7 && strings.HasPrefix(s, "#") {
				preview.FillColor = hexToColor(s)
				preview.Refresh()
			}
		}

		form.Append(label, container.NewBorder(nil, nil, preview, colorPickerBtn, entry))
	}

	d := dialog.NewCustomConfirm("Event Colors", "Save", "Cancel", container.NewVScroll(form), func(save bool) {
		if !save {
			return
		}

		for i, item := range form.Items {
			// Center object of Border is entry
			border := item.Widget.(*fyne.Container)
			entry := border.Objects[0].(*widget.Entry)
			newColor := strings.ToUpper(strings.TrimSpace(entry.Text))
			if !strings.HasPrefix(newColor, "#") || len(newColor) != 7 {
				continue
			}

			key := m.eventTypes[i].Key
			if err := m.rt.SaveEventTypeColors(m.ctx, key, newColor, m.eventTypes[i].FontColor); err != nil {
				log.Error().Err(err).Str("key", key).Msg("failed to save color")
			}
		}

		// Refresh UI
		m.loadCategoryColors()
		m.statusMsg = "Colors saved"
		m.refreshMainUI()
	}, m.win)

	d.Resize(fyne.NewSize(450, 500))
	d.Show()
}
