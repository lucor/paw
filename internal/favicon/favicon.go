// Copyright 2022 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package favicon provides a favicon downloader
package favicon

import (
	"context"
	"fmt"
	"image"
	"image/draw"
	"net/http"
	"sort"
	"sync"
	"time"

	// Imports required to decode favicon images
	_ "image/png"

	_ "golang.org/x/image/bmp"
	_ "lucor.dev/paw/internal/ico"
)

const (
	defaultMinSize = 32
)

type Service func(host string) string

type Options struct {
	// Client is http.Client used to download the favicon. Leave nil to use the
	// http.Default with a timeout of 10 seconds
	Client *http.Client
	// MinSize is the min size of the favicon to be considered valid.
	// favicon smaller than minSize will be ignored unless ForceMinSize is true
	MinSize int
	// ForceMinSize when true will force to return a favicon even if its size is
	// smaller than MinSize
	ForceMinSize bool
	Service      Service
}

// Download tries to download the favicon with highest resolution for the specified host
// By default it looks into the standard locations
// - http://<host>/apple-touch-icon.png
// - http://<host>/favicon.ico
// - http://<host>/favicon.png
// Alternatively a third-party service can be used via the Service option.
// Example:
// // Use the DuckDuckGo service
//
//	ddg := func(host string) string  {
//		return fmt.Sprintf("https://icons.duckduckgo.com/ip3/%s.ico", host)
//	}
//	img, err := favicon.Download(e.ctx, host, favicon.Options{
//		ForceMinSize: true,
//		Service: ddg,
//	})
func Download(ctx context.Context, host string, opts Options) (image.Image, error) {
	minSize := opts.MinSize
	forceMinSize := opts.ForceMinSize
	if minSize <= 0 {
		minSize = defaultMinSize
	}
	client := opts.Client
	if client == nil {
		client = http.DefaultClient
		client.Timeout = 10 * time.Second
	}

	urls := []string{
		fmt.Sprintf("http://%s/apple-touch-icon.png", host),
		fmt.Sprintf("http://%s/favicon.ico", host),
		fmt.Sprintf("http://%s/favicon.png", host),
	}

	service := opts.Service
	if service != nil {
		urls = []string{service(host)}
	}

	var result []image.Image
	wg := &sync.WaitGroup{}
	for _, url := range urls {
		url := url
		wg.Add(1)
		go func() {
			defer wg.Done()
			img, err := download(ctx, client, url)
			if err != nil {
				return
			}
			result = append(result, img)
		}()
	}
	wg.Wait()

	if result == nil {
		return nil, fmt.Errorf("could not found any favicon at default locations")
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Bounds().Dx() > result[j].Bounds().Dx()
	})

	var out image.Image
	out = result[0]
	if minSize > out.Bounds().Dx() {
		if !forceMinSize {
			return nil, fmt.Errorf("min size required %dx%d, found icon %dx%d", minSize, minSize, out.Bounds().Dx(), out.Bounds().Dy())
		}
		m := image.NewRGBA(image.Rect(0, 0, minSize, minSize))

		p := (minSize - out.Bounds().Dx()) / 2
		dp := image.Point{p, p}

		draw.Draw(m, m.Bounds(), image.Transparent, image.Point{}, draw.Src)
		r := image.Rectangle{dp, dp.Add(out.Bounds().Size())}
		draw.Draw(m, r, out, out.Bounds().Min, draw.Src)
		out = m
	}

	return out, nil
}

func download(ctx context.Context, client *http.Client, url string) (image.Image, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create HTTP request: %w", err)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not fetch favicon: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("could not fetch favicon: %s", res.Status)
	}
	defer res.Body.Close()
	img, _, err := image.Decode(res.Body)
	return img, err
}
