// Package icons provides embedded Lucide PNGs as Fyne resources.
package icons

import (
	"embed"

	"fyne.io/fyne/v2"
	"go.lucor.dev/paw/internal/icon"
)

//go:embed light/*.png dark/*.png active/*.png danger/*.png
var assets embed.FS

// resource loads one generated PNG from the embedded assets.
func resource(path string) fyne.Resource {
	content, err := assets.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return fyne.NewStaticResource("lucide-"+path, content)
}

// themed follows theme changes without rebuilding the containing view.
func themed(name string) fyne.Resource {
	return icon.NewThemedResource(resource("dark/"+name+".png"), resource("light/"+name+".png"))
}

var (
	// Copy copies a field to the clipboard.
	Copy = themed("copy")
	// Show reveals a concealed field.
	Show = themed("eye")
	// Hide conceals a revealed field.
	Hide = themed("eye-off")
	// Edit opens the entry editor.
	Edit = themed("square-pen")
	// Delete removes an entry after confirmation.
	Delete = resource("danger/trash.png")
	// External opens a website.
	External = themed("external-link")
	// Audit opens the password audit.
	Audit = themed("shield-check")
	// Import identifies file import.
	Import = themed("file-input")
	// Export identifies file export.
	Export = themed("file-output")
	// LockVault locks the active vault.
	LockVault = themed("lock-keyhole")
	// CreateVault is the blue circled plus used for vault creation.
	CreateVault = resource("active/circle-plus.png")
	// RestoreVault is the theme-aware open folder icon.
	RestoreVault = themed("folder-open")
	// Sync is the theme-aware cloud icon.
	Sync = themed("cloud")
	// Back is the theme-aware back navigation arrow.
	Back = themed("arrow-left")
	// Next is the theme-aware forward navigation icon.
	Next = themed("chevron-right")
	// Up closes an expanded section.
	Up = themed("chevron-up")
	// Download saves a key to a file.
	Download = themed("download")
	// Generate starts the password generator.
	Generate = themed("dices")
	// GenerateKey identifies SSH key generation.
	GenerateKey = themed("rotate-ccw-key")
	// ImportKey identifies SSH key import.
	ImportKey = themed("file-key")
	// Vault is the theme-aware vault icon used by vault forms.
	Vault = themed("vault")
	// Lock identifies a locked vault on the unlock screen.
	Lock = themed("lock")
	// LockOpen identifies an unlocked vault in the selector.
	LockOpen = themed("lock-open")
	// More opens contextual actions for the current screen.
	More = themed("ellipsis-vertical")
	// Plus adds an entry to the current vault.
	Plus = themed("plus")
	// Search identifies the entry search field.
	Search = themed("search")
	// Down identifies the vault selection menu.
	Down = themed("chevron-down")
	// Codes identifies TOTP codes.
	Codes = themed("timer")
	// Settings opens application preferences.
	Settings = themed("settings")
	// Note identifies text notes.
	Note = themed("file-text")
	// Login is the fallback icon for login entries.
	Login = themed("globe")
	// Globe is the generic website and favicon fallback icon.
	Globe = themed("globe")
	// Password identifies password entries.
	Password = themed("key-round")
	// SSH identifies SSH keys.
	SSH = themed("terminal")
	// VaultActive marks the active Vault destination.
	VaultActive = resource("active/vault.png")
	// CodesActive marks the active Codes destination.
	CodesActive = resource("active/timer.png")
	// SettingsActive marks the active Settings destination.
	SettingsActive = resource("active/settings.png")
)
