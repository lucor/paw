// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later


// Package haveibeenpwned implements a client for the haveibeenpwned.com API v3
// to search if passwords have been exposed in data breaches
package haveibeenpwned

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"lucor.dev/paw/internal/paw"
)

const apiURL = "https://api.pwnedpasswords.com/range/%s"

var defaultClient = &http.Client{
	Timeout: 10 * time.Second,
}

// httpClient interface
type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type Pwned struct {
	Item  paw.Item
	Count int
}

// Search searches if the password has been exposed in data
// breaches using the Have I Been Pwned APIs
func Search(ctx context.Context, item paw.Item) (pwned bool, count int, err error) {
	meta := item.GetMetadata()
	if meta == nil {
		return pwned, count, fmt.Errorf("metadata cannot be nil")
	}
	var p string
	switch meta.Type {
	case paw.PasswordItemType:
		p = item.(*paw.Password).Value
	case paw.LoginItemType:
		p = item.(*paw.Login).Password.Value
	default:
		return pwned, count, fmt.Errorf("invalid item type %q", meta.Type)
	}
	return hibp(ctx, defaultClient, p)
}

// hibp consumes the range endpoint. It returns true if the provided password has been
// exposed in data breaches along with a count of how many times it appears in the data set.
// See https://haveibeenpwned.com/API/v3#PwnedPasswords
func hibp(ctx context.Context, c httpClient, password string) (bool, int, error) {
	// The HIBP range endpoint takes the first 5 chars of the SHA1(password) as
	// input and returns the suffix of every hash beginning with the specified
	// prefix, followed by a count of how many times it appears in the data set.
	h := sha1.New()
	io.WriteString(h, password)

	// password hash encoded as hex
	ph := make([]byte, 40)
	hex.Encode(ph, h.Sum(nil))

	// make uppercase to compare with API response hashes
	phu := bytes.ToUpper(ph)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf(apiURL, phu[0:5]), nil)
	if err != nil {
		return false, 0, err
	}

	// Enable padding to enhance privacy
	req.Header.Add("Add-Padding", "true")
	resp, err := c.Do(req)
	if err != nil {
		return false, 0, err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if bytes.Equal(phu[5:], line[0:35]) {
			count, err := strconv.Atoi(string(line[36:]))
			return true, count, err
		}
	}

	if err := scanner.Err(); err != nil {
		return false, 0, err
	}

	return false, 0, nil
}
