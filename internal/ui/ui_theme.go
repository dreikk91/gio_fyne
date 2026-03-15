package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type modernTheme struct {
	base     fyne.Theme
	fontSize float32
}

func newModernTheme(fontSize int) *modernTheme {
	if fontSize <= 0 {
		fontSize = 12
	}
	return &modernTheme{
		base:     theme.DefaultTheme(),
		fontSize: float32(fontSize),
	}
}

func (t *modernTheme) SetFontSize(size int) {
	if size <= 0 {
		size = 12
	}
	t.fontSize = float32(size)
}

func (t *modernTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return cBg
	case theme.ColorNameInputBackground:
		return cPanel
	case theme.ColorNameForeground:
		return cText
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 148, G: 163, B: 184, A: 255} // Slate-400
	case theme.ColorNamePlaceHolder:
		return cSoft
	case theme.ColorNameButton:
		return cBorder
	case theme.ColorNamePrimary:
		return cAccent
	case theme.ColorNameHover:
		return cAccentSoft
	case theme.ColorNameFocus:
		return cAccent
	case theme.ColorNameShadow:
		return color.NRGBA{A: 0} // Transparent shadows for a cleaner look
	case theme.ColorNameSuccess:
		return cGood
	case theme.ColorNameWarning:
		return cWarn
	case theme.ColorNameError:
		return cBad
	}
	return t.base.Color(name, variant)
}

func (t *modernTheme) Font(style fyne.TextStyle) fyne.Resource {
	return t.base.Font(style)
}

func (t *modernTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

func (t *modernTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return t.fontSize
	case theme.SizeNamePadding:
		return 6 // Reduced padding for higher density
	case theme.SizeNameInputRadius:
		return 6
	case theme.SizeNameSelectionRadius:
		return 4
	}
	return t.base.Size(name)
}

