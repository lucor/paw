// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package paw

import (
	"reflect"
	"testing"
)

func TestVault_FilterItemMetadata(t *testing.T) {
	v := &Vault{
		Name:         "test vault",
		ItemMetadata: make(map[ItemType]map[string]*Metadata),
	}
	note := NewNote()
	note.Name = "test name"
	v.AddItem(note)

	password := NewPassword()
	password.Name = "test password"
	v.AddItem(password)

	tests := []struct {
		name string
		opts *VaultFilterOptions
		want []*Metadata
	}{
		{
			name: "no filter",
			opts: &VaultFilterOptions{},
			want: []*Metadata{
				note.GetMetadata(),
				password.GetMetadata(),
			},
		},
		{
			name: "filter by name",
			opts: &VaultFilterOptions{
				Name: "test name",
			},
			want: []*Metadata{
				note.GetMetadata(),
			},
		},
		{
			name: "filter by type",
			opts: &VaultFilterOptions{
				ItemType: PasswordItemType,
			},
			want: []*Metadata{
				password.GetMetadata(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := v.FilterItemMetadata(tt.opts); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Vault.FilterItemMetadata() = %v, want %v", got, tt.want)
			}
		})
	}
}
