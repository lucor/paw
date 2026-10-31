// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

type vaultIconSize uint8

const (
	vaultIconLarge vaultIconSize = iota
	vaultIconMedium
	vaultIconSmall
)

const (
	vaultPencilBadgeSize = 20
	vaultPencilIconSize  = 14
)

// newEditableVaultPreview adds the shared edit badge to a clickable large preview.
func newEditableVaultPreview(appearance storage.VaultAppearance, button *widget.Button) fyne.CanvasObject {
	background := canvas.NewCircle(theme.Color(theme.ColorNameBackground))
	icon := container.NewCenter(container.NewGridWrap(fyne.NewSquareSize(vaultPencilIconSize), widget.NewIcon(icons.Pencil)))
	pencil := container.NewGridWrap(fyne.NewSquareSize(vaultPencilBadgeSize), container.NewStack(background, icon))
	size, _ := vaultIconLarge.dimensions()
	return container.NewGridWrap(fyne.NewSquareSize(size), container.NewStack(button, newVaultIcon(appearance, vaultIconLarge),
		container.NewBorder(nil, nil, nil, container.NewVBox(layout.NewSpacer(), pencil)),
	))
}

func (s vaultIconSize) dimensions() (containerSize, iconSize float32) {
	switch s {
	case vaultIconLarge:
		return 80, 44
	case vaultIconMedium:
		return 40, 24
	default:
		return 28, 18
	}
}

type vaultIcon struct {
	widget.BaseWidget
	appearance storage.VaultAppearance
	size       vaultIconSize
}

func newVaultIcon(appearance storage.VaultAppearance, size vaultIconSize) *vaultIcon {
	icon := &vaultIcon{appearance: storage.NormalizeVaultAppearance(appearance), size: size}
	icon.ExtendBaseWidget(icon)
	return icon
}

func (i *vaultIcon) CreateRenderer() fyne.WidgetRenderer {
	background := canvas.NewRectangle(color.Transparent)
	image := canvas.NewImageFromResource(nil)
	image.FillMode = canvas.ImageFillContain
	renderer := &vaultIconRenderer{WidgetRenderer: widget.NewSimpleRenderer(container.NewWithoutLayout(background, image)), icon: i, background: background, image: image}
	renderer.Refresh()
	return renderer
}

type vaultIconRenderer struct {
	fyne.WidgetRenderer
	icon       *vaultIcon
	background *canvas.Rectangle
	image      *canvas.Image
	resource   string
}

func (r *vaultIconRenderer) Layout(size fyne.Size) {
	r.background.Resize(size)
	_, iconSize := r.icon.size.dimensions()
	imageSize := fyne.NewSquareSize(iconSize)
	r.image.Resize(imageSize)
	r.image.Move(fyne.NewPos((size.Width-imageSize.Width)/2, (size.Height-imageSize.Height)/2))
}

func (r *vaultIconRenderer) MinSize() fyne.Size {
	containerSize, _ := r.icon.size.dimensions()
	return fyne.NewSquareSize(containerSize)
}

func (r *vaultIconRenderer) Refresh() {
	variant := theme.VariantLight
	if app := fyne.CurrentApp(); app != nil {
		variant = app.Settings().ThemeVariant()
	}
	palette := vaultIconPalette(r.icon.appearance.Color, variant, r.icon.Theme())
	r.background.FillColor = palette.background
	containerSize, _ := r.icon.size.dimensions()
	r.background.CornerRadius = containerSize * 0.2
	r.background.Refresh()

	source := vaultIconResource(r.icon.appearance.Icon)
	if source.Name() == r.resource {
		r.WidgetRenderer.Refresh()
		return
	}
	r.image.Resource = source
	r.resource = source.Name()
	r.image.Refresh()
	r.WidgetRenderer.Refresh()
}

type vaultIconColors struct {
	background color.Color
}

type vaultColorPalette struct {
	backgroundLight color.NRGBA
	backgroundDark  color.NRGBA
}

var vaultIconPalettes = map[string]vaultColorPalette{
	"gray":   {backgroundLight: hexColor(0xE2E8F0), backgroundDark: hexColor(0x273342)},
	"blue":   {backgroundLight: hexColor(0xDBEAFE), backgroundDark: hexColor(0x1E3A5F)},
	"green":  {backgroundLight: hexColor(0xDCFCE7), backgroundDark: hexColor(0x173B2A)},
	"purple": {backgroundLight: hexColor(0xF3E8FF), backgroundDark: hexColor(0x3B2453)},
	"orange": {backgroundLight: hexColor(0xFFEDD5), backgroundDark: hexColor(0x4A2C16)},
	"red":    {backgroundLight: hexColor(0xFEE2E2), backgroundDark: hexColor(0x4B2424)},
}

func hexColor(value uint32) color.NRGBA {
	return color.NRGBA{R: uint8(value >> 16), G: uint8(value >> 8), B: uint8(value), A: 255}
}

func vaultIconPalette(id string, variant fyne.ThemeVariant, activeTheme fyne.Theme) vaultIconColors {
	if id == "none" {
		return vaultIconColors{background: activeTheme.Color(theme.ColorNameInputBackground, variant)}
	}
	palette, ok := vaultIconPalettes[id]
	if !ok {
		return vaultIconPalette("none", variant, activeTheme)
	}
	if isDarkVaultTheme(activeTheme, variant) {
		return vaultIconColors{background: palette.backgroundDark}
	}
	return vaultIconColors{background: palette.backgroundLight}
}

func isDarkVaultTheme(activeTheme fyne.Theme, variant fyne.ThemeVariant) bool {
	red, green, blue, _ := activeTheme.Color(theme.ColorNameForeground, variant).RGBA()
	luminance := (299*uint64(red) + 587*uint64(green) + 114*uint64(blue)) / 1000
	return luminance > 32767
}

func vaultIconResource(id string) fyne.Resource {
	switch id {
	case "paw":
		return icons.Paw
	case "briefcase-business":
		return icons.BriefcaseBusiness
	case "house":
		return icons.House
	case "archive":
		return icons.Archive
	case "shield":
		return icons.Shield
	case "folder":
		return icons.Folder
	case "key-round":
		return icons.Password
	case "laptop":
		return icons.Laptop
	default: // Preserve the icon identifier used by vaults created before Paw was added.
		return icons.Vault
	}
}

func vaultColor(id string) color.Color {
	return vaultIconPalette(id, theme.VariantLight, theme.DefaultTheme()).background
}

func selectedVaultColor(id string) color.Color {
	app := fyne.CurrentApp()
	if app == nil {
		return vaultColor(id)
	}
	return vaultIconPalette(id, app.Settings().ThemeVariant(), app.Settings().Theme()).background
}
