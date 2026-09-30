package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type organizerTheme struct{ base fyne.Theme }

func newOrganizerTheme() fyne.Theme { return &organizerTheme{base: theme.DefaultTheme()} }

func (t *organizerTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	dark := variant == theme.VariantDark
	pair := func(light, night color.NRGBA) color.Color {
		if dark {
			return night
		}
		return light
	}
	switch name {
	case theme.ColorNameBackground, theme.ColorNameOverlayBackground:
		return pair(color.NRGBA{246, 247, 249, 255}, color.NRGBA{28, 29, 32, 255})
	case "sidebarBackground":
		return pair(color.NRGBA{234, 236, 240, 255}, color.NRGBA{35, 36, 40, 255})
	case theme.ColorNameInputBackground:
		return pair(color.NRGBA{255, 255, 255, 255}, color.NRGBA{43, 44, 49, 255})
	// Keep the same neutral silhouette when an action becomes unavailable.
	// Fyne still uses disabled text/icons and suppresses hover and activation.
	case theme.ColorNameButton, theme.ColorNameDisabledButton:
		return pair(color.NRGBA{233, 235, 239, 255}, color.NRGBA{53, 55, 61, 255})
	case theme.ColorNameForeground:
		return pair(color.NRGBA{32, 35, 41, 255}, color.NRGBA{239, 240, 244, 255})
	case theme.ColorNameDisabled:
		return pair(color.NRGBA{109, 113, 122, 255}, color.NRGBA{155, 159, 170, 255})
	case theme.ColorNameSeparator, theme.ColorNameInputBorder:
		return pair(color.NRGBA{207, 211, 218, 255}, color.NRGBA{76, 79, 88, 255})
	case theme.ColorNamePrimary:
		return pair(color.NRGBA{25, 94, 195, 255}, color.NRGBA{112, 173, 255, 255})
	case theme.ColorNameError:
		return pair(color.NRGBA{190, 42, 48, 255}, color.NRGBA{242, 130, 132, 255})
	case theme.ColorNameForegroundOnPrimary, theme.ColorNameForegroundOnError, theme.ColorNameForegroundOnSuccess:
		return pair(color.NRGBA{255, 255, 255, 255}, color.NRGBA{18, 28, 45, 255})
	case theme.ColorNameSuccess:
		return pair(color.NRGBA{27, 119, 70, 255}, color.NRGBA{104, 206, 151, 255})
	}

	return t.base.Color(name, variant)
}

func (t *organizerTheme) Font(style fyne.TextStyle) fyne.Resource    { return t.base.Font(style) }
func (t *organizerTheme) Icon(name fyne.ThemeIconName) fyne.Resource { return t.base.Icon(name) }
func (t *organizerTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 4
	case theme.SizeNameInnerPadding:
		return 8
	case theme.SizeNameText:
		return 14
	case theme.SizeNameButtonRadius, theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 6
	default:
		return t.base.Size(name)
	}
}
