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
	"net/http"
	"strconv"
	"time"
)

const apiURL = "https://api.pwnedpasswords.com/range/%s"

var defaultClient = &http.Client{
	Timeout: 10 * time.Second,
}

// httpClient interface
type httpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client checks password hashes using the HIBP range endpoint.
type Client struct{}

// SearchHash checks a password's SHA-1 digest, sending only its five-character prefix.
func (Client) SearchHash(ctx context.Context, hash [sha1.Size]byte) (pwned bool, count int, err error) {
	return hibp(ctx, defaultClient, hash)
}

// hibp consumes the range endpoint. It returns true if the hashed password has been
// exposed in data breaches along with a count of how many times it appears in the data set.
// See https://haveibeenpwned.com/API/v3#PwnedPasswords
func hibp(ctx context.Context, c httpClient, hash [sha1.Size]byte) (bool, int, error) {
	// The HIBP range endpoint takes the first 5 chars of the SHA1(password) as
	// input and returns the suffix of every hash beginning with the specified
	// prefix, followed by a count of how many times it appears in the data set.
	// password hash encoded as hex
	ph := make([]byte, 40)
	hex.Encode(ph, hash[:])

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
	if resp.StatusCode != http.StatusOK {
		return false, 0, fmt.Errorf("breach lookup returned HTTP %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanned := false
	for scanner.Scan() {
		scanned = true
		line := scanner.Bytes()
		if len(line) < 37 || line[35] != ':' {
			return false, 0, fmt.Errorf("invalid breach lookup response")
		}
		if bytes.Equal(phu[5:], line[0:35]) {
			count, err := strconv.Atoi(string(line[36:]))
			return count > 0, count, err
		}
	}

	if err := scanner.Err(); err != nil {
		return false, 0, err
	}
	if !scanned {
		return false, 0, fmt.Errorf("empty breach lookup response")
	}

	return false, 0, nil
}
