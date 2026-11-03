// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package icon

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed paw.png
var pawIconPNG []byte

// PawIcon is Paw's high-resolution application icon.
var PawIcon = &fyne.StaticResource{
	StaticName:    "paw.png",
	StaticContent: pawIconPNG,
}
