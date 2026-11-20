// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPromptCloneVaultName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "source name default", input: "\n", want: "Personal"},
		{name: "custom name", input: "  Work copy  \n", want: "Work copy"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			prompt := testInteractivePrompter(t, testCase.input)
			name, err := promptCloneVaultName(prompt, "Personal")
			require.NoError(t, err)
			require.Equal(t, testCase.want, name)
		})
	}
}

func TestPromptCloneVaultNameRejectsWhitespace(t *testing.T) {
	prompt := testInteractivePrompter(t, "\n")
	_, err := promptCloneVaultName(prompt, "")
	require.ErrorContains(t, err, "vault name is required")
}
