// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	json "encoding/json/v2"
	"fmt"

	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
)

// showJSON presents a Paw item for CLI consumers.
type showJSON struct {
	*model.Item
	itemType model.ItemType
}

// MarshalJSON encodes the item in Paw's CLI JSON format.
func (item showJSON) MarshalJSON() ([]byte, error) {
	output := map[string]any{
		"id":   item.ID,
		"name": item.Title,
		"type": item.itemType.String(),
		"tags": item.Tags,
		"totp": nil,
	}
	if item.CreationAt != nil {
		output["createdAt"] = *item.CreationAt
	}
	if item.ModifiedAt != nil {
		output["modifiedAt"] = *item.ModifiedAt
	}
	if item.Subtitle != nil {
		output["subtitle"] = *item.Subtitle
	}
	if item.Favorite != nil {
		output["favorite"] = *item.Favorite
	}
	if item.Scope != nil {
		output["urls"] = item.Scope.URLs
		output["androidApps"] = item.Scope.AndroidApps
	}
	decoded, err := model.DecodeCredentials(item.Credentials)
	if err != nil {
		return nil, fmt.Errorf("cannot decode item credentials: %w", err)
	}
	if decoded.BasicAuth != nil {
		if decoded.BasicAuth.Username != nil {
			output["username"] = decoded.BasicAuth.Username.Value
		}
		if decoded.BasicAuth.Password != nil {
			output["password"] = decoded.BasicAuth.Password.Value
		}
	}
	if decoded.Note != nil {
		output["note"] = decoded.Note.Content.Value
	}
	if decoded.SSHKey != nil {
		key := decoded.SSHKey
		privateKey, publicKey, fingerprint, err := model.FormatSSHKey(*key)
		if err != nil {
			return nil, fmt.Errorf("cannot format SSH key: %w", err)
		}
		output["sshKey"] = map[string]any{
			"keyType":             key.KeyType,
			"keyComment":          key.KeyComment,
			"privateKey":          privateKey,
			"publicKey":           publicKey,
			"fingerprint":         fingerprint,
			"creationDate":        key.CreationDate,
			"expiryDate":          key.ExpiryDate,
			"keyGenerationSource": key.KeyGenerationSource,
		}
		output["addToAgent"] = item.Metadata.AddToAgent
	}
	var totps []cxf.TOTP
	for _, raw := range item.Credentials {
		kind, err := cxf.CredentialTypeOf(raw)
		if err != nil {
			return nil, err
		}
		if kind == cxf.CredentialTypeTOTP {
			var value cxf.TOTP
			if err := json.Unmarshal(raw, &value); err != nil {
				return nil, fmt.Errorf("cannot decode TOTP credential: %w", err)
			}
			totps = append(totps, value)
		}
	}
	if len(totps) > 0 {
		output["totp"] = totps[0]
	}
	if len(totps) > 1 {
		output["additionalTotp"] = totps[1:]
	}
	sections := make([]map[string]any, 0, len(decoded.CustomFields))
	for _, group := range decoded.CustomFields {
		fields := make([]map[string]any, 0, len(group.Credential.Fields))
		for _, field := range group.Credential.Fields {
			entry := map[string]any{"fieldType": field.FieldType, "value": field.Value, "label": field.Label}
			if field.ID != nil {
				entry["id"] = *field.ID
			}
			fields = append(fields, entry)
		}
		section := map[string]any{"label": group.Credential.Label, "fields": fields}
		if group.Credential.ID != nil {
			section["id"] = *group.Credential.ID
		}
		sections = append(sections, section)
	}
	output["customFields"] = sections
	files, err := item.Files()
	if err != nil {
		return nil, fmt.Errorf("cannot read attachment metadata: %w", err)
	}
	output["attachments"] = files
	return json.Marshal(output)
}
