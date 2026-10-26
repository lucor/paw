package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"go.lucor.dev/cxf"

	"lucor.dev/paw/internal/paw"
)

const (
	vaultExtensionName         = "paw.pm/vault"
	storageSchemaVersion uint8 = 1
	cxfMajorVersion      uint8 = 1
	cxfMinorVersion      uint8 = 0
	exporterRPID               = "paw.pm"
	exporterName               = "Paw"
)

// Vault holds a CXF document and the key used to encrypt its files.
type Vault struct {
	cxf.Header
	// Metadata contains the Paw vault name and creation time.
	Metadata VaultMetadata `json:"-"`
	key      *paw.Key
}

// VaultMetadata holds fields Paw stores in the vault extension.
type VaultMetadata struct {
	// VaultName is the name shown for this vault.
	VaultName string
	// CreationAt is the optional vault creation time in Unix seconds.
	CreationAt *uint64
}

// NewVault creates an empty CXF vault with a new account ID.
func NewVault(key *paw.Key, name string) (*Vault, error) {
	if key == nil {
		return nil, fmt.Errorf("cannot create vault: key is nil")
	}
	if !utf8.ValidString(name) {
		return nil, fmt.Errorf("cannot create vault: invalid UTF-8")
	}
	id, err := newID()
	if err != nil {
		return nil, fmt.Errorf("cannot generate vault account ID: %w", err)
	}
	timestamp := uint64(time.Now().Unix())
	creationAt := timestamp
	extension, err := json.Marshal(vaultExtension{
		Name:          vaultExtensionName,
		SchemaVersion: storageSchemaVersion,
		VaultName:     name,
		CreationAt:    &creationAt,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot encode vault metadata")
	}
	vault := &Vault{
		Header: cxf.Header{
			Version:             cxf.Version{Major: cxfMajorVersion, Minor: cxfMinorVersion},
			ExporterRPID:        exporterRPID,
			ExporterDisplayName: exporterName,
			Timestamp:           timestamp,
			Accounts: []cxf.Account{{
				ID:          id,
				Username:    "",
				Email:       "",
				Collections: []cxf.Collection{},
				Items:       []cxf.Item{},
				Extensions:  []json.RawMessage{extension},
			}},
		},
		Metadata: VaultMetadata{VaultName: name, CreationAt: &creationAt},
		key:      key,
	}
	if err := vault.Validate(); err != nil {
		return nil, fmt.Errorf("cannot create vault: %w", err)
	}
	return vault, nil
}

// DecodeVault decodes a CXF vault and attaches its runtime encryption key.
func DecodeVault(data []byte, key *paw.Key) (*Vault, error) {
	if key == nil {
		return nil, fmt.Errorf("cannot decode vault: key is nil")
	}
	var vault Vault
	if err := json.Unmarshal(data, &vault); err != nil {
		return nil, fmt.Errorf("cannot decode vault: %w", err)
	}
	vault.key = key
	return &vault, nil
}

// Key returns the key used to encrypt the vault's files.
func (vault *Vault) Key() *paw.Key {
	if vault == nil {
		return nil
	}
	return vault.key
}

// Name returns the vault name stored in the account extension.
func (vault *Vault) Name() (string, error) {
	return vault.Metadata.VaultName, nil
}

// CreationAt returns the optional vault creation time in Unix seconds.
func (vault *Vault) CreationAt() (*uint64, error) {
	if vault.Metadata.CreationAt == nil {
		return nil, nil
	}
	creationAt := *vault.Metadata.CreationAt
	return &creationAt, nil
}

// FilterItems returns summaries matching title text and item type.
func (vault *Vault) FilterItems(name string, itemType paw.ItemType) ([]cxf.Item, error) {
	if err := vault.Validate(); err != nil {
		return nil, err
	}
	name = strings.ToLower(name)
	items := make([]cxf.Item, 0, len(vault.Accounts[0].Items))
	for index := range vault.Accounts[0].Items {
		item := &vault.Accounts[0].Items[index]
		if name != "" && !strings.Contains(strings.ToLower(item.Title), name) {
			continue
		}
		typeOfItem, err := ItemTypeOf(item)
		if err != nil {
			return nil, err
		}
		if itemType != 0 && typeOfItem != itemType {
			continue
		}
		items = append(items, *item)
	}
	return items, nil
}

// Validate checks the CXF document and Paw vault rules.
func (vault *Vault) Validate() error {
	if vault == nil {
		return fmt.Errorf("vault is nil")
	}
	if err := vault.Header.Validate(); err != nil {
		return fmt.Errorf("invalid CXF vault: %w", err)
	}
	if vault.Metadata.VaultName == "" {
		return fmt.Errorf("invalid Paw vault metadata")
	}
	return validatePawVault(&vault.Header)
}

// validatePawVault checks rules that apply to Paw's stored vaults.
func validatePawVault(header *cxf.Header) error {
	if header.Version.Major != cxfMajorVersion || header.Version.Minor != cxfMinorVersion {
		return fmt.Errorf("unsupported CXF version %d.%d", header.Version.Major, header.Version.Minor)
	}
	if _, err := decodeVaultExtension(header.Accounts); err != nil {
		return err
	}
	for _, item := range header.Accounts[0].Items {
		if len(item.Credentials) != 0 {
			return fmt.Errorf("vault contains item credentials")
		}
	}
	return nil
}

// MarshalJSON validates and encodes the CXF document without the runtime key.
func (vault *Vault) MarshalJSON() ([]byte, error) {
	if err := vault.Validate(); err != nil {
		return nil, err
	}
	document := vault.Header
	account := document.Accounts[0]
	index, raw, err := findVaultExtension(document.Accounts)
	if err != nil {
		return nil, err
	}
	encoded, err := encodeVaultExtension(raw, vault.Metadata)
	if err != nil {
		return nil, err
	}
	account.Extensions[index] = encoded
	document.Accounts[0] = account
	return json.Marshal(document)
}

// UnmarshalJSON decodes and validates a CXF vault atomically.
func (vault *Vault) UnmarshalJSON(data []byte) error {
	if vault == nil {
		return fmt.Errorf("vault is nil")
	}
	var header cxf.Header
	if err := json.Unmarshal(data, &header); err != nil {
		return fmt.Errorf("cannot decode CXF vault: %w", err)
	}
	// Header decoding already checks CXF rules, so this only checks Paw rules.
	metadata, err := decodeVaultExtension(header.Accounts)
	if err != nil {
		return err
	}
	if err := validatePawVault(&header); err != nil {
		return err
	}
	*vault = Vault{
		Header: header,
		Metadata: VaultMetadata{
			VaultName:  metadata.VaultName,
			CreationAt: metadata.CreationAt,
		},
	}
	return nil
}

type vaultExtension struct {
	Name          string  `json:"name"`
	SchemaVersion uint8   `json:"schemaVersion"`
	VaultName     string  `json:"vaultName"`
	CreationAt    *uint64 `json:"creationAt,omitempty"`
}

// decodeVaultExtension reads the Paw metadata from the single vault account.
func decodeVaultExtension(accounts []cxf.Account) (vaultExtension, error) {
	_, raw, err := findVaultExtension(accounts)
	if err != nil {
		return vaultExtension{}, err
	}
	var metadata vaultExtension
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return vaultExtension{}, fmt.Errorf("invalid Paw vault metadata")
	}
	if metadata.Name != vaultExtensionName || metadata.VaultName == "" {
		return vaultExtension{}, fmt.Errorf("invalid Paw vault metadata")
	}
	if metadata.SchemaVersion != storageSchemaVersion {
		return vaultExtension{}, fmt.Errorf("unsupported Paw storage schema version %d", metadata.SchemaVersion)
	}
	return metadata, nil
}

// findVaultExtension returns the Paw metadata extension and its position.
func findVaultExtension(accounts []cxf.Account) (int, json.RawMessage, error) {
	if len(accounts) != 1 {
		return -1, nil, fmt.Errorf("vault must contain exactly one account")
	}
	index := -1
	var raw json.RawMessage
	for i, rawExtension := range accounts[0].Extensions {
		var header struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(rawExtension, &header); err != nil {
			return -1, nil, fmt.Errorf("cannot decode vault extension name")
		}
		if header.Name != vaultExtensionName {
			continue
		}
		if index >= 0 {
			return -1, nil, fmt.Errorf("duplicate Paw vault metadata")
		}
		index = i
		raw = accounts[0].Extensions[i]
	}
	if index < 0 {
		return -1, nil, fmt.Errorf("missing Paw vault metadata")
	}
	return index, raw, nil
}

// encodeVaultExtension updates known Paw fields and preserves unknown members.
func encodeVaultExtension(raw json.RawMessage, metadata VaultMetadata) (json.RawMessage, error) {
	fields := make(map[string]json.RawMessage)
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("cannot decode Paw vault metadata")
	}
	name, err := json.Marshal(vaultExtensionName)
	if err != nil {
		return nil, fmt.Errorf("cannot encode Paw vault metadata")
	}
	schemaVersion, err := json.Marshal(storageSchemaVersion)
	if err != nil {
		return nil, fmt.Errorf("cannot encode Paw vault metadata")
	}
	vaultName, err := json.Marshal(metadata.VaultName)
	if err != nil {
		return nil, fmt.Errorf("cannot encode Paw vault metadata")
	}
	fields["name"] = name
	fields["schemaVersion"] = schemaVersion
	fields["vaultName"] = vaultName
	if metadata.CreationAt == nil {
		delete(fields, "creationAt")
	} else {
		creationAt, err := json.Marshal(metadata.CreationAt)
		if err != nil {
			return nil, fmt.Errorf("cannot encode Paw vault metadata")
		}
		fields["creationAt"] = creationAt
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("cannot encode Paw vault metadata")
	}
	return encoded, nil
}
