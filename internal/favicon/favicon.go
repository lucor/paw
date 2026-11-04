// SPDX-FileCopyrightText: 2022-2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package favicon adapts the shared favicon downloader for Paw's storage format.
package favicon

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/url"

	"golang.org/x/image/draw"

	sharedfavicon "go.lucor.dev/favicon"
	"go.lucor.dev/paw/internal/paw"
)

const storedRasterSize = 64

// Options configures the HTTP client used to download a favicon.
type Options struct {
	Client *http.Client
}

// Download retrieves a website favicon and returns Paw's stored image format.
func Download(ctx context.Context, u *url.URL, opts Options) ([]byte, string, error) {
	result, err := sharedfavicon.Download(ctx, u, sharedfavicon.Options{
		Client:    opts.Client,
		UserAgent: paw.UserAgentFaviconDownloader,
	})
	if err != nil {
		return nil, "", err
	}
	if result.Icon.SVG != nil {
		return result.Icon.SVG, "svg", nil
	}
	if result.Icon.Raster == nil {
		return nil, "", fmt.Errorf("downloaded favicon has no image data")
	}

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, resize(result.Icon.Raster, storedRasterSize)); err != nil {
		return nil, "", fmt.Errorf("encode resized favicon: %w", err)
	}
	return encoded.Bytes(), "png", nil
}

func resize(source image.Image, size int) image.Image {
	bounds := source.Bounds()
	destination := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(destination, destination.Bounds(), image.Transparent, image.Point{}, draw.Src)

	if bounds.Dx() <= size && bounds.Dy() <= size {
		origin := image.Pt((size-bounds.Dx())/2, (size-bounds.Dy())/2)
		draw.Draw(destination, image.Rectangle{Min: origin, Max: origin.Add(bounds.Size())}, source, bounds.Min, draw.Src)
		return destination
	}

	draw.BiLinear.Scale(destination, destination.Bounds(), source, bounds, draw.Over, nil)
	return destination
}
