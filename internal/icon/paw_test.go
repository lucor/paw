// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package icon

import (
	"bytes"
	"image/png"
	"os"
	"testing"
)

const pawIconSize = 1024

func TestPawIconMatchesHighResolutionPackageAsset(t *testing.T) {
	packageIcon, err := os.ReadFile("../../logo/paw.png")
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(packageIcon, PawIcon.Content()) {
		t.Fatal("embedded application icon differs from the package icon")
	}

	config, err := png.DecodeConfig(bytes.NewReader(packageIcon))
	if err != nil {
		t.Fatal(err)
	}
	if config.Width != pawIconSize || config.Height != pawIconSize {
		t.Fatalf("package icon size is %dx%d, want %dx%d", config.Width, config.Height, pawIconSize, pawIconSize)
	}
}
