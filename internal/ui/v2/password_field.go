// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package v2

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
	content   *fyne.Container
}

// NewPasswordField creates an empty field with hidden strength feedback.
func NewPasswordField() *PasswordField {
	f := &PasswordField{Entry: widget.NewPasswordEntry(), strength: NewPasswordStrengthIndicator()}
	f.content = container.NewVBox(f.Entry, f.strength)
	f.ExtendBaseWidget(f)
	f.Entry.OnChanged = f.changed
	return f
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
