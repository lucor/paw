// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package favicon provides a favicon downloader
package favicon

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"image"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	// Imports required to decode favicon images
	"image/jpeg"
	"image/png"

	"golang.org/x/image/draw"

	"github.com/fyne-io/image/ico"

	"lucor.dev/paw/internal/paw"
)

const (
	defaultMinSize    = 32
	httpClientTimeout = 5 * time.Second
	sniffDataLen      = 512
)

type Options struct {
	// Client is http.Client used to download the favicon. Leave nil to use the
	// http.Default with a timeout of 10 seconds
	Client *http.Client
	// MinSize is the min size of the favicon to be considered valid.
	// favicon smaller than minSize will be ignored unless ForceMinSize is true
	MinSize int
}

// Download tries to download the favicon with highest resolution for the specified host
// By default it looks into the standard locations
func Download(ctx context.Context, u *url.URL, opts Options) ([]byte, string, error) {
	minSize := opts.MinSize
	if minSize <= 0 {
		minSize = defaultMinSize
	}
	client := opts.Client
	if client == nil {
		client = http.DefaultClient
		client.Timeout = httpClientTimeout
	}

	if u.Scheme == "" {
		u.Scheme = "https"
	}

	faviconData, err := findFavicon(ctx, client, u)
	if err != nil {
		return nil, "", fmt.Errorf("not found favicon in HTML")
	}
	faviconURL := faviconData.MakeRawURL(u)
	return downloadFavicon(ctx, client, faviconURL)
}

func downloadFavicon(ctx context.Context, client *http.Client, faviconURL string) ([]byte, string, error) {

	res, err := makeRequest(ctx, client, faviconURL)
	if err != nil {
		return nil, "", fmt.Errorf("download: %w", err)
	}
	defer res.Body.Close()

	r := bufio.NewReader(res.Body)

	resContentType := res.Header.Get("Content-Type")

	if resContentType == "image/svg+xml" {
		b, err := io.ReadAll(r)
		return b, "svg", err
	}

	// content-type could be not set or misleading
	// try to sniff the data
	detectedContentType := resContentType
	sniffData, err := r.Peek(sniffDataLen)
	if err == nil {
		detectedContentType = http.DetectContentType(sniffData)
	}

	var img image.Image
	switch detectedContentType {
	case "image/svg+xml":
		b, err := io.ReadAll(r)
		return b, "svg", err
	case "image/x-icon", "image/vnd.microsoft.icon":
		images, err := ico.DecodeAll(r)
		if err == nil {
			img = findBetterSizeFromImages(images)
		}
	case "image/png":
		img, err = png.Decode(r)
	case "image/jpeg":
		// This is a rare case, but some sites provides JPEG
		img, err = jpeg.Decode(r)
	default:
		err = fmt.Errorf("unsupported content type %s detected as %s", res.Header.Get("Content-Type"), detectedContentType)
	}

	if err != nil {
		return nil, "", fmt.Errorf("could not decode favicon: %w", err)
	}

	img = resize(img, defaultMinSize)

	buf := bytes.Buffer{}
	err = png.Encode(&buf, img)
	return buf.Bytes(), "png", err
}

func resize(img image.Image, wantSize int) image.Image {
	bounds := img.Bounds()
	if wantSize > bounds.Dx() {
		m := image.NewRGBA(image.Rect(0, 0, wantSize, wantSize))

		p := (wantSize - bounds.Dx()) / 2
		dp := image.Point{p, p}

		draw.Draw(m, m.Bounds(), image.Transparent, image.Point{}, draw.Src)
		r := image.Rectangle{dp, dp.Add(bounds.Size())}
		draw.Draw(m, r, img, bounds.Min, draw.Src)
		img = m
	} else if wantSize < img.Bounds().Dx() {
		m := image.NewRGBA(image.Rect(0, 0, wantSize, wantSize))
		draw.BiLinear.Scale(m, m.Bounds(), img, bounds, draw.Over, nil)
		img = m
	}
	return img
}

func makeRequest(ctx context.Context, client *http.Client, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create HTTP request: %w", err)
	}
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", paw.UserAgentFaviconDownloader)
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not fetch URL: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("invalid response code: %s", res.Status)
	}
	return res, nil
}

func findFavicon(ctx context.Context, client *http.Client, u *url.URL) (*faviconData, error) {
	res, err := makeRequest(ctx, client, u.String())
	if err != nil {
		return nil, fmt.Errorf("could not fetch URL: %w", err)
	}
	defer res.Body.Close()

	favicons := parseHeadSection(res.Body)
	if len(favicons) == 0 {
		fallback := &faviconData{
			href: "/favicon.ico",
			rel:  "icon",
			size: 0,
			mime: "image/x-icon",
		}
		return fallback, nil
	}

	return findBetterSize(favicons), nil
}

func parseHeadSection(r io.Reader) []*faviconData {

	// Create a new tokenizer from the HTML content
	tokenizer := html.NewTokenizer(r)

	favicons := make([]*faviconData, 0)
	// Iterate through the tokens

	for {
		tokenType := tokenizer.Next()

		switch tokenType {
		case html.ErrorToken:
			return favicons
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			if token.Data == "link" {
				faviconData := getFaviconData(token)
				if faviconData != nil {
					favicons = append(favicons, faviconData)
				}
			}
		case html.EndTagToken:
			token := tokenizer.Token()
			if token.Data == "head" {
				return favicons
			}
		}
	}
}

type faviconData struct {
	href string
	rel  string
	size int
	mime string
}

func (f *faviconData) MakeRawURL(u *url.URL) string {
	href := f.href
	if strings.HasPrefix(href, "//") {
		// protocol-relative url
		return fmt.Sprintf("%s:%s", u.Scheme, href)
	}
	if strings.HasPrefix(href, "/") {
		// relative url
		return fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, href)
	}
	return href
}

// getFaviconData checks if the given token is a <link> element with rel
// containg "icon" and returns its href value
func getFaviconData(token html.Token) *faviconData {
	var rel string
	var href string
	var mime string
	var size int

Loop:
	for _, attr := range token.Attr {
		if attr.Key == "rel" {
			switch attr.Val {
			case "icon", "shortcut icon", "apple-touch-icon":
				rel = attr.Val
			default:
				break Loop
			}
		} else if attr.Key == "href" {
			href = attr.Val
		} else if attr.Key == "sizes" {
			v, _, found := strings.Cut(attr.Val, "x")
			if !found {
				continue
			}
			var err error
			size, err = strconv.Atoi(v)
			if err != nil {
				continue
			}
		} else if attr.Key == "type" {
			mime = attr.Val
		}
	}

	if rel == "" {
		return nil
	}

	return &faviconData{
		rel:  rel,
		href: href,
		mime: mime,
		size: size,
	}
}

// findBetterSize finds the closest number to the target in the given slice of integers
func findBetterSize(favicons []*faviconData) *faviconData {
	closestIdx := 0
	smallestDifference := math.Abs(float64(defaultMinSize - favicons[0].size))

	for idx := 1; idx < len(favicons); idx++ {
		f := favicons[idx]
		diff := math.Abs(float64(defaultMinSize - f.size))
		if diff < smallestDifference {
			closestIdx = idx
			smallestDifference = diff
		}
	}
	return favicons[closestIdx]
}

// findBetterSize finds the closest number to the target in the given slice of integers
func findBetterSizeFromImages(images []image.Image) image.Image {
	closestIdx := 0
	smallestDifference := math.Abs(float64(defaultMinSize - images[0].Bounds().Dx()))

	for idx := 1; idx < len(images); idx++ {
		bounds := images[idx].Bounds()
		diff := math.Abs(float64(defaultMinSize - bounds.Dx()))
		if diff < smallestDifference {
			closestIdx = idx
			smallestDifference = diff
		}
	}
	return images[closestIdx]
}
