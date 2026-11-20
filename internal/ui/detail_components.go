// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"strings"
	"time"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

const detailTimestampLayout = "2 Jan 2006, 15:04:05"

const multilineThreshold = 160

const detailPreviewMaxRunes = 120

const detailPreviewMaxLines = 5

// isLongText detects values that should open in the full-value view.
func isLongText(value string) bool {
	return strings.Contains(value, "\n") || utf8.RuneCountInString(value) > multilineThreshold
}

// detailTextPreview limits long values to a compact preview.
func detailTextPreview(value string) (string, bool) {
	if !isLongText(value) {
		return value, false
	}
	runes := []rune(value)
	if len(runes) > detailPreviewMaxRunes {
		return string(runes[:detailPreviewMaxRunes]) + "…", true
	}
	lines := 1
	for index, r := range runes {
		if r == '\n' {
			lines++
			if lines > detailPreviewMaxLines {
				return string(runes[:index]) + "…", true
			}
		}
	}
	return value, false
}

// newDetailTextPreview presents a value preview and an optional Show more action.
func newDetailTextPreview(value string, showMore func()) fyne.CanvasObject {
	preview, truncated := detailTextPreview(value)
	text := newText(preserveLineEdgeSpaces(preview), bodyStyle)
	text.Wrapping = fyne.TextWrapBreak
	text.Selectable = true
	if !truncated {
		return text
	}
	button := widget.NewButton("Show more", showMore)
	button.Importance = widget.LowImportance
	return container.NewVBox(text, button)
}

// formatDetailTimestamp formats a stored Unix timestamp for item details.
func formatDetailTimestamp(timestamp uint64) string {
	return time.Unix(int64(timestamp), 0).Local().Format(detailTimestampLayout)
}

// newPageHeader aligns navigation, title, and trailing actions consistently.
func newPageHeader(back, title, trailing fyne.CanvasObject) fyne.CanvasObject {
	return container.NewBorder(nil, nil, back, trailing, title)
}

// newSecondaryPage places fixed controls around the page body.
func newSecondaryPage(header, footer, content fyne.CanvasObject) fyne.CanvasObject {
	return container.NewBorder(header, footer, nil, nil, content)
}

// newDetailField places a caption above a read-only value and its actions.
func newDetailField(label string, value fyne.CanvasObject, actions ...fyne.CanvasObject) fyne.CanvasObject {
	return newDetailFieldWithActions(label, value, nil, actions)
}

// newDetailFieldWithActions aligns separate actions with a field label and value.
func newDetailFieldWithActions(label string, value fyne.CanvasObject, labelActions, valueActions []fyne.CanvasObject) fyne.CanvasObject {
	labelText := fyne.CanvasObject(newText(label, sectionTitleStyle))
	if len(labelActions) > 0 {
		labelText = container.New(layout.NewCustomPaddedVBoxLayout(0), layout.NewSpacer(), labelText)
	}
	header := container.NewBorder(nil, nil, labelText, detailActions(labelActions...), nil)
	if value == nil {
		return header
	}
	var actions fyne.CanvasObject
	if len(valueActions) > 0 {
		actions = container.NewVBox(detailActions(valueActions...), layout.NewSpacer())
	}
	valueRow := container.NewBorder(nil, nil, nil, actions, value)
	return container.New(layout.NewCustomPaddedVBoxLayout(0), header, valueRow)
}

// newCustomFieldsSectionCard gives custom-field sections a light border and plain body.
func newCustomFieldsSectionCard(content fyne.CanvasObject) fyne.CanvasObject {
	background := canvas.NewRectangle(theme.Color(theme.ColorNameBackground))
	background.StrokeColor = theme.Color(theme.ColorNameInputBackground)
	background.StrokeWidth = 1
	background.CornerRadius = theme.Size(theme.SizeNamePadding)
	return container.NewStack(background, content)
}

// newDetailMultilineField places actions beside the top of wrapped content.
func newDetailMultilineField(label string, value fyne.CanvasObject, actions ...fyne.CanvasObject) fyne.CanvasObject {
	return newDetailFieldWithActions(label, value, nil, actions)
}

// newDetailFieldHeader aligns a field caption with its right-side actions.
func newDetailFieldHeader(label string, actions ...fyne.CanvasObject) fyne.CanvasObject {
	return container.NewBorder(nil, nil, newText(label, sectionTitleStyle), detailActions(actions...), nil)
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
	size := pawSize(pawSizeNameDetailIconArea)
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

// detailFullValueView displays a complete long value without editing it.
type detailFullValueView struct {
	View  fyne.CanvasObject
	value string
	text  *widget.Label
}

// newDetailFullValueView builds a scrollable read-only value page.
func newDetailFullValueView(title, value string, monospace bool, back func(), download func(string)) *detailFullValueView {
	view := &detailFullValueView{value: value}
	view.text = newText(preserveLineEdgeSpaces(value), bodyStyle)
	view.text.Wrapping = fyne.TextWrapBreak
	view.text.Selectable = true
	view.text.TextStyle.Monospace = monospace
	copy := newIconButton("Copy "+title, icons.Copy, func() { detailCopy(title, view.value) })
	actions := []fyne.CanvasObject{copy}
	if download != nil {
		actions = append(actions, newIconButton("Download "+title, icons.Download, func() { download(view.value) }))
	}
	header := newPageHeader(newIconButton("Back", icons.Back, back), newText(title, titleStyle), detailActions(actions...))
	view.View = newSecondaryPage(header, nil, container.NewVScroll(view.text))
	return view
}

// Clear removes the full value from the view.
func (v *detailFullValueView) Clear() {
	v.value = ""
	v.text.SetText("")
}

// detailSecretField builds a masked value with explicit Reveal and Copy actions.
func detailSecretField(label, value string) (fyne.CanvasObject, *detailSecret) {
	if value == "" {
		return nil, nil
	}
	displayLabel := label != ""
	if label == "" {
		label = "value"
	}
	secret := &detailSecret{value: value, label: label, mask: "••••••••••••"}
	secret.text = newText(secret.mask, bodyStyle)
	secret.text.Wrapping = fyne.TextWrapBreak
	show := newIconButton("Show "+label, icons.Show, nil)
	secret.button = show
	show.OnTapped = secret.Toggle
	copy := newIconButton("Copy "+label, icons.Copy, func() { detailCopy(label, secret.value) })
	var field fyne.CanvasObject
	if displayLabel {
		field = newDetailField(label, secret.text, show, copy)
	} else {
		field = container.NewBorder(nil, nil, nil, detailActions(show, copy), secret.text)
	}
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
	if s.text != nil {
		s.text.SetText(s.mask)
	}
	if s.surface != nil {
		s.surface.SetContent(nil)
		s.surface.Hide()
	}
	if s.button != nil {
		s.button.SetIcon(icons.Show)
		s.button.label = "Show " + s.label
		s.button.Refresh()
	}
}

// clearDetailSecrets removes sensitive values from a detail view.
func clearDetailSecrets(secrets []*detailSecret) {
	for _, secret := range secrets {
		secret.Clear()
	}
}
