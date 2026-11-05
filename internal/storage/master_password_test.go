// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChangeMasterPasswordValidatesBeforeStorage(t *testing.T) {
	store, vault := newItemStoreVault(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	for _, testCase := range []struct {
		name, id, current, next string
		ctx                     context.Context
		want                    string
	}{
		{name: "invalid vault ID", id: "../outside", current: "old", next: "new", want: "invalid vault ID"},
		{name: "missing current password", id: vault.Accounts[0].ID, next: "new", want: "master passwords are required"},
		{name: "missing new password", id: vault.Accounts[0].ID, current: "old", want: "master passwords are required"},
		{name: "canceled", id: vault.Accounts[0].ID, current: "old", next: "new", ctx: ctx, want: context.Canceled.Error()},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := testCase.ctx
			if ctx == nil {
				ctx = context.Background()
			}
			err := store.ChangeMasterPassword(ctx, testCase.id, testCase.current, testCase.next)
			require.ErrorContains(t, err, testCase.want)
		})
	}
}
