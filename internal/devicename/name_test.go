// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package devicename

import (
	"strings"
	"testing"
)

func TestNames(t *testing.T) {
	for _, tc := range []struct {
		name string
		want string
	}{
		{
			name: " Pixel 5 \n",
			want: "Pixel 5",
		},
		{
			name: " LOCALHOST ",
			want: "",
		},
		{
			name: "localhost.localdomain",
			want: "",
		},
		{
			name: " \t",
			want: "",
		},
	} {
		if got := normalizedHostname(tc.name); got != tc.want {
			t.Errorf("normalizedHostname(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
	for _, value := range []string{"", " ", "LOCALHOST", strings.Repeat("a", 201)} {
		if Validate(value) == nil {
			t.Errorf("Validate(%q) succeeded", value)
		}
	}
	if err := Validate("Pixel 5"); err != nil {
		t.Fatal(err)
	}
}
