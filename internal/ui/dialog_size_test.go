// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"github.com/stretchr/testify/require"
)

func TestSizeCompactDialog(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		width float32
	}{
		{name: "narrow", width: 320},
		{name: "android sized", width: 430},
		{name: "desktop", width: 1200},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			minimum := fyne.NewSize(220, 96)
			windowSize := fyne.NewSize(testCase.width, 700)
			size := compactDialogSize(minimum, windowSize)
			wantWidth := float32(math.Min(float64(compactDialogMaxWidth), float64(testCase.width-2*compactDialogSideMargin)))
			if wantWidth < minimum.Width {
				wantWidth = minimum.Width
			}
			require.Equal(t, wantWidth, size.Width)
			require.Equal(t, minimum.Height, size.Height, "sizing should preserve content-driven height")
		})
	}

	t.Run("minimum content size", func(t *testing.T) {
		minimum := fyne.NewSize(300, 140)
		size := compactDialogSize(minimum, fyne.NewSize(320, 600))
		require.Equal(t, minimum, size, "dialog content minimum should take precedence on a narrow window")
	})
}
