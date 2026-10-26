// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package favicon

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDownloadResolvesRelativeFaviconURLs checks links resolve against the page URL.
func TestDownloadResolvesRelativeFaviconURLs(t *testing.T) {
	var iconData bytes.Buffer
	iconImage := image.NewRGBA(image.Rect(0, 0, 16, 16))
	iconImage.Set(8, 8, color.RGBA{R: 0xff, A: 0xff})
	require.NoError(t, png.Encode(&iconData, iconImage))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/account/login":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><head><link rel="icon" href="assets/favicon.png"></head></html>`))
		case "/account/assets/favicon.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(iconData.Bytes())
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	pageURL, err := http.NewRequest(http.MethodGet, server.URL+"/account/login", nil)
	require.NoError(t, err)
	parsedURL := pageURL.URL
	data, format, err := Download(context.Background(), parsedURL, Options{Client: server.Client()})
	require.NoError(t, err)
	require.Equal(t, "png", format)
	require.NotEmpty(t, data)
	_, err = png.Decode(bytes.NewReader(data))
	require.NoError(t, err)
}
