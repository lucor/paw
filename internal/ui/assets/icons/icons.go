// Package icons provides embedded PNG icons as Fyne resources.
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
	// Info identifies application information.
	Info = themed("info")
	// Scale identifies the software license.
	Scale = themed("scale")
	// ScrollText identifies third-party license notices.
	ScrollText = themed("scroll-text")
	// Code identifies the source code.
	Code = themed("code")
	// Bug identifies issue reporting.
	Bug = themed("bug")
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
	// ArchiveRestore identifies restoring a vault from a backup.
	ArchiveRestore = themed("archive-restore")
	// PlusCircle identifies creating a new vault.
	PlusCircle = themed("circle-plus")
	// Sync is the theme-aware cloud icon.
	Sync = themed("cloud")
	// Timer identifies the background auto-lock setting.
	Timer = themed("timer")
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
	// PawLight and PawDark are the transparent Paw application mark variants.
	PawLight = resource("light/paw.png")
	PawDark  = resource("dark/paw.png")
	// Paw is the theme-aware Paw application mark.
	Paw = icon.NewThemedResource(PawDark, PawLight)
	// VaultWhite is the vault glyph for colored appearance surfaces.
	VaultWhite = resource("dark/vault.png")
	// BriefcaseBusiness identifies the business briefcase appearance.
	BriefcaseBusiness = themed("briefcase-business")
	// BriefcaseBusinessWhite is the glyph for colored appearance surfaces.
	BriefcaseBusinessWhite = resource("dark/briefcase-business.png")
	// House identifies the home appearance.
	House = themed("house")
	// HouseWhite is the glyph for colored appearance surfaces.
	HouseWhite = resource("dark/house.png")
	// Archive identifies the archive appearance.
	Archive = themed("archive")
	// ArchiveWhite is the glyph for colored appearance surfaces.
	ArchiveWhite = resource("dark/archive.png")
	// Shield identifies the shield appearance.
	Shield = themed("shield")
	// ShieldWhite is the glyph for colored appearance surfaces.
	ShieldWhite = resource("dark/shield.png")
	// Folder identifies the folder appearance.
	Folder = themed("folder")
	// FolderWhite is the glyph for colored appearance surfaces.
	FolderWhite = resource("dark/folder.png")
	// Laptop identifies the laptop appearance.
	Laptop = themed("laptop")
	// LaptopWhite is the glyph for colored appearance surfaces.
	LaptopWhite = resource("dark/laptop.png")
	// KeyRoundWhite is the key glyph for colored appearance surfaces.
	KeyRoundWhite = resource("dark/key-round.png")
	// Pencil indicates that the vault appearance can be edited.
	Pencil = themed("pencil")
	// Check marks the selected vault color.
	Check = themed("check")
	// CheckWhite marks a selected color swatch.
	CheckWhite = resource("dark/check.png")
	// Lock identifies a locked vault on the unlock screen.
	Lock = themed("lock")
	// LockOpen identifies an unlocked vault in the selector.
	LockOpen = themed("lock-open")
	// More opens contextual actions for the current screen.
	More = themed("ellipsis-vertical")
	// Plus adds an entry to the current vault.
	Plus = themed("plus")
	// Minus decreases a numeric setting.
	Minus = themed("minus")
	// List represents the complete item category.
	List = themed("list")
	// Play identifies startup behavior.
	Play = themed("play")
	// Image identifies favicon settings.
	Image = themed("image")
	// Hash identifies TOTP algorithms.
	Hash = themed("hash")
	// Binary identifies TOTP digits.
	Binary = themed("binary")
	// SunMoon identifies theme settings.
	SunMoon = themed("sun-moon")
	// WholeWord identifies passphrase word count.
	WholeWord = themed("whole-word")
	// Search identifies the entry search field.
	Search = themed("search")
	// Down identifies the vault selection menu.
	Down = themed("chevron-down")
	// Codes identifies TOTP codes.
	Codes = themed("timer")
	// Settings opens application settings.
	Settings = themed("settings")
	// Settings2 opens custom TOTP settings.
	Settings2 = themed("settings-2")
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
