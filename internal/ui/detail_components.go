// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

const detailTimestampLayout = "2 Jan 2006, 15:04:05"

// formatDetailTimestamp formats a stored Unix timestamp for item details.
func formatDetailTimestamp(timestamp uint64) string {
	return time.Unix(int64(timestamp), 0).Local().Format(detailTimestampLayout)
}

// newPageHeader aligns navigation, title, and trailing actions consistently.
func newPageHeader(back, title, trailing fyne.CanvasObject) fyne.CanvasObject {
	return container.NewBorder(nil, nil, back, trailing, title)
}

// newSecondaryPage gives secondary views one shared header position and page margin.
func newSecondaryPage(header, footer, content fyne.CanvasObject) fyne.CanvasObject {
	page := container.NewBorder(header, footer, nil, nil, content)
	return container.New(layout.NewCustomPaddedLayout(24, 24, 24, 24), page)
}

// newDetailField places a caption above a read-only value and its actions.
func newDetailField(label string, value fyne.CanvasObject, actions ...fyne.CanvasObject) fyne.CanvasObject {
	return container.NewVBox(
		newText(label, captionStyle),
		container.NewBorder(nil, nil, nil, detailActions(actions...), value),
	)
}

// newDetailMultilineField places actions beside a caption above wrapped content.
func newDetailMultilineField(label string, value fyne.CanvasObject, actions ...fyne.CanvasObject) fyne.CanvasObject {
	return container.NewVBox(
		container.NewBorder(nil, nil, newText(label, captionStyle), detailActions(actions...), nil),
		value,
	)
}

// newDetailFieldHeader aligns a field caption with its right-side actions.
func newDetailFieldHeader(label string, actions ...fyne.CanvasObject) fyne.CanvasObject {
	return container.NewBorder(nil, nil, newText(label, captionStyle), detailActions(actions...), nil)
}

// detailActions keeps field actions in a consistent horizontal row.
func detailActions(actions ...fyne.CanvasObject) fyne.CanvasObject {
	return container.NewHBox(actions...)
}

// appendDetailSection separates each logical detail group with a native divider.
func appendDetailSection(sections *[]fyne.CanvasObject, section fyne.CanvasObject) {
	if section == nil {
		return
	}
	if len(*sections) > 0 {
		*sections = append(*sections, widget.NewSeparator())
	}
	*sections = append(*sections, section)
}

// newDetailScrollableContent keeps metadata at the viewport bottom for short entries.
func newDetailScrollableContent(sections []fyne.CanvasObject, metadata fyne.CanvasObject) fyne.CanvasObject {
	content := append([]fyne.CanvasObject(nil), sections...)
	if metadata != nil {
		content = append(content, layout.NewSpacer(), widget.NewSeparator(), metadata)
	}
	return container.New(layout.NewCustomPaddedVBoxLayout(theme.Size(theme.SizeNamePadding)), content...)
}

// newDetailMetadata keeps metadata rows together with only Fyne's line spacing.
func newDetailMetadata(rows []string) fyne.CanvasObject {
	return newText(strings.Join(rows, "\n"), captionStyle)
}

// newEntryIcon gives a favicon or entry icon the same themed compact frame.
func newEntryIcon(resource fyne.Resource) fyne.CanvasObject {
	return newEntryIconPreview(newIconPreview(resource))
}

// newEntryIconPreview gives a mutable icon preview the standard themed size.
func newEntryIconPreview(preview fyne.CanvasObject) fyne.CanvasObject {
	size := theme.Size(theme.SizeNameInlineIcon) * 2
	return container.NewGridWrap(fyne.NewSquareSize(size), preview)
}

// detailSecret keeps a credential and its reveal controls for lifecycle cleanup.
type detailSecret struct {
	value    string
	text     *widget.Label
	button   *iconButton
	label    string
	mask     string
	revealed bool
	surface  *widget.Card
}

// detailSecretField builds a masked value with explicit Reveal and Copy actions.
func detailSecretField(label, value string) (fyne.CanvasObject, *detailSecret) {
	if value == "" {
		return nil, nil
	}
	secret := &detailSecret{value: value, label: label, mask: "••••••••••••"}
	secret.text = newText(secret.mask, bodyStyle)
	secret.text.Wrapping = fyne.TextWrapBreak
	show := newIconButton("Show "+label, icons.Show, nil)
	secret.button = show
	show.OnTapped = secret.Toggle
	copy := newIconButton("Copy "+label, icons.Copy, func() { detailCopy(label, secret.value) })
	field := newDetailField(label, secret.text, show, copy)
	return field, secret
}

// Toggle changes the secret's visible state.
func (s *detailSecret) Toggle() {
	s.revealed = !s.revealed
	if s.revealed {
		s.text.SetText(s.value)
		if s.surface != nil {
			s.surface.SetContent(s.text)
			s.surface.Show()
		} else {
			s.text.Show()
		}
		s.button.SetIcon(icons.Hide)
		s.button.label = "Hide " + s.label
	} else {
		s.text.SetText(s.mask)
		if s.surface != nil {
			s.surface.SetContent(nil)
			s.surface.Hide()
		}
		s.button.SetIcon(icons.Show)
		s.button.label = "Show " + s.label
	}
	s.button.Refresh()
}

// Clear removes the credential from the view and resets its reveal controls.
func (s *detailSecret) Clear() {
	s.value = ""
	s.revealed = false
	s.text.SetText(s.mask)
	if s.surface != nil {
		s.surface.SetContent(nil)
		s.surface.Hide()
	}
	s.button.SetIcon(icons.Show)
	s.button.label = "Show " + s.label
	s.button.Refresh()
}

// clearDetailSecrets removes sensitive values from a detail view.
func clearDetailSecrets(secrets []*detailSecret) {
	for _, secret := range secrets {
		secret.Clear()
	}
}
