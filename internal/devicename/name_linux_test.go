// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !android

package devicename

import "testing"

func TestDistributionName(t *testing.T) {
	if got := distributionName("ID=fedora\nNAME=\"Fedora Linux\"\n"); got != "Fedora Linux" {
		t.Fatalf("distributionName = %q", got)
	}
	if got := distributionName("ID=unknown\n"); got != "" {
		t.Fatalf("distributionName = %q", got)
	}
}
