package model

import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"unicode/utf8"

	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/paw"
)

// PasswordSettingsFromField reads Paw's generation settings from a CXF password field.
func PasswordSettingsFromField(field *cxf.EditableField) PasswordSettings {
	settings := PasswordSettings{Mode: paw.CustomPassword}
	if field == nil {
		return settings
	}
	for _, raw := range field.Extensions {
		var extension passwordExtension
		if json.Unmarshal(raw, &extension) == nil && extension.Name == passwordExtensionName {
			return extension.PasswordSettings
		}
	}
	return settings
}

// PasswordField updates a concealed password and preserves its other metadata.
func PasswordField(value string, settings PasswordSettings, previous *cxf.EditableField) (*cxf.EditableField, error) {
	if !utf8.ValidString(value) {
		return nil, errors.New("invalid UTF-8")
	}
	if !validPasswordMode(settings.Mode) {
		return nil, fmt.Errorf("unsupported mode %d", settings.Mode)
	}
	encodedSettings, err := json.Marshal(passwordExtension{Name: passwordExtensionName, PasswordSettings: settings}, json.Deterministic(true))
	if err != nil {
		return nil, errors.New("cannot encode password settings")
	}
	field := cxf.EditableField{FieldType: cxf.FieldTypeConcealedString, Value: value}
	if previous != nil {
		field = *previous
		field.FieldType = cxf.FieldTypeConcealedString
		field.Value = value
	}
	field.Extensions, err = replacePasswordSettingsExtension(field.Extensions, encodedSettings)
	if err != nil {
		return nil, err
	}
	return &field, nil
}

func replacePasswordSettingsExtension(extensions []jsontext.Value, updated jsontext.Value) ([]jsontext.Value, error) {
	result := make([]jsontext.Value, 0, len(extensions)+1)
	replaced := false
	for _, raw := range extensions {
		var header struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(raw, &header) == nil && header.Name == passwordExtensionName {
			if !replaced {
				// Update Paw's known settings without dropping future extension members.
				var fields, changes map[string]jsontext.Value
				if err := json.Unmarshal(raw, &fields); err != nil {
					return nil, errors.New("cannot decode password settings")
				}
				if err := json.Unmarshal(updated, &changes); err != nil {
					return nil, errors.New("cannot decode updated password settings")
				}
				for key, value := range changes {
					fields[key] = value
				}
				merged, err := json.Marshal(fields, json.Deterministic(true))
				if err != nil {
					return nil, errors.New("cannot encode password settings")
				}
				result = append(result, merged)
				replaced = true
			}
			continue
		}
		result = append(result, raw)
	}
	if !replaced {
		result = append(result, updated)
	}
	return result, nil
}
