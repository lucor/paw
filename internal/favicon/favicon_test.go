// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package favicon

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDownloadResolvesRelativeFaviconURLs checks links resolve against the page URL.
func TestDownloadResolvesRelativeFaviconURLs(t *testing.T) {
	var iconData bytes.Buffer
	iconImage := image.NewRGBA(image.Rect(0, 0, 16, 16))
	iconImage.Set(8, 8, color.RGBA{R: 0xff, A: 0xff})
	require.NoError(t, png.Encode(&iconData, iconImage))

	var requestedPaths []string
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestedPaths = append(requestedPaths, request.URL.Path)
		var body []byte
		header := make(http.Header)
		switch request.URL.Path {
		case "/account/login":
			header.Set("Content-Type", "text/html")
			body = []byte(`<html><head><link rel="icon" href="assets/favicon.png"></head></html>`)
		case "/account/assets/favicon.png":
			header.Set("Content-Type", "image/png")
			body = iconData.Bytes()
		default:
			return nil, fmt.Errorf("unexpected request path %q", request.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(bytes.NewReader(body)),
			Request:    request,
		}, nil
	})}

	pageURL, err := url.Parse("https://example.test/account/login")
	require.NoError(t, err)
	data, format, err := Download(context.Background(), pageURL, Options{Client: client})
	require.NoError(t, err)
	require.Equal(t, []string{"/account/login", "/account/assets/favicon.png"}, requestedPaths)
	require.Equal(t, "png", format)
	require.NotEmpty(t, data)
	_, err = png.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, image.Pt(storedRasterSize, storedRasterSize), image.Pt(config.Width, config.Height))
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}
