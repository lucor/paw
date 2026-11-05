// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"flag"
	"fmt"
	"os"

	"go.lucor.dev/paw/internal/ui"
)

func main() {
	outputDir := flag.String("output", "", "directory for generated PNG files")
	flag.Parse()
	if err := run(*outputDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(outputDir string) error {
	return ui.GenerateScreenshotGallery(outputDir)
}
