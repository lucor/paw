// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package widgets

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
)

func TestBadgeMinSizeIncludesVerticalPadding(t *testing.T) {
	test.NewTempApp(t)

	foreground := color.NRGBA{R: 20, G: 30, B: 40, A: 255}
	badge := NewBadge("3 saved", color.NRGBA{}, foreground)

	text := canvas.NewText(badge.Text, foreground)
	text.TextSize = badge.TextSize
	wantHeight := text.MinSize().Height + 2*badge.PaddingY
	if got := badge.MinSize().Height; got != wantHeight {
		t.Fatalf("MinSize().Height = %v, want %v", got, wantHeight)
	}
}

func TestBadgeTextUpdateChangesMinimumWidth(t *testing.T) {
	test.NewTempApp(t)

	badge := NewBadge("1 saved", color.NRGBA{}, color.NRGBA{A: 255})
	initialWidth := badge.MinSize().Width
	badge.SetText("100 saved")

	if got := badge.MinSize().Width; got <= initialWidth {
		t.Fatalf("MinSize().Width after SetText() = %v, want greater than %v", got, initialWidth)
	}
}

func TestBadgeDoesNotStretchToAllocatedContainer(t *testing.T) {
	test.NewTempApp(t)
	badge := NewBadge("2 saved", color.NRGBA{}, color.NRGBA{A: 255})
	renderer := badge.CreateRenderer().(*badgeRenderer)
	minSize := renderer.MinSize()
	allocated := fyne.NewSize(minSize.Width+24, minSize.Height+24)

	renderer.Layout(allocated)

	if got := renderer.background.Size(); got != minSize {
		t.Fatalf("background size = %v, want badge minimum size %v", got, minSize)
	}
	wantPosition := fyne.NewPos(12, 12)
	if got := renderer.background.Position(); got != wantPosition {
		t.Fatalf("background position = %v, want centered at %v", got, wantPosition)
	}
}
