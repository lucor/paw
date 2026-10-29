// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/zxcvbn"
)

// PasswordField combines a native password entry with strength feedback.
type PasswordField struct {
	widget.BaseWidget
	// Entry provides native password controls and validation.
	Entry *widget.Entry
	// OnChanged receives the native entry event after strength feedback updates.
	OnChanged func(string)
	strength  *PasswordStrengthIndicator
	actions   *fyne.Container
	content   *fyne.Container
}

// NewPasswordField creates an empty field with hidden strength feedback.
func NewPasswordField() *PasswordField {
	f := &PasswordField{
		Entry:    widget.NewPasswordEntry(),
		strength: NewPasswordStrengthIndicator(),
		actions:  container.NewHBox(),
	}
	input := container.NewBorder(nil, nil, nil, f.actions, f.Entry)
	f.content = container.NewVBox(input, f.strength)
	f.ExtendBaseWidget(f)
	f.Entry.OnChanged = f.changed
	return f
}

// SetTrailingAction places an optional accessory beside the entry, above strength feedback.
func (f *PasswordField) SetTrailingAction(action fyne.CanvasObject) {
	f.actions.Objects = nil
	if action != nil {
		f.actions.Add(action)
	}
	f.actions.Refresh()
	f.Refresh()
}

func (f *PasswordField) changed(value string) {
	wasVisible := f.strength.Visible()
	if value == "" {
		f.strength.Clear()
	} else {
		f.strength.SetScore(zxcvbn.PasswordStrength(value, nil).Score)
	}
	if wasVisible != f.strength.Visible() {
		f.Refresh()
	}
	if f.OnChanged != nil {
		f.OnChanged(value)
	}
}

// CreateRenderer groups the native entry and indicator without an extra form row.
func (f *PasswordField) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(f.content)
}

// Validate forwards form validation to the native entry.
func (f *PasswordField) Validate() error { return f.Entry.Validate() }

// SetOnValidationChanged connects native validation events to the enclosing form.
func (f *PasswordField) SetOnValidationChanged(callback func(error)) {
	f.Entry.SetOnValidationChanged(callback)
}
