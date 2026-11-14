// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package test provides shared helpers for Paw's test suites.
package test

import (
	"testing"

	"filippo.io/age"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

// VaultCreator creates a vault with a Paw key and saves it for password unlock.
type VaultCreator interface {
	CreateVaultWithKey(string, *paw.Key, ...model.VaultAppearance) (*model.Vault, error)
	SaveVaultKey(string, *paw.Key, *age.ScryptRecipient) error
	DeleteVault(string) error
}

// NewPasswordRecipient creates a low-cost age recipient for encrypted fixtures.
func NewPasswordRecipient(password string) (*age.ScryptRecipient, error) {
	recipient, err := age.NewScryptRecipient(password)
	if err != nil {
		return nil, err
	}
	recipient.SetWorkFactor(10)
	return recipient, nil
}

// CreateVault creates a test vault with a Paw key and no password wrapper.
func CreateVault(t testing.TB, storage VaultCreator, name string, appearances ...model.VaultAppearance) *model.Vault {
	t.Helper()
	key, err := paw.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	vault, err := storage.CreateVaultWithKey(name, key, appearances...)
	if err != nil {
		t.Fatal(err)
	}
	return vault
}

// CreateVaultWithPassword creates a test vault with a low-cost password wrapper.
func CreateVaultWithPassword(t testing.TB, storage VaultCreator, name, password string, appearances ...model.VaultAppearance) *model.Vault {
	t.Helper()
	vault := CreateVault(t, storage, name, appearances...)
	recipient, err := NewPasswordRecipient(password)
	if err != nil {
		_ = storage.DeleteVault(vault.Accounts[0].ID)
		t.Fatal(err)
	}
	if err := storage.SaveVaultKey(vault.Accounts[0].ID, vault.Key(), recipient); err != nil {
		_ = storage.DeleteVault(vault.Accounts[0].ID)
		t.Fatal(err)
	}
	return vault
}
