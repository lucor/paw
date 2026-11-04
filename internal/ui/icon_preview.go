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
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// iconPreview frames a favicon or entry icon with a theme-aware background.
type iconPreview struct {
	widget.BaseWidget
	background *canvas.Rectangle
	icon       *widget.Icon
	content    *fyne.Container
	iconSize   float32
	favicon    bool
}

func newIconPreview(resource fyne.Resource) *iconPreview {
	return newIconPreviewWithIconSize(resource, pawSize(pawSizeNameDetailIcon))
}

func newIconPreviewWithIconSize(resource fyne.Resource, iconSize float32) *iconPreview {
	if resource == nil {
		resource = icons.Globe
	}
	preview := &iconPreview{
		background: canvas.NewRectangle(color.Transparent),
		icon:       widget.NewIcon(resource),
		iconSize:   iconSize,
		favicon:    isDownloadedFavicon(resource),
	}
	preview.ExtendBaseWidget(preview)
	return preview
}

func (f *iconPreview) SetResource(resource fyne.Resource) {
	if resource == nil {
		resource = icons.Globe
	}
	f.favicon = isDownloadedFavicon(resource)
	f.icon.SetResource(resource)
	if f.content != nil {
		f.content.Objects[1] = f.iconObject()
		f.content.Refresh()
	}
	f.Refresh()
}

func (f *iconPreview) SetFavicon(favicon *model.Favicon) {
	if favicon == nil || len(favicon.Data) == 0 {
		f.SetResource(icons.Globe)
		return
	}
	f.SetResource(favicon)
}

func isDownloadedFavicon(resource fyne.Resource) bool {
	favicon, ok := resource.(*model.Favicon)
	return ok && len(favicon.Data) > 0
}

func (f *iconPreview) CreateRenderer() fyne.WidgetRenderer {
	f.content = container.NewStack(f.background, f.iconObject())
	return &iconPreviewRenderer{WidgetRenderer: widget.NewSimpleRenderer(f.content), preview: f}
}

func (f *iconPreview) iconObject() fyne.CanvasObject {
	return container.NewCenter(container.NewGridWrap(fyne.NewSquareSize(f.iconSize), f.icon))
}

type iconPreviewRenderer struct {
	fyne.WidgetRenderer
	preview *iconPreview
}

func (r *iconPreviewRenderer) Refresh() {
	if r.preview.favicon {
		r.preview.background.FillColor = theme.Color(pawColorNameFaviconBackground)
	} else {
		r.preview.background.FillColor = theme.Color(theme.ColorNameInputBackground)
	}
	r.preview.background.CornerRadius = theme.Size(theme.SizeNameButtonRadius)
	r.preview.background.Refresh()
	r.WidgetRenderer.Refresh()
}
