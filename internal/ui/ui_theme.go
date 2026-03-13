package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type win10Theme struct {
	base     fyne.Theme
	fontSize float32
}

func newWin10Theme(fontSize int) *win10Theme {
	if fontSize <= 0 {
		fontSize = 12
	}
	return &win10Theme{
		base:     theme.DefaultTheme(),
		fontSize: float32(fontSize),
	}
}

func (t *win10Theme) SetFontSize(size int) {
	if size <= 0 {
		size = 12
	}
	t.fontSize = float32(size)
}

func (t *win10Theme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return cBg
	case theme.ColorNameInputBackground:
		return cPanel
	case theme.ColorNameForeground, "text": // "text" is technically theme.ColorNameText in newer Fyne versions
		return cText
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 200, G: 206, B: 214, A: 255}
	case theme.ColorNamePlaceHolder:
		return cSoft
	case theme.ColorNameButton:
		return cAccent2
	case theme.ColorNamePrimary:
		return cAccent
	case theme.ColorNameHover:
		return cAccentSoft
	case theme.ColorNameFocus:
		return cAccent
	case theme.ColorNameShadow:
		return cBorder
	}
	return t.base.Color(name, variant)
}

func (t *win10Theme) Font(style fyne.TextStyle) fyne.Resource {
	return t.base.Font(style)
}

func (t *win10Theme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

func (t *win10Theme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameText {
		return t.fontSize
	}
	return t.base.Size(name)
}
