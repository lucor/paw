package model

import (
	"fmt"
	"strings"
)

// VaultInfo contains the storage ID and public details shown to users.
type VaultInfo struct {
	// ID is the vault's canonical Base64url identifier.
	ID string
	// Name is the display name.
	Name string
	// Appearance contains the public icon and color identifiers.
	Appearance VaultAppearance
	// NeedsMigration reports whether the vault still uses the legacy format.
	NeedsMigration bool
}

// VaultAppearance contains public stable identifiers for displaying a vault.
type VaultAppearance struct {
	Icon  string
	Color string
}

// DefaultVaultAppearance returns Paw's appearance for vaults without customization.
func DefaultVaultAppearance() VaultAppearance {
	return VaultAppearance{Icon: "paw", Color: "none"}
}

// NormalizeVaultAppearance replaces unsupported identifiers with Paw defaults.
func NormalizeVaultAppearance(appearance VaultAppearance) VaultAppearance {
	if !isVaultAppearanceIcon(appearance.Icon) {
		appearance.Icon = "paw"
	}
	if !isVaultAppearanceColor(appearance.Color) {
		appearance.Color = "none"
	}
	return appearance
}

func isVaultAppearanceIcon(icon string) bool {
	switch icon {
	case "paw", "vault", "briefcase-business", "house", "archive", "shield", "folder", "key-round", "laptop":
		return true
	default:
		return false
	}
}

func isVaultAppearanceColor(color string) bool {
	switch color {
	case "none", "gray", "blue", "green", "purple", "orange", "red":
		return true
	default:
		return false
	}
}

// FindVaultInfo resolves an ID or an unambiguous display name.
func FindVaultInfo(vaults []VaultInfo, value string) (VaultInfo, error) {
	var match VaultInfo
	for _, vault := range vaults {
		if vault.ID == value {
			return vault, nil
		}
		if strings.EqualFold(vault.Name, value) {
			if match.ID != "" {
				return VaultInfo{}, fmt.Errorf("vault name %q is ambiguous", value)
			}
			match = vault
		}
	}
	if match.ID == "" {
		return VaultInfo{}, fmt.Errorf("vault %q does not exist", value)
	}
	return match, nil
}
