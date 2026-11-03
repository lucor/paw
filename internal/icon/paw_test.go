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

func TestMacOSPackageIconHasPaddingAndOpaqueBlackMatte(t *testing.T) {
	data, err := os.ReadFile("../../logo/paw-macos.png")
	if err != nil {
		t.Fatal(err)
	}

	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if config.Width != pawIconSize || config.Height != pawIconSize {
		t.Fatalf("macOS package icon size is %dx%d, want %dx%d", config.Width, config.Height, pawIconSize, pawIconSize)
	}

	icon, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	assertPixel := func(x, y int, wantR, wantG, wantB uint32, description string) {
		t.Helper()
		r, g, b, a := icon.At(x, y).RGBA()
		if r != wantR || g != wantG || b != wantB || a != 0xffff {
			t.Fatalf("macOS icon %s at (%d, %d) is %#04x%#04x%#04x%#04x", description, x, y, r, g, b, a)
		}
	}
	assertPixel(0, 0, 0, 0, 0, "corner")
	assertPixel(10, pawIconSize/2, 0, 0, 0, "padding")
	assertPixel(70, pawIconSize/2, 0xffff, 0xffff, 0xffff, "logo edge")
}
