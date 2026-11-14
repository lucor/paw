// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// ConnectionLink identifies a Sync listener and its expected identity.
type ConnectionLink struct {
	Host        string
	Port        int
	Fingerprint string
}

// ParseConnectionLink validates a paw:// Sync connection link.
func ParseConnectionLink(value string) (ConnectionLink, error) {
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "paw" || u.Opaque != "" || u.User != nil || u.Path != "/sync" || u.Fragment != "" {
		return ConnectionLink{}, fmt.Errorf("invalid Sync connection link")
	}
	if len(u.Query()) != 1 || len(u.Query()["fp"]) != 1 {
		return ConnectionLink{}, fmt.Errorf("invalid Sync connection link")
	}
	host, portText, err := net.SplitHostPort(u.Host)
	if err != nil || host == "" {
		return ConnectionLink{}, fmt.Errorf("invalid Sync connection endpoint")
	}
	if ip := net.ParseIP(host); ip == nil && (strings.ContainsAny(host, " %:/\\") || strings.Trim(host, ".") != host) {
		return ConnectionLink{}, fmt.Errorf("invalid Sync connection endpoint")
	}
	port, err := strconv.Atoi(portText)
	fingerprint := u.Query().Get("fp")
	decoded, fpErr := hex.DecodeString(fingerprint)
	if err != nil || port < 1 || port > 65535 || fpErr != nil || len(decoded) != 32 || fingerprint != strings.ToLower(fingerprint) {
		return ConnectionLink{}, fmt.Errorf("invalid Sync connection link")
	}
	return ConnectionLink{Host: host, Port: port, Fingerprint: fingerprint}, nil
}

// String formats the connection link using the canonical URI form.
func (l ConnectionLink) String() string {
	return "paw://" + net.JoinHostPort(l.Host, strconv.Itoa(l.Port)) + "/sync?fp=" + l.Fingerprint
}
