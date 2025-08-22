// SPDX-FileCopyrightText: 2024-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later


package paw

import "net/url"

type Autofill struct {
	AllowHTTP  bool     `json:"allow_http,omitempty"`
	TLDPlusOne string   `json:"tld_plus_one,omitempty"`
	URL        *url.URL `json:"url,omitempty"`
}
