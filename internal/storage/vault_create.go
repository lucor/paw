// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"bytes"
	"fmt"

	"lucor.dev/paw/internal/model"
	"lucor.dev/paw/internal/paw"
)

// prepareVault creates a CXF vault and its password-encrypted key contents.
func prepareVault(name, password string) (*model.Vault, []byte, error) {
	var keyData bytes.Buffer
	key, err := paw.MakeKey(password, &keyData)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create vault key: %w", err)
	}
	vault, err := model.NewVault(key, name)
	if err != nil {
		return nil, nil, err
	}
	return vault, keyData.Bytes(), nil
}
