// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package browser

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"text/template"
)

const nativeMessagingManifestFileName = "paw.json"

const firefoxExtensionIDs = `["paw@lucor.dev"]`
const firefoxManifestTpl = `{
	"name": "paw",
	"description": "Native manifest for the Paw browser extension",
	"path": "{{ .Path }}",
	"type": "stdio",
	"allowed_extensions": {{ .ExtensionIDs }}
}
`

const chromeExtensionIDs = `["chrome-extension://lkncfaojhcgoefgkjpfoniakecdiclof/"]`
const chromeManifestTpl = `{
	"name": "paw",
	"description": "Native manifest for the Paw browser extension",
	"path": "{{ .Path }}",
	"type": "stdio",
	"allowed_origins": {{ .ExtensionIDs }}
}
`

type manifestTplData struct {
	Path         string
	ExtensionIDs string
}

func getPawExecutablePath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Abs(exePath)
}

// WriteNativeManifests writes native manifests
// Currently only chrome and firefox on linux are supported.
// TODO add support for macOS, windows and mobile.
func WriteNativeManifests() error {
	pawPath, err := getPawExecutablePath()
	if err != nil {
		return err
	}

	firefoxData := manifestTplData{Path: pawPath, ExtensionIDs: firefoxExtensionIDs}
	firefoxNativeManifestLocations, err := firefoxNativeManifestLocations()
	if err != nil {
		return err
	}
	writeNativeManifest(firefoxManifestTpl, firefoxData, firefoxNativeManifestLocations)

	chromeData := manifestTplData{Path: pawPath, ExtensionIDs: chromeExtensionIDs}
	chromeNativeManifestLocations, err := chromeNativeManifestLocations()
	if err != nil {
		return err
	}
	writeNativeManifest(chromeManifestTpl, chromeData, chromeNativeManifestLocations)
	return nil
}

func writeNativeManifest(tpl string, data manifestTplData, locations []string) error {
	tmpl, err := template.New("manifest").Parse(tpl)
	if err != nil {
		return err
	}

	for _, location := range locations {

		_ = os.MkdirAll(filepath.Dir(location), 0700)

		file, err := os.Create(location)
		if err != nil {
			return err
		}
		defer file.Close()

		err = tmpl.Execute(file, data)
		if err != nil {
			return err
		}
	}

	return nil
}

// chromeNativeManifestLocations defines the native manifest locations for chrome/chromium
// see: https://developer.chrome.com/docs/extensions/develop/concepts/native-messaging
// TODO: handle darwin and windows
func chromeNativeManifestLocations() ([]string, error) {
	if runtime.GOOS == "darwin" {
		return []string{}, nil
	}

	if runtime.GOOS == "windows" {
		return []string{}, nil
	}

	// fallback to linux and *nix OSes
	uhd, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not get the user home directory: %w", err)
	}

	return []string{
		filepath.Join(uhd, ".config/google-chrome/NativeMessagingHosts", nativeMessagingManifestFileName),
		filepath.Join(uhd, ".config/chromium/NativeMessagingHosts", nativeMessagingManifestFileName),
	}, nil
}

// firefoxNativeManifestLocations defines the native manifest locations for firefox
// See: https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/Native_manifests
// TODO: handle darwin and windows
func firefoxNativeManifestLocations() ([]string, error) {
	if runtime.GOOS == "darwin" {
		return []string{}, nil
	}

	if runtime.GOOS == "windows" {
		return []string{}, nil
	}

	// fallback to linux and *nix OSes
	uhd, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not get the user home directory: %w", err)
	}

	return []string{
		filepath.Join(uhd, ".mozilla/native-messaging-hosts", nativeMessagingManifestFileName),
	}, nil
}
