// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

const (
	numericStepperButtonSize = 32
	numericStepperValueWidth = 40
)

func newNumericStepper(value *int, min, max, step int, changed func()) fyne.CanvasObject {
	label := newText(strconv.Itoa(*value), bodyStyle)
	label.TextStyle.Monospace = true
	label.Alignment = fyne.TextAlignCenter
	minus := widget.NewButtonWithIcon("", icons.Minus, nil)
	plus := widget.NewButtonWithIcon("", icons.Plus, nil)
	update := func() {
		label.SetText(strconv.Itoa(*value))
		minus.Enable()
		plus.Enable()
		if *value <= min {
			minus.Disable()
		}
		if *value >= max {
			plus.Disable()
		}
	}
	minus.OnTapped = func() {
		if *value > min {
			*value -= step
			if *value < min {
				*value = min
			}
			update()
			if changed != nil {
				changed()
			}
		}
	}
	plus.OnTapped = func() {
		if *value < max {
			*value += step
			if *value > max {
				*value = max
			}
			update()
			if changed != nil {
				changed()
			}
		}
	}
	update()
	buttonSize := fyne.NewSquareSize(numericStepperButtonSize)
	valueSize := fyne.NewSize(numericStepperValueWidth, numericStepperButtonSize)
	return container.NewHBox(
		container.NewGridWrap(buttonSize, minus),
		container.NewGridWrap(valueSize, label),
		container.NewGridWrap(buttonSize, plus),
	)
}
