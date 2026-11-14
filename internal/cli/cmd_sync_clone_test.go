// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bytes"
	"strings"
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
			var output bytes.Buffer
			prompt := newPrompter(strings.NewReader(testCase.input), &output)
			name, err := promptCloneVaultName(prompt, "Personal")
			require.NoError(t, err)
			require.Equal(t, testCase.want, name)
			require.Equal(t, "New vault name [Personal]: ", output.String())
		})
	}
}

func TestPromptCloneVaultNameRejectsWhitespace(t *testing.T) {
	var output bytes.Buffer
	prompt := newPrompter(strings.NewReader("   \n"), &output)
	_, err := promptCloneVaultName(prompt, "Personal")
	require.ErrorContains(t, err, "vault name is required")
	require.Equal(t, "New vault name [Personal]: ", output.String())
}
