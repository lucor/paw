// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	_ "embed"
	"encoding/base64"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	fynestorage "fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

//go:embed testdata/github-favicon.png
var screenshotFavicon []byte

// GenerateScreenshotGallery captures the UI states used for visual review.
func GenerateScreenshotGallery(outputDir string) error {
	if outputDir == "" {
		return errors.New("screenshot output directory is required")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		if err := captureScreenshotGallery(outputDir, variant); err != nil {
			return err
		}
	}
	return nil
}

type screenshotGallery struct {
	outputDir string
	err       error
}

// captureScreenshotGallery renders the sample screens in one theme variant.
func captureScreenshotGallery(outputDir string, variant fyne.ThemeVariant) error {
	driver := test.NewApp()
	defer driver.Quit()
	driver.Settings().SetTheme(screenshotTheme{Theme: newPawTheme(), variant: variant})
	window := driver.NewWindow("Paw UI gallery")
	defer window.Close()
	window.Resize(fyne.NewSize(440, 800))
	window.Show()
	gallery := &screenshotGallery{outputDir: outputDir}
	suffix := "-" + screenshotVariantName(variant) + ".png"
	capture := func(name string, view fyne.CanvasObject) {
		window.SetContent(view)
		window.Resize(fyne.NewSize(440, 800))
		gallery.capture(window, name+suffix)
	}
	captureAt := func(name string, view fyne.CanvasObject, size fyne.Size) {
		window.SetContent(view)
		window.Resize(size)
		gallery.capture(window, name+suffix)
	}
	captureCurrent := func(name string, size fyne.Size) {
		window.Resize(size)
		gallery.capture(window, name+suffix)
	}

	addVault := NewAddVault(func() {}, func() {}, func() {})
	capture("add-vault", addVault)
	captureAt("add-vault-narrow", addVault, fyne.NewSize(320, 540))
	createVault := NewCreateVault(func() {}, func(string, string, model.VaultAppearance) error { return nil })
	capture("create-vault", createVault)
	captureAt("create-vault-narrow-empty", createVault, fyne.NewSize(320, 540))
	var createFields []*widget.Entry
	var createForm *widget.Form
	for _, object := range test.LaidOutObjects(createVault) {
		switch object := object.(type) {
		case *widget.Entry:
			createFields = append(createFields, object)
		case *widget.Form:
			createForm = object
		}
	}
	if len(createFields) == 3 && createForm != nil {
		createFields[0].SetText("Personal")
		var previewButton *widget.Button
		for _, object := range test.LaidOutObjects(createVault) {
			if button, ok := object.(*widget.Button); ok && button.Icon == nil && button.Size().Width == 80 {
				previewButton = button
				break
			}
		}
		if previewButton == nil {
			return errors.New("vault appearance preview button was not rendered")
		}
		previewPosition := driver.Driver().AbsolutePositionForObject(previewButton)
		test.TapCanvas(window.Canvas(), previewPosition.Add(fyne.NewPos(8, 8)))
		captureCurrent("vault-appearance-default", fyne.NewSize(440, 800))
		captureCurrent("vault-appearance-narrow", fyne.NewSize(320, 540))
		for _, object := range test.LaidOutObjects(window.Content()) {
			if button, ok := object.(*widget.Button); ok && button.Icon != nil && strings.Contains(button.Icon.Name(), "briefcase-business") {
				test.Tap(button)
				break
			}
		}
		for _, object := range test.LaidOutObjects(window.Content()) {
			if circle, ok := object.(*canvas.Circle); ok && circle.FillColor == selectedVaultColor("green") {
				position := driver.Driver().AbsolutePositionForObject(circle).Add(fyne.NewPos(circle.Size().Width/2, circle.Size().Height/2))
				test.TapCanvas(window.Canvas(), position)
				break
			}
		}
		captureCurrent("vault-appearance-custom", fyne.NewSize(440, 800))
		for _, object := range test.LaidOutObjects(window.Content()) {
			if button, ok := object.(*widget.Button); ok && button.Icon == icons.Back {
				test.Tap(button)
				break
			}
		}
		captureCurrent("create-vault-customized", fyne.NewSize(440, 800))
		createFields[1].SetText("example master password")
		createFields[2].SetText("example master password")
		capture("create-vault-valid", createVault)
	}
	capture("unlock-vault", NewUnlockVault(model.VaultInfo{ID: "personal", Name: "Personal"}, nil, nil, func() {}, func() {}, func() {}))
	unlockFailure := NewUnlockVault(model.VaultInfo{ID: "personal", Name: "Personal"}, nil, nil, nil, nil, func() {})
	window.SetContent(unlockFailure)
	window.Resize(fyne.NewSize(440, 800))
	unlockFailure.showIncorrectPassword()
	captureCurrent("unlock-vault-error", fyne.NewSize(440, 800))
	capture("vault-selector", NewSelectVault([]VaultChoice{
		{Name: "Personal", Unlocked: true, Open: func() {}},
		{Name: "Work", Open: func() {}},
		{Name: "Archive", Unlocked: true, Open: func() {}},
	}, func() {}))
	vaultSettings := NewVaultSettingsView("Personal", model.DefaultVaultAppearance(), func() {}, func() {}, func() {}, func() {})
	capture("vault-settings", vaultSettings)
	captureAt("vault-settings-narrow", vaultSettings, fyne.NewSize(320, 540))
	masterPassword := newMasterPasswordView(func() {})
	capture("change-master-password", masterPassword.view)
	masterPassword.current.SetText("example current password")
	masterPassword.password.Entry.SetText("example new master password")
	masterPassword.confirm.SetText("example new master password")
	masterPassword.password.strength.label.Refresh()
	capture("change-master-password-filled", masterPassword.view)
	captureAt("change-master-password-narrow", masterPassword.view, fyne.NewSize(320, 540))
	vaultEditor := NewVaultIdentityEditor("Personal", model.DefaultVaultAppearance(), func(string, model.VaultAppearance) error { return nil }, func() {})
	capture("edit-vault", vaultEditor)
	captureAt("edit-vault-narrow", vaultEditor, fyne.NewSize(320, 540))
	customizedEditor := NewVaultIdentityEditor("Work", model.DefaultVaultAppearance(), func(string, model.VaultAppearance) error { return nil }, func() {})
	window.SetContent(customizedEditor)
	window.Resize(fyne.NewSize(440, 800))
	preview := screenshotFindVaultIdentityPreview(customizedEditor)
	if preview == nil {
		return errors.New("vault identity preview button was not rendered")
	}
	test.Tap(preview)
	appearanceView := window.Content()
	for _, object := range test.LaidOutObjects(appearanceView) {
		if button, ok := object.(*widget.Button); ok && button.Icon != nil && strings.Contains(button.Icon.Name(), "briefcase-business") {
			test.Tap(button)
			break
		}
	}
	for _, object := range test.LaidOutObjects(appearanceView) {
		if circle, ok := object.(*canvas.Circle); ok && circle.FillColor == selectedVaultColor("green") {
			position := driver.Driver().AbsolutePositionForObject(circle).Add(fyne.NewPos(circle.Size().Width/2, circle.Size().Height/2))
			test.TapCanvas(window.Canvas(), position)
			break
		}
	}
	for _, object := range test.LaidOutObjects(appearanceView) {
		if button, ok := object.(*widget.Button); ok && button.Icon == icons.Back {
			test.Tap(button)
			break
		}
	}
	captureCurrent("edit-vault-customized", fyne.NewSize(440, 800))

	listOptions := screenshotEntryListFixture()
	githubFavicon := model.NewFavicon("github.com", screenshotFavicon, "png")
	listOptions.Entries[0].Icon = githubFavicon
	list := NewEntryList(listOptions)
	capture("entry-list", list)
	captureAt("entry-list-narrow", list, fyne.NewSize(320, 540))
	emptyVaultOptions := screenshotEntryListFixture()
	emptyVaultOptions.Entries = nil
	capture("entry-list-empty-vault", NewEntryList(emptyVaultOptions))
	searchEmptyOptions := screenshotEntryListFixture()
	searchEmptyOptions.Search = "not-found"
	capture("entry-list-search-empty", NewEntryList(searchEmptyOptions))
	categoryEmptyOptions := screenshotEntryListFixture()
	categoryEmptyOptions.Category = "password"
	categoryEmptyOptions.Entries = []EntryRow{{Title: "GitHub", Category: "login", Icon: icons.Login, Open: func() {}}}
	capture("entry-list-category-empty", NewEntryList(categoryEmptyOptions))
	window.Resize(fyne.NewSize(320, 540))
	gallery.capture(window, "entry-list-short-window"+suffix)
	for _, menu := range []struct {
		name  string
		label string
	}{{"vault-selector-menu-active", "Select vault"}, {"entry-add-menu", "Add entry"}, {"vault-actions-menu", "Vault actions"}} {
		window.SetContent(list)
		button := screenshotFindPilotIconButton(list, menu.label)
		if button == nil {
			return fmt.Errorf("%s button was not rendered", menu.label)
		}
		test.Tap(button)
		captureCurrent(menu.name, fyne.NewSize(320, 540))
		screenshotCloseOverlay(window)
	}

	window.SetContent(list)
	categoryLabel := screenshotFindPilotLabel(list, "All items")
	if categoryLabel == nil {
		return errors.New("all items category was not rendered")
	}
	screenshotChooseEntryCategory(driver, window, categoryLabel, "Logins")
	selectedOptions := screenshotEntryListFixture()
	selectedOptions.Category = "login"
	selectedList := NewEntryList(selectedOptions)
	capture("entry-list-category-selected", selectedList)
	categoryLabel = screenshotFindPilotLabel(selectedList, "Logins")
	if categoryLabel == nil {
		return errors.New("logins category was not rendered")
	}
	window.SetContent(selectedList)
	position := driver.Driver().AbsolutePositionForObject(categoryLabel).Add(fyne.NewPos(categoryLabel.Size().Width/2, categoryLabel.Size().Height/2))
	test.TapCanvas(window.Canvas(), position)
	captureCurrent("entry-category-menu-selected", fyne.NewSize(440, 800))
	screenshotCloseOverlay(window)
	searchEntry, ok := screenshotFindPilotEntry(list, "Search entries…").(*widget.Entry)
	if !ok {
		return errors.New("entry search field was not rendered")
	}
	searchEntry.SetText("github")
	searchOptions := screenshotEntryListFixture()
	searchOptions.Category = "login"
	searchOptions.Search = "github"
	searchList := NewEntryList(searchOptions)
	capture("entry-list-search", searchList)

	navigation := screenshotFindPilotNavigation(list)
	if navigation == nil {
		return errors.New("bottom navigation was not rendered")
	}
	codeContext, cancelCodes := context.WithCancel(context.Background())
	defer cancelCodes()
	codesView := newCodesView(codeContext, []codesEntry{
		{Title: "GitHub", Username: "dev@example.com", ItemID: "github", Icon: githubFavicon, Credential: screenshotTOTPCredential()},
		{Title: "Google", Username: "user@gmail.com", ItemID: "google", Icon: icons.Globe, Credential: screenshotTOTPCredential()},
		{Title: "GitLab", Username: "dev@example.com", ItemID: "gitlab", Icon: icons.Globe, Credential: screenshotTOTPCredential()},
	}, nil)
	codesOptions := screenshotEntryListFixture()
	codesOptions.Codes = codesView.View
	codesList := NewEntryList(codesOptions)
	window.SetContent(codesList)
	window.Resize(fyne.NewSize(440, 800))
	codesNavigation := screenshotFindPilotNavigation(window.Content())
	if codesNavigation == nil {
		return errors.New("codes navigation was not rendered")
	}
	test.Tap(codesNavigation.buttons[1].button)
	gallery.capture(window, "codes"+suffix)
	window.SetContent(codesList)
	window.Resize(fyne.NewSize(320, 540))
	codesNavigation = screenshotFindPilotNavigation(window.Content())
	if codesNavigation == nil {
		return errors.New("codes navigation was not rendered")
	}
	codesNavigation.Select(0)
	codesNavigation.Select(1)
	gallery.capture(window, "codes-list-narrow"+suffix)
	codeActions := screenshotFindPilotIconButton(codesList, "Vault actions")
	if codeActions == nil {
		return errors.New("vault actions button was not rendered in the Codes view")
	}
	test.Tap(codeActions)
	captureCurrent("codes-actions-menu", fyne.NewSize(320, 540))
	screenshotCloseOverlay(window)
	emptyContext, cancelEmpty := context.WithCancel(context.Background())
	defer cancelEmpty()
	emptyCodes := newCodesView(emptyContext, nil, nil)
	emptyOptions := screenshotEntryListFixture()
	emptyOptions.Codes = emptyCodes.View
	emptyList := NewEntryList(emptyOptions)
	window.SetContent(emptyList)
	window.Resize(fyne.NewSize(440, 800))
	emptyNavigation := screenshotFindPilotNavigation(window.Content())
	if emptyNavigation == nil {
		return errors.New("empty Codes navigation was not rendered")
	}
	test.Tap(emptyNavigation.buttons[1].button)
	gallery.capture(window, "codes-empty"+suffix)
	cancelEmpty()
	cancelCodes()
	navigation.Select(2)

	login, err := screenshotLoginEditorFixture()
	if err != nil {
		return err
	}
	login.Metadata.Favicon = githubFavicon
	password, err := model.NewPassword("Home Wi-Fi", "synthetic-password", model.PasswordSettings{Mode: paw.CustomPassword})
	if err != nil {
		return err
	}
	note, err := model.NewNote("Recovery note", "Synthetic note for UI review.")
	if err != nil {
		return err
	}
	sshItem, err := screenshotSSHDetailFixture()
	if err != nil {
		return err
	}

	for _, item := range []*model.Item{login, password, note, sshItem} {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		view, clear, err := newEntryDetailWithClear(ctx, item, window, func() {}, func() {}, func() {})
		if err != nil {
			return err
		}
		capture("entry-detail-"+screenshotPilotItemName(item), view)
		if item == login {
			captureAt("entry-detail-login-totp-narrow", view, fyne.NewSize(320, 640))
			window.SetContent(view)
			button := screenshotFindPilotIconButton(view, "Entry actions")
			if button == nil {
				return errors.New("entry actions button was not rendered")
			}
			test.Tap(button)
			captureCurrent("entry-detail-actions-menu", fyne.NewSize(320, 540))
			screenshotCloseOverlay(window)
		}
		if item == sshItem {
			captureAt("entry-detail-ssh-key-narrow", view, fyne.NewSize(320, 640))
			button := screenshotFindPilotIconButton(view, "Show private key")
			if button == nil {
				return errors.New("private key reveal button was not rendered")
			}
			test.Tap(button)
			captureCurrent("entry-detail-ssh-key-revealed", fyne.NewSize(320, 640))
		}
		cancel()
		clear()
	}

	loginEditor, err := NewLoginEditor(context.Background(), login, model.NewDefaultSettings().FaviconDownloader, false)
	if err != nil {
		return err
	}
	capture("edit-login", loginEditor.View)
	window.SetContent(loginEditor.View)
	showTOTPSettingsDialog(loginEditor, window)
	captureCurrent("totp-settings-dialog", fyne.NewSize(440, 800))
	popup := window.Canvas().Overlays().Top()
	fyne.DoAndWait(func() {
		for _, object := range test.LaidOutObjects(popup) {
			if button, ok := object.(*widget.Button); ok && button.Text == "Cancel" {
				button.OnTapped()
				break
			}
		}
	})
	passwordEditor, err := NewPasswordEditor(password, false)
	if err != nil {
		return err
	}
	capture("edit-password", passwordEditor.View)
	noteEditor, err := NewNoteEditor(note, false)
	if err != nil {
		return err
	}
	capture("edit-note", noteEditor.View)
	sshEditor, err := NewSSHKeyEditor(sshItem, false)
	if err != nil {
		return err
	}
	captureAt("edit-ssh-key", sshEditor.View, fyne.NewSize(440, 926))
	captureAt("edit-ssh-key-narrow", sshEditor.View, fyne.NewSize(320, 640))
	test.Tap(sshEditor.RevealPrivateButton)
	capture("edit-ssh-key-private-revealed", sshEditor.View)

	appState := &model.AppState{Settings: model.NewDefaultSettings()}
	settings := NewSettingsView(appState, nil, window)
	capture("settings", settings.View)
	for _, object := range test.LaidOutObjects(settings.View) {
		if scroll, ok := object.(*container.Scroll); ok {
			scroll.ScrollToBottom()
			break
		}
	}
	captureCurrent("settings-about", fyne.NewSize(440, 800))
	capture("about-paw", NewAboutPawView(window, func() {}))

	for _, state := range []string{"idle", "running", "completed", "partial", "no-passwords", "findings", "exposed-findings", "reused-findings", "success", "error"} {
		audit := newPilotAudit(state)
		name := state
		if state == "idle" {
			name = "initial"
		}
		capture("password-audit-"+name, audit.View)
		if state == "partial" {
			captureAt("password-audit-partial-narrow", audit.View, fyne.NewSize(320, 540))
		}
		if state == "findings" || state == "exposed-findings" || state == "reused-findings" {
			captureAt("password-audit-"+name+"-narrow", audit.View, fyne.NewSize(320, 540))
		}
		audit.Close()
	}

	backupRoot, err := os.MkdirTemp("", "paw-gallery-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(backupRoot)
	backupStore, err := storage.NewFileStorage(backupRoot)
	if err != nil {
		return err
	}
	defer backupStore.Close()
	backupFlow := &restoreFlow{window: window, store: backupStore, back: func() {}, restored: func(*model.Vault) {}}
	backupFlow.selectFile("")
	captureCurrent("restore-select-file", fyne.NewSize(440, 800))
	backupFlow.unlock(fynestorage.NewFileURI("/tmp/paw-example.cxf.age"), "")
	captureCurrent("restore-unlock-backup", fyne.NewSize(440, 800))
	createdAt, exportedAt := uint64(1705055040), uint64(1709475420)
	backup := &storage.PawBackup{Name: "Personal", CreationAt: &createdAt, ExportedAt: exportedAt, Collections: 2, Credentials: 5, AccountID: "account-123"}
	backupFlow.review(backup, "")
	captureCurrent("restore-review", fyne.NewSize(440, 800))
	captureAt("restore-review-narrow", window.Content(), fyne.NewSize(320, 540))
	key, err := paw.MakeOneTimeKey()
	if err != nil {
		return err
	}
	vault, err := model.NewVault(key, "Personal")
	if err != nil {
		return err
	}
	if err := backupStore.UpdateVaultIdentity(vault, "Personal", model.DefaultVaultAppearance()); err != nil {
		return err
	}
	backupFlow.review(backup, "")
	captureCurrent("restore-review-collision", fyne.NewSize(440, 800))
	backupFlow.complete(vault)
	captureCurrent("restore-complete", fyne.NewSize(440, 800))

	exportView := newExportCredentialsView(window, nil, vault, "Personal", func() {})
	window.SetContent(exportView.View)
	captureCurrent("export-encrypted-default", fyne.NewSize(440, 800))
	exportView.Mode.SetSelected(exportPlaintextLabel)
	captureCurrent("export-plaintext-warning", fyne.NewSize(440, 800))

	passwordItem, err := model.NewPassword("Example password", "", model.PasswordSettings{Mode: paw.CustomPassword})
	if err != nil {
		return err
	}
	passwordEditor, err = NewPasswordEditor(passwordItem, true)
	if err != nil {
		return err
	}
	window.SetContent(passwordEditor.View)
	passwordSettings := model.NewDefaultSettings().Password
	generator := NewPasswordGenerator(passwordSettings)
	for _, mode := range []paw.PasswordMode{paw.RandomPassword, paw.PassphrasePassword, paw.PinPassword} {
		password := &paw.PasswordSpec{Mode: mode}
		switch mode {
		case paw.RandomPassword:
			password.Length = passwordSettings.Random.DefaultLength
			password.Format = passwordSettings.Random.DefaultFormat
		case paw.PassphrasePassword:
			password.Length = passwordSettings.Passphrase.DefaultLength
		case paw.PinPassword:
			password.Length = passwordSettings.Pin.DefaultLength
			password.Format = paw.DigitsFormat
		}
		generator.ShowPasswordGenerator(binding.NewString(), password, window)
		popup := window.Canvas().Overlays().Top()
		entry := screenshotPasswordGeneratorEntry(popup)
		if entry == nil {
			return errors.New("password generator field was not rendered")
		}
		captureCurrent("password-generator-"+screenshotPilotModeName(mode.String()), fyne.NewSize(440, 800))
		screenshotCloseOverlay(window)
	}
	return gallery.err
}

func newPilotAudit(state string) *PasswordAudit {
	audit := NewPasswordAudit("Personal", nil, func(fyne.CanvasObject) {}, func(string) {}, func() {})
	audit.result = screenshotAuditFixture()
	audit.hasResult = state != "idle"
	audit.running = state == "running"
	if state != "idle" && state != "running" && state != "no-passwords" {
		audit.checkedAt = time.Date(2026, time.October, 30, 14, 15, 0, 0, time.UTC)
	}
	if state == "partial" {
		audit.result.ExposedFailed = true
		audit.result.ExposedChecked = 1
	}
	if state == "clear" || state == "success" || state == "error" {
		audit.result.Findings = nil
	}
	if state == "no-passwords" {
		audit.result = AuditResult{LocalComplete: true}
	}
	if state == "error" {
		audit.result.LoadErrors = 1
		audit.result.LocalComplete = false
	}
	audit.overview()
	if state == "running" {
		audit.updateProgress(audit.runID, AuditProgress{Done: 28, Total: 42, Phase: AuditPhaseBreaches})
	}
	if state == "findings" {
		audit.result.Findings[0].Subtitle = "very.long.username@example-company.test"
		audit.result.Findings[1].Subtitle = ""
		audit.result.Findings = append(audit.result.Findings, AuditFinding{
			AuditEntry: AuditEntry{ID: "router", Title: "Home router", Icon: icons.Password}, Score: 1,
		})
		audit.showFindings("weak")
	} else if state == "exposed-findings" {
		audit.result.Findings = append(audit.result.Findings,
			AuditFinding{AuditEntry: AuditEntry{ID: "forum", Title: "Old Forum", Subtitle: "user@example.com", Icon: icons.Login}, Exposed: true, ExposedCount: 832},
			AuditFinding{AuditEntry: AuditEntry{ID: "shop", Title: "Shopping Account", Subtitle: "shop@example.com", Icon: icons.Login}, Exposed: true, ExposedCount: 3},
		)
		audit.showFindings("exposed")
	} else if state == "reused-findings" {
		audit.result.Findings = append(audit.result.Findings,
			AuditFinding{AuditEntry: AuditEntry{ID: "gitlab", Title: "GitLab", Subtitle: "dev@example.com", Icon: icons.Login}, ReuseCount: 2, ReuseGroup: 2},
			AuditFinding{AuditEntry: AuditEntry{ID: "old-forum", Title: "Old forum", Subtitle: "user@example.com", Icon: icons.Login}, ReuseCount: 2, ReuseGroup: 2},
		)
		audit.showFindings("reused")
	}
	return audit
}

func (g *screenshotGallery) capture(window fyne.Window, name string) {
	if g.err != nil {
		return
	}
	test.LaidOutObjects(window.Content())
	window.Canvas().Refresh(window.Content())
	if overlay := window.Canvas().Overlays().Top(); overlay != nil {
		test.LaidOutObjects(overlay)
		window.Canvas().Refresh(overlay)
	}
	file, err := os.Create(filepath.Join(g.outputDir, name))
	if err == nil {
		err = png.Encode(file, window.Canvas().Capture())
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
	}
	if err != nil {
		g.err = fmt.Errorf("capture %s: %w", name, err)
	}
}

type screenshotTheme struct {
	fyne.Theme
	variant fyne.ThemeVariant
}

func (t screenshotTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	return (&pawTheme{Theme: t.Theme, variant: t.variant, variantPinned: true}).Color(name, t.variant)
}

func (t screenshotTheme) Size(name fyne.ThemeSizeName) float32 {
	return (&pawTheme{Theme: t.Theme}).Size(name)
}

func screenshotVariantName(variant fyne.ThemeVariant) string {
	if variant == theme.VariantDark {
		return "dark"
	}
	return "light"
}

func screenshotEntryListFixture() EntryListOptions {
	return EntryListOptions{
		VaultName: "Personal", Codes: widget.NewLabel("Codes"), Settings: widget.NewLabel("Settings"),
		VaultActions: NewVaultActions(func() {}, func() {}, func() {}, func() {}),
		AddEntry:     func(EntryKind) {}, AddVault: func() {},
		Categories: []EntryCategory{
			{ID: "all", Label: "All items", Icon: icons.List}, {ID: "login", Label: "Logins", Icon: icons.Login},
			{ID: "password", Label: "Passwords", Icon: icons.Password}, {ID: "note", Label: "Notes", Icon: icons.Note},
			{ID: "ssh", Label: "SSH keys", Icon: icons.SSH},
		},
		Vaults: []VaultChoice{{Name: "Personal", Unlocked: true, Active: true, Open: func() {}}, {Name: "Work", Open: func() {}}},
		Entries: []EntryRow{
			{Title: "GitHub", Subtitle: "dev@example.com", Category: "login", Icon: icons.Login, Open: func() {}},
			{Title: "Google", Subtitle: "user@gmail.com", Category: "login", Icon: icons.Login, Open: func() {}},
			{Title: "Home WiFi", Category: "password", Icon: icons.Password, Open: func() {}},
			{Title: "Production server", Subtitle: "deploy", Category: "ssh", Icon: icons.SSH, Open: func() {}},
			{Title: "Recovery codes", Category: "note", Icon: icons.Note, Open: func() {}},
		},
	}
}

func screenshotTOTPCredential() cxf.TOTP {
	return cxf.TOTP{Type: cxf.CredentialTypeTOTP, Secret: "JBSWY3DPEHPK3PXP", Algorithm: "sha1", Period: 30, Digits: 6}
}

func screenshotLoginEditorFixture() (*model.Item, error) {
	item, err := model.NewLogin("GitHub", "dev@example.com", "synthetic-password", model.PasswordSettings{Mode: paw.CustomPassword})
	if err != nil {
		return nil, err
	}
	credential, err := json.Marshal(screenshotTOTPCredential(), json.Deterministic(true))
	if err != nil {
		return nil, err
	}
	item.Credentials = append(item.Credentials, credential)
	return item, nil
}

func screenshotSSHDetailFixture() (*model.Item, error) {
	seed := make([]byte, ed25519.SeedSize)
	for index := range seed {
		seed[index] = byte(index)
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, err
	}
	credential := cxf.SSHKey{Type: cxf.CredentialTypeSSHKey, KeyType: "ssh-ed25519", KeyComment: "Deploy key", PrivateKey: base64.RawURLEncoding.EncodeToString(der)}
	return model.NewSSHKey("Production server", credential, "", model.PasswordSettings{Mode: paw.CustomPassword})
}

func screenshotAuditFixture() AuditResult {
	return AuditResult{Passwords: 3, ExposedChecked: 3, LocalComplete: true, Findings: []AuditFinding{
		{AuditEntry: AuditEntry{ID: "github", Title: "GitHub", Subtitle: "dev@example.com", Icon: icons.Login}, Score: 0, ReuseCount: 2, ReuseGroup: 1, Exposed: true, ExposedCount: 12450},
		{AuditEntry: AuditEntry{ID: "forum", Title: "Old forum", Subtitle: "user@example.com", Icon: icons.Login}, Score: 1, ReuseCount: 2, ReuseGroup: 1},
	}}
}

func screenshotFindPilotLabel(view fyne.CanvasObject, text string) *widget.Label {
	for _, object := range test.LaidOutObjects(view) {
		if label, ok := object.(*widget.Label); ok && label.Text == text {
			return label
		}
	}
	return nil
}

func screenshotFindPilotEntry(view fyne.CanvasObject, placeholder string) fyne.CanvasObject {
	for _, object := range test.LaidOutObjects(view) {
		if entry, ok := object.(*widget.Entry); ok && entry.PlaceHolder == placeholder {
			return entry
		}
	}
	return nil
}

func screenshotFindPilotIconButton(view fyne.CanvasObject, label string) *iconButton {
	for _, object := range test.LaidOutObjects(view) {
		if button, ok := object.(*iconButton); ok && button.AccessibilityLabel() == label {
			return button
		}
	}
	return nil
}

func screenshotFindPilotNavigation(view fyne.CanvasObject) *Navigation {
	if navigation, ok := view.(*Navigation); ok {
		return navigation
	}
	container, ok := view.(*fyne.Container)
	if !ok {
		return nil
	}
	for _, child := range container.Objects {
		if navigation := screenshotFindPilotNavigation(child); navigation != nil {
			return navigation
		}
	}
	return nil
}

func screenshotFindVaultIdentityPreview(view fyne.CanvasObject) *widget.Button {
	for _, object := range test.LaidOutObjects(view) {
		if button, ok := object.(*widget.Button); ok && button.Text == "" && button.Icon == nil && button.Size().Width >= 80 {
			return button
		}
	}
	return nil
}

func screenshotPasswordGeneratorEntry(view fyne.CanvasObject) *widget.Entry {
	for _, object := range test.LaidOutObjects(view) {
		if entry, ok := object.(*widget.Entry); ok {
			return entry
		}
	}
	return nil
}

func screenshotChooseEntryCategory(app fyne.App, window fyne.Window, label *widget.Label, name string) {
	position := app.Driver().AbsolutePositionForObject(label).Add(fyne.NewPos(label.Size().Width/2, label.Size().Height/2))
	test.TapCanvas(window.Canvas(), position)
	menu := window.Canvas().Overlays().Top()
	for _, object := range test.LaidOutObjects(menu) {
		if label, ok := object.(*widget.Label); ok && label.Text == name {
			position := app.Driver().AbsolutePositionForObject(label).Add(fyne.NewPos(label.Size().Width/2, label.Size().Height/2))
			test.TapCanvas(window.Canvas(), position)
			return
		}
	}
}

func screenshotCloseOverlay(window fyne.Window) {
	if overlay := window.Canvas().Overlays().Top(); overlay != nil {
		window.Canvas().Overlays().Remove(overlay)
	}
}

func screenshotPilotItemName(item *model.Item) string {
	switch item.Metadata.Type {
	case model.LoginItemType:
		return "login-totp"
	case model.PasswordItemType:
		return "password"
	case model.NoteItemType:
		return "note"
	case model.SSHKeyItemType:
		return "ssh-key"
	default:
		return "unknown"
	}
}

func screenshotPilotModeName(mode string) string {
	switch mode {
	case paw.PassphrasePassword.String():
		return "passphrase"
	case paw.PinPassword.String():
		return "pin"
	default:
		return "random"
	}
}
