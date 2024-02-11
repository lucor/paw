// Copyright 2023 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package widget

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	fynewidget "fyne.io/fyne/v2/widget"
)

type passwordRevealer struct {
	fynewidget.BaseWidget

	button     *fynewidget.Button
	label      *fynewidget.Label
	password   string
	obfuscated string
	revealed   bool
}

func NewPasswordRevealer(password string) fyne.Widget {
	obfuscated := strings.Repeat("*", len(password))
	pr := &passwordRevealer{
		button:     fynewidget.NewButtonWithIcon("", theme.VisibilityOffIcon(), func() {}),
		label:      fynewidget.NewLabel(obfuscated),
		password:   password,
		obfuscated: obfuscated,
	}
	pr.button.OnTapped = pr.reveal
	pr.ExtendBaseWidget(pr)
	return pr
}

func (r *passwordRevealer) CreateRenderer() fyne.WidgetRenderer {
	return fynewidget.NewSimpleRenderer(container.NewBorder(nil, nil, nil, r.button, container.NewHScroll(r.label)))
}

func (r *passwordRevealer) Tapped(*fyne.PointEvent) {
	r.reveal()
}

func (r *passwordRevealer) reveal() {
	r.revealed = !r.revealed
	if r.revealed {
		r.button.SetIcon(theme.VisibilityIcon())
		r.label.SetText(r.password)
		return
	}
	r.button.SetIcon(theme.VisibilityOffIcon())
	r.label.SetText(r.obfuscated)
}
