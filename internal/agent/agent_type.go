// SPDX-FileCopyrightText: 2023-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later


package agent

const (

	// TypeExtension is the Type Extension type for the Paw Agent
	TypeExtension = "type@paw"
)

// processTypeRequest process the custom agent request.
// Note: we are using the sshagent server implentation that always returns a SSH_AGENT_EXTENSION_FAILURE in accord to SSH PROTOCOL spec.
// So even if we return a detailed error, the client will always receive a "generic extension failure" error.
// See https://cs.opensource.google/go/x/crypto/+/refs/tags/v0.5.0:ssh/agent/server.go;l=183
func (a *Agent) processTypeRequest(contents []byte) ([]byte, error) {
	return []byte(a.t), nil
}
