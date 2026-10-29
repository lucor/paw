// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// iconPreview frames a favicon or entry icon with a theme-aware background.
type iconPreview struct {
	widget.BaseWidget
	background *canvas.Rectangle
	icon       *widget.Icon
}

func newIconPreview(resource fyne.Resource) *iconPreview {
	if resource == nil {
		resource = icons.Globe
	}
	preview := &iconPreview{
		background: canvas.NewRectangle(color.Transparent),
		icon:       widget.NewIcon(resource),
	}
	preview.ExtendBaseWidget(preview)
	return preview
}

func (f *iconPreview) SetResource(resource fyne.Resource) {
	if resource == nil {
		resource = icons.Globe
	}
	f.icon.SetResource(resource)
	f.Refresh()
}

func (f *iconPreview) SetFavicon(favicon *paw.Favicon) {
	if favicon == nil || len(favicon.Data) == 0 {
		f.SetResource(icons.Globe)
		return
	}
	f.SetResource(favicon)
}

func (f *iconPreview) CreateRenderer() fyne.WidgetRenderer {
	content := container.NewStack(f.background, container.NewCenter(f.icon))
	return &iconPreviewRenderer{WidgetRenderer: widget.NewSimpleRenderer(content), preview: f}
}

type iconPreviewRenderer struct {
	fyne.WidgetRenderer
	preview *iconPreview
}

func (r *iconPreviewRenderer) Refresh() {
	r.preview.background.FillColor = theme.Color(theme.ColorNameInputBackground)
	r.preview.background.CornerRadius = theme.Size(theme.SizeNameButtonRadius)
	r.preview.background.Refresh()
	r.WidgetRenderer.Refresh()
}
