// Copyright 2022 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package paw

import (
	"encoding/hex"
	"time"

	"golang.org/x/crypto/blake2b"
)

// Item represents the basic paw identity
type Metadata struct {
	// Title reprents the item name
	Name string `json:"name,omitempty"`
	// Type represents the item type
	Type ItemType `json:"type,omitempty"`
	// Modified holds the modification date
	Modified time.Time `json:"modified,omitempty"`
	// Created holds the creation date
	Created time.Time `json:"created,omitempty"`
	// Icon
	Favicon *Favicon `json:"favicon,omitempty"`
}

func (m *Metadata) ID() string {
	key := append([]byte(m.Type.String()), []byte(m.Name)...)
	hash, err := blake2b.New256(key)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func (m *Metadata) GetMetadata() *Metadata {
	return m
}

func (m *Metadata) IsEmpty() bool {
	return m.Name == ""
}

func (m *Metadata) String() string {
	return m.Name
}

// ByID implements sort.Interface Metadata on the ID value.
type ByString []*Metadata

func (s ByString) Len() int { return len(s) }
func (s ByString) Less(i, j int) bool {
	return s[i].String() < s[j].String()
}
func (s ByString) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
