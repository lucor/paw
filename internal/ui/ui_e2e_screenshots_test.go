// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"filippo.io/age"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	fynestorage "fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

func TestUIE2EScreenshotGallery(t *testing.T) {
	outputDir := os.Getenv("PAW_UI_SNAPSHOT_DIR")
	if outputDir == "" {
		outputDir = t.TempDir()
	}
	require.NoError(t, os.MkdirAll(outputDir, 0o755))
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		t.Run(variantName(variant), func(t *testing.T) {
			runUIE2EScreenshotGallery(t, outputDir, variant)
		})
	}
	t.Logf("UI screenshot gallery: %s", outputDir)
}

func runUIE2EScreenshotGallery(t *testing.T, outputDir string, variant fyne.ThemeVariant) {
	t.Helper()
	driver := test.NewApp()
	t.Cleanup(driver.Quit)
	driver.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
	window := driver.NewWindow("Paw UI gallery")
	t.Cleanup(window.Close)
	window.Resize(fyne.NewSize(440, 800))
	window.Show()
	suffix := "-" + variantName(variant) + ".png"
	capture := func(name string, view fyne.CanvasObject) {
		window.SetContent(view)
		window.Resize(fyne.NewSize(440, 800))
		captureUIPilotScreen(t, window, outputDir, name+suffix)
	}
	captureAt := func(name string, view fyne.CanvasObject, size fyne.Size) {
		window.SetContent(view)
		window.Resize(size)
		captureUIPilotScreen(t, window, outputDir, name+suffix)
	}
	captureCurrent := func(name string, size fyne.Size) {
		window.Resize(size)
		captureUIPilotScreen(t, window, outputDir, name+suffix)
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
		require.NotNil(t, previewButton)
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
	capture("unlock-vault", NewUnlockVault(model.VaultInfo{ID: "personal", Name: "Personal"}, &storage.StorageMock{}, nil, func() {}, func() {}, func() {}))
	unlockFailureStore := &storage.StorageMock{}
	unlockFailureStore.OnLoadVaultKey = func(string, string) (*paw.Key, error) { return nil, &age.NoIdentityMatchError{} }
	unlockFailure := NewUnlockVault(model.VaultInfo{ID: "personal", Name: "Personal"}, unlockFailureStore, nil, nil, nil, func() {})
	window.SetContent(unlockFailure)
	window.Resize(fyne.NewSize(440, 800))
	unlockFailure.password.SetText("incorrect password")
	test.Tap(unlockFailure.button)
	require.Eventually(t, func() bool {
		var visible bool
		fyne.DoAndWait(func() { visible = unlockFailure.passwordError.Visible() })
		return visible
	}, time.Second, 10*time.Millisecond)
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
	test.Tap(findVaultIdentityPreview(t, customizedEditor))
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

	listOptions := entryListFixture()
	githubFavicon := darkFaviconFixture()
	listOptions.Entries[0].Icon = githubFavicon
	list := NewEntryList(listOptions)
	capture("entry-list", list)
	captureAt("entry-list-narrow", list, fyne.NewSize(320, 540))
	emptyVaultOptions := entryListFixture()
	emptyVaultOptions.Entries = nil
	capture("entry-list-empty-vault", NewEntryList(emptyVaultOptions))
	searchEmptyOptions := entryListFixture()
	searchEmptyOptions.Search = "not-found"
	capture("entry-list-search-empty", NewEntryList(searchEmptyOptions))
	categoryEmptyOptions := entryListFixture()
	categoryEmptyOptions.Category = "password"
	categoryEmptyOptions.Entries = []EntryRow{{Title: "GitHub", Category: "login", Icon: icons.Login, Open: func() {}}}
	capture("entry-list-category-empty", NewEntryList(categoryEmptyOptions))
	window.Resize(fyne.NewSize(320, 540))
	captureUIPilotScreen(t, window, outputDir, "entry-list-short-window"+suffix)
	for _, menu := range []struct {
		name  string
		label string
	}{{"vault-selector-menu-active", "Select vault"}, {"entry-add-menu", "Add entry"}, {"vault-actions-menu", "Vault actions"}} {
		window.SetContent(list)
		test.Tap(findPilotIconButton(t, list, menu.label))
		captureCurrent(menu.name, fyne.NewSize(320, 540))
		closePilotOverlay(window)
	}

	window.SetContent(list)
	categoryLabel := findPilotLabel(t, list, "All items")
	chooseEntryCategory(t, driver, window, categoryLabel, "Logins")
	require.Equal(t, "Logins", findPilotLabel(t, list, "Logins").Text)
	require.Equal(t, 2, findPilotList(t, list).Length())
	selectedOptions := entryListFixture()
	selectedOptions.Category = "login"
	selectedList := NewEntryList(selectedOptions)
	capture("entry-list-category-selected", selectedList)
	categoryLabel = findPilotLabel(t, selectedList, "Logins")
	window.SetContent(selectedList)
	position := driver.Driver().AbsolutePositionForObject(categoryLabel).Add(fyne.NewPos(categoryLabel.Size().Width/2, categoryLabel.Size().Height/2))
	test.TapCanvas(window.Canvas(), position)
	captureCurrent("entry-category-menu-selected", fyne.NewSize(440, 800))
	closePilotOverlay(window)
	findPilotEntry(t, list, "Search entries…").(*widget.Entry).SetText("github")
	require.Equal(t, 1, findPilotList(t, list).Length())
	searchOptions := entryListFixture()
	searchOptions.Category = "login"
	searchOptions.Search = "github"
	searchList := NewEntryList(searchOptions)
	capture("entry-list-search", searchList)

	navigation := findPilotNavigation(t, list)
	codeContext, cancelCodes := context.WithCancel(context.Background())
	codesView := newCodesView(codeContext, []codesEntry{
		{Title: "GitHub", Username: "dev@example.com", ItemID: "github", Icon: githubFavicon, Credential: testTOTPCredential()},
		{Title: "Google", Username: "user@gmail.com", ItemID: "google", Icon: icons.Globe, Credential: testTOTPCredential()},
		{Title: "GitLab", Username: "dev@example.com", ItemID: "gitlab", Icon: icons.Globe, Credential: testTOTPCredential()},
	}, nil)
	codesOptions := entryListFixture()
	codesOptions.Codes = codesView.View
	codesList := NewEntryList(codesOptions)
	window.SetContent(codesList)
	window.Resize(fyne.NewSize(440, 800))
	codesNavigation := findPilotNavigation(t, window.Content())
	test.Tap(codesNavigation.buttons[1].button)
	require.Equal(t, 1, codesNavigation.Selected)
	captureUIPilotScreen(t, window, outputDir, "codes"+suffix)
	window.SetContent(codesList)
	window.Resize(fyne.NewSize(320, 540))
	codesNavigation = findPilotNavigation(t, window.Content())
	codesNavigation.Select(0)
	codesNavigation.Select(1)
	captureUIPilotScreen(t, window, outputDir, "codes-list-narrow"+suffix)
	test.Tap(findPilotIconButton(t, codesList, "Vault actions"))
	captureCurrent("codes-actions-menu", fyne.NewSize(320, 540))
	closePilotOverlay(window)
	emptyContext, cancelEmpty := context.WithCancel(context.Background())
	emptyCodes := newCodesView(emptyContext, nil, nil)
	emptyOptions := entryListFixture()
	emptyOptions.Codes = emptyCodes.View
	emptyList := NewEntryList(emptyOptions)
	window.SetContent(emptyList)
	window.Resize(fyne.NewSize(440, 800))
	emptyNavigation := findPilotNavigation(t, window.Content())
	test.Tap(emptyNavigation.buttons[1].button)
	captureUIPilotScreen(t, window, outputDir, "codes-empty"+suffix)
	cancelEmpty()
	cancelCodes()
	navigation.Select(2)

	login := loginEditorFixture(t)
	login.Metadata.Favicon = githubFavicon
	password, err := model.NewPassword("Home Wi-Fi", "synthetic-password", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	note, err := model.NewNote("Recovery note", "Synthetic note for UI review.")
	require.NoError(t, err)
	sshItem, _ := sshDetailFixture(t)

	for _, item := range []*model.Item{login, password, note, sshItem} {
		ctx, cancel := context.WithCancel(context.Background())
		view, err := NewEntryDetail(ctx, item, window, func() {}, func() {}, func() {})
		require.NoError(t, err)
		capture("entry-detail-"+pilotItemName(item), view)
		if item == login {
			captureAt("entry-detail-login-totp-narrow", view, fyne.NewSize(320, 640))
			window.SetContent(view)
			test.Tap(findPilotIconButton(t, view, "Entry actions"))
			captureCurrent("entry-detail-actions-menu", fyne.NewSize(320, 540))
			closePilotOverlay(window)
		}
		if item == sshItem {
			captureAt("entry-detail-ssh-key-narrow", view, fyne.NewSize(320, 640))
			test.Tap(findPilotIconButton(t, view, "Show private key"))
			captureCurrent("entry-detail-ssh-key-revealed", fyne.NewSize(320, 640))
		}
		cancel()
	}

	loginEditor := newTestLoginEditor(t, login, false)
	capture("edit-login", loginEditor.View)
	window.SetContent(loginEditor.View)
	showTOTPSettingsDialog(loginEditor, window)
	captureCurrent("totp-settings-dialog", fyne.NewSize(440, 800))
	popup := totpDialogOverlay(window)
	fyne.DoAndWait(func() {
		for _, object := range test.LaidOutObjects(popup) {
			if button, ok := object.(*widget.Button); ok && button.Text == "Cancel" {
				button.OnTapped()
				break
			}
		}
	})
	passwordEditor, err := NewPasswordEditor(password, false)
	require.NoError(t, err)
	capture("edit-password", passwordEditor.View)
	noteEditor, err := NewNoteEditor(note, false)
	require.NoError(t, err)
	capture("edit-note", noteEditor.View)
	sshEditor, err := NewSSHKeyEditor(sshItem, false)
	require.NoError(t, err)
	captureAt("edit-ssh-key", sshEditor.View, fyne.NewSize(440, 926))
	captureAt("edit-ssh-key-narrow", sshEditor.View, fyne.NewSize(320, 640))
	test.Tap(sshEditor.RevealPrivateButton)
	capture("edit-ssh-key-private-revealed", sshEditor.View)

	appState := &model.AppState{Settings: model.NewDefaultSettings()}
	settings := NewSettingsView(appState, &storage.StorageMock{}, window)
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

	backupStore := &storage.StorageMock{}
	backupStore.OnVaults = func() ([]model.VaultInfo, error) { return nil, nil }
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
	backupStore.OnVaults = func() ([]model.VaultInfo, error) {
		return []model.VaultInfo{{Name: "Personal"}}, nil
	}
	backupFlow.review(backup, "")
	captureCurrent("restore-review-collision", fyne.NewSize(440, 800))
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, "Personal")
	require.NoError(t, err)
	backupFlow.complete(vault)
	captureCurrent("restore-complete", fyne.NewSize(440, 800))

	exportView := newExportCredentialsView(window, &storage.StorageMock{}, vault, "Personal", func() {})
	window.SetContent(exportView.View)
	captureCurrent("export-encrypted-default", fyne.NewSize(440, 800))
	exportView.Mode.SetSelected(exportPlaintextLabel)
	captureCurrent("export-plaintext-warning", fyne.NewSize(440, 800))

	passwordItem, err := model.NewPassword("Example password", "", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	passwordEditor, err = NewPasswordEditor(passwordItem, true)
	require.NoError(t, err)
	window.SetContent(passwordEditor.View)
	passwordSettings := model.NewDefaultSettings().Password
	generator := NewPasswordGenerator(key, passwordSettings)
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
		result := passwordGeneratorEntry(t, popup).Text
		if mode == paw.PassphrasePassword {
			words := strings.Split(result, "-")
			require.Len(t, words, passwordSettings.Passphrase.DefaultLength)
			for _, word := range words {
				require.NotEmpty(t, word)
				require.Empty(t, strings.Trim(word, "abcdefghijklmnopqrstuvwxyz"))
			}
		} else if mode == paw.PinPassword {
			require.Regexp(t, `^\d+$`, result)
		} else {
			require.Len(t, result, passwordSettings.Random.DefaultLength)
		}
		captureCurrent("password-generator-"+pilotModeName(mode.String()), fyne.NewSize(440, 800))
		closePilotOverlay(window)
	}
}

func newPilotAudit(state string) *PasswordAudit {
	audit := NewPasswordAudit("Personal", nil, func(fyne.CanvasObject) {}, func(string) {}, func() {})
	audit.result = auditFixture()
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
		audit.showFindings("weak")
	} else if state == "exposed-findings" {
		audit.result.Findings = append(audit.result.Findings,
			AuditFinding{AuditEntry: AuditEntry{ID: "forum", Title: "Old Forum", Subtitle: "user@example.com", Icon: icons.Login}, Exposed: true, ExposedCount: 832},
			AuditFinding{AuditEntry: AuditEntry{ID: "shop", Title: "Shopping Account", Subtitle: "shop@example.com", Icon: icons.Login}, Exposed: true, ExposedCount: 3},
		)
		audit.showFindings("exposed")
	} else if state == "reused-findings" {
		audit.showFindings("reused")
	}
	return audit
}

func findPilotIconButton(t *testing.T, view fyne.CanvasObject, label string) *iconButton {
	t.Helper()
	for _, object := range test.LaidOutObjects(view) {
		if button, ok := object.(*iconButton); ok && button.AccessibilityLabel() == label {
			return button
		}
	}
	t.Fatalf("icon button %q is not rendered", label)
	return nil
}

func findPilotLabel(t *testing.T, view fyne.CanvasObject, text string) *widget.Label {
	t.Helper()
	for _, object := range test.LaidOutObjects(view) {
		if label, ok := object.(*widget.Label); ok && label.Text == text {
			return label
		}
	}
	t.Fatalf("label %q is not rendered", text)
	return nil
}

func findPilotEntry(t *testing.T, view fyne.CanvasObject, placeholder string) fyne.CanvasObject {
	t.Helper()
	for _, object := range test.LaidOutObjects(view) {
		if entry, ok := object.(*widget.Entry); ok && entry.PlaceHolder == placeholder {
			return entry
		}
	}
	t.Fatalf("entry with placeholder %q is not rendered", placeholder)
	return nil
}

func findPilotNavigation(t *testing.T, view fyne.CanvasObject) *Navigation {
	t.Helper()
	if navigation := findNavigationContainer(view); navigation != nil {
		return navigation
	}
	t.Fatal("bottom navigation is not rendered")
	return nil
}

func findNavigationContainer(view fyne.CanvasObject) *Navigation {
	if navigation, ok := view.(*Navigation); ok {
		return navigation
	}
	container, ok := view.(*fyne.Container)
	if !ok {
		return nil
	}
	for _, child := range container.Objects {
		if navigation := findNavigationContainer(child); navigation != nil {
			return navigation
		}
	}
	return nil
}

func findPilotList(t *testing.T, view fyne.CanvasObject) *widget.List {
	t.Helper()
	for _, object := range test.LaidOutObjects(view) {
		if list, ok := object.(*widget.List); ok {
			return list
		}
	}
	t.Fatal("list is not rendered")
	return nil
}

func closePilotOverlay(window fyne.Window) {
	if overlay := window.Canvas().Overlays().Top(); overlay != nil {
		window.Canvas().Overlays().Remove(overlay)
	}
}

func pilotItemName(item *model.Item) string {
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

func pilotModeName(mode string) string {
	switch mode {
	case paw.PassphrasePassword.String():
		return "passphrase"
	case paw.PinPassword.String():
		return "pin"
	default:
		return "random"
	}
}
