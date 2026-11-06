// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	json "encoding/json/v2"
	"fmt"
	"regexp"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

func TestCodesViewCountCopyNavigationAndClear(t *testing.T) {
	for _, count := range []int{1, 2} {
		t.Run(codeCountLabel(count), func(t *testing.T) {
			application := test.NewTempApp(t)
			window := test.NewTempWindow(t, nil)
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			opener := &testCodeEntryOpener{}
			entries := make([]codesEntry, count)
			for index := range entries {
				entries[index] = codesEntry{Title: "GitHub", Username: "dev@example.com", ItemID: fmt.Sprintf("entry-%d", index), Icon: icons.Globe, Credential: testTOTPCredential()}
			}
			view := newCodesView(ctx, entries, opener)
			window.SetContent(view.View)
			window.Resize(fyne.NewSize(440, 720))
			require.Contains(t, auditTexts(view.View), codeCountLabel(count))
			require.Contains(t, auditTexts(view.View), "GitHub")
			require.Contains(t, auditTexts(view.View), "dev@example.com")
			require.Len(t, view.displays, count)

			var copyButton *iconButton
			var progress *widget.ProgressBar
			var rows []*ActionRow
			for _, object := range test.LaidOutObjects(view.View) {
				switch object := object.(type) {
				case *iconButton:
					if object.AccessibilityLabel() == "Copy verification code" {
						copyButton = object
					}
				case *widget.ProgressBar:
					progress = object
				case *ActionRow:
					rows = append(rows, object)
				}
			}
			require.NotNil(t, copyButton)
			require.NotNil(t, progress)
			require.Len(t, rows, count)
			require.Regexp(t, `^[0-9]+s$`, progress.TextFormatter())
			test.Tap(copyButton)
			require.Regexp(t, `^[0-9]{6}$`, application.Clipboard().Content())
			require.Empty(t, opener.opened, "copying a code must not navigate to the entry")
			test.Tap(rows[0].button)
			require.Equal(t, "entry-0", opener.opened)

			view.Clear()
			require.Empty(t, view.displays[0].Value())
			require.Empty(t, view.displays[0].settings.Secret)
		})
	}
}

func TestCodesSectionReusesVaultHeaderAndHidesAddAction(t *testing.T) {
	test.NewTempApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	codes := newCodesView(ctx, []codesEntry{{Title: "GitHub", Credential: testTOTPCredential()}}, nil)
	options := entryListFixture()
	options.Codes = codes.View
	view := NewEntryList(options)
	test.NewTempWindow(t, view)
	navigation := findPilotNavigation(t, view)
	navigation.Select(1)
	var vaultSelector, vaultActions, addHidden, fallbackIcon bool
	for _, object := range test.LaidOutObjects(view) {
		if row, ok := object.(*ActionRow); ok && row.title.Text == "GitHub" {
			fallbackIcon = row.leading.Resource == icons.Globe
		}
		if button, ok := object.(*iconButton); ok {
			switch button.AccessibilityLabel() {
			case "Select vault":
				vaultSelector = true
			case "Vault actions":
				vaultActions = true
			case "Add entry":
				addHidden = button.Hidden
			}
		}
	}
	require.True(t, vaultSelector)
	require.True(t, vaultActions)
	require.True(t, addHidden, "Codes must hide the Add entry action")
	require.True(t, fallbackIcon, "missing entry icons use the globe fallback")
	require.Equal(t, 1, navigation.Selected)
	require.True(t, navigation.buttons[1].selected)
	renderer := test.WidgetRenderer(navigation.buttons[1]).(*navigationButtonRenderer)
	require.Equal(t, icons.CodesActive, renderer.icon.Resource)
	require.Contains(t, auditTexts(view), "Verification codes")
}

func TestCodesViewEmptyState(t *testing.T) {
	app := test.NewTempApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	view := newCodesView(ctx, nil, nil)
	window := test.NewTempWindow(t, view.View)
	window.Resize(fyne.NewSize(440, 720))
	window.Show()
	texts := auditTexts(view.View)
	require.Contains(t, texts, "0 codes")
	require.Contains(t, texts, "No verification codes")
	require.Contains(t, texts, "Verification codes from your login items will appear here.")
	for _, object := range test.LaidOutObjects(view.View) {
		label, ok := object.(*widget.Label)
		if !ok {
			continue
		}
		switch label.Text {
		case "No verification codes":
			require.Equal(t, fyne.TextAlignCenter, label.Alignment)
			require.Equal(t, theme.SizeNameHeadingText, label.SizeName)
			position := app.Driver().AbsolutePositionForObject(label)
			require.Greater(t, position.Y, window.Canvas().Size().Height/3, "the empty message is centered in the available Codes content")
		case "Verification codes from your login items will appear here.":
			require.Equal(t, fyne.TextAlignCenter, label.Alignment)
			require.Equal(t, theme.SizeNameCaptionText, label.SizeName)
		}
	}
}

func TestCodesViewNarrowLayout(t *testing.T) {
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		t.Run(variantName(variant), func(t *testing.T) {
			app := test.NewTempApp(t)
			app.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			view := newCodesView(ctx, []codesEntry{{
				Title:    "A very long verification service name that should wrap safely",
				Username: "a.very.long.account.name@example.test", Icon: icons.Globe,
				Credential: testTOTPCredential(),
			}}, nil)
			window := test.NewTempWindow(t, view.View)
			window.Show()
			window.Resize(fyne.NewSize(320, 620))
			var code *widget.Label
			var progress *widget.ProgressBar
			var copyButton *iconButton
			for _, object := range test.LaidOutObjects(view.View) {
				switch object := object.(type) {
				case *widget.Label:
					if regexp.MustCompile(`^[0-9]{3} [0-9]{3}$`).MatchString(object.Text) {
						code = object
					}
				case *widget.ProgressBar:
					progress = object
				case *iconButton:
					if object.AccessibilityLabel() == "Copy verification code" {
						copyButton = object
					}
				}
			}
			require.NotNil(t, code)
			require.NotNil(t, progress)
			require.NotNil(t, copyButton)
			require.Greater(t, progress.Size().Width, float32(0))
			require.InDelta(t, progress.MinSize().Height, progress.Size().Height, 0.1, "the progress bar keeps its native minimum height")
			progressCenter := app.Driver().AbsolutePositionForObject(progress).Y + progress.Size().Height/2
			copyCenter := app.Driver().AbsolutePositionForObject(copyButton).Y + copyButton.Size().Height/2
			require.InDelta(t, copyCenter, progressCenter, 3, "the native progress bar and Copy action should share a row")
			driver := app.Driver()
			codePosition := driver.AbsolutePositionForObject(code)
			progressPosition := driver.AbsolutePositionForObject(progress)
			copyPosition := driver.AbsolutePositionForObject(copyButton)
			require.GreaterOrEqual(t, progressPosition.X, codePosition.X+code.Size().Width)
			require.GreaterOrEqual(t, copyPosition.X, progressPosition.X+progress.Size().Width)
			require.LessOrEqual(t, copyPosition.X+copyButton.Size().Width, float32(320))
		})
	}
}

type testCodeEntryOpener struct{ opened string }

func (o *testCodeEntryOpener) openCodeEntry(itemID string) { o.opened = itemID }

func TestCodesViewUsesActiveVaultAndLockClearsSecrets(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	firstLogin := codeLoginFixture(t, "GitHub", true)
	firstLogin.Metadata.Favicon = &model.Favicon{Host: "github.com"}
	secondLogin := codeLoginFixture(t, "Other login", false)
	firstVault := codeVaultFixture(t, "Personal", firstLogin, secondLogin)
	workLogin := codeLoginFixture(t, "Work Git", true)
	secondVault := codeVaultFixture(t, "Work", workLogin)
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	for _, vault := range []*model.Vault{firstVault, secondVault} {
		require.NoError(t, store.UpdateVaultIdentity(vault, vault.Metadata.VaultName, model.DefaultVaultAppearance()))
	}
	require.NoError(t, store.StoreItem(firstVault, firstLogin))
	require.NoError(t, store.StoreItem(firstVault, secondLogin))
	require.NoError(t, store.StoreItem(secondVault, workLogin))
	application := &app{win: window, main: container.NewVScroll(container.NewVBox()), storage: store, filter: make(map[string]*vaultFilterOptions), unlockedVault: map[string]*model.Vault{}}
	application.setVaultView(firstVault, firstVault.Accounts[0].ID)
	firstCodes := application.codesView
	require.Len(t, firstCodes.displays, 1, "entries without TOTP are excluded")
	require.Contains(t, auditTexts(firstCodes.View), "1 code")
	require.Contains(t, auditTexts(firstCodes.View), "GitHub")
	require.Contains(t, auditTexts(firstCodes.View), "dev@example.com")
	require.Equal(t, icons.Login, firstCodes.entries[0].Icon, "an empty favicon uses the login fallback icon")
	window.SetContent(application.main)
	navigation := findPilotNavigation(t, application.main.Content)
	navigation.Select(1)
	application.openCodeEntry(firstLogin.ID)
	require.NotSame(t, application.main, window.Content())
	test.Tap(findPilotIconButton(t, window.Content(), "Back"))
	require.Same(t, application.main, window.Content(), "Back restores the existing shell and scroll state")
	require.Same(t, firstCodes, application.codesView)
	require.Equal(t, 1, navigation.Selected)

	application.setVaultView(secondVault, secondVault.Accounts[0].ID)
	require.Empty(t, firstCodes.displays[0].settings.Secret, "switching vault clears the prior vault's TOTP")
	require.Len(t, application.codesView.displays, 1)
	require.Contains(t, auditTexts(application.codesView.View), "1 code")

	secondCodes := application.codesView
	application.lockVault()
	require.Empty(t, secondCodes.displays[0].settings.Secret, "locking the vault clears its TOTP")
}

func codeLoginFixture(t *testing.T, title string, withTOTP bool) *model.Item {
	t.Helper()
	item, err := model.NewLogin(title, "dev@example.com", "synthetic-password", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	if withTOTP {
		credential := testTOTPCredential()
		credential.Username = "dev@example.com"
		raw, err := json.Marshal(credential, json.Deterministic(true))
		require.NoError(t, err)
		item.Credentials = append(item.Credentials, raw)
	}
	return item
}

func codeVaultFixture(t *testing.T, name string, items ...*model.Item) *model.Vault {
	t.Helper()
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, name)
	require.NoError(t, err)
	for _, item := range items {
		summary, err := item.Summary()
		require.NoError(t, err)
		vault.Accounts[0].Items = append(vault.Accounts[0].Items, *summary)
	}
	return vault
}

func testTOTPCredential() cxf.TOTP {
	return cxf.TOTP{Type: cxf.CredentialTypeTOTP, Secret: "JBSWY3DPEHPK3PXP", Algorithm: "sha1", Period: 30, Digits: 6}
}
