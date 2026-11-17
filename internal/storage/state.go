// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	json "encoding/json/v2"
	"fmt"
	"io"
	"os"
	"time"

	"go.lucor.dev/paw/internal/devicename"
	"go.lucor.dev/paw/internal/model"
)

// LoadAppState load the configuration from the underlying storage
func (s *FileStorage) LoadAppState() (*model.AppState, error) {
	defaultAppState := &model.AppState{
		Modified: time.Now().UTC(),
		Settings: model.NewDefaultSettings(),
	}
	r, err := s.root.Open(appStateFileName)
	if os.IsNotExist(err) {
		return s.ensureDeviceName(defaultAppState)
	}
	if err != nil {
		return defaultAppState, fmt.Errorf("could not read URI: %w", err)
	}
	appState := &model.AppState{}
	err = json.UnmarshalRead(r, appState)
	closeErr := r.Close()
	if err != nil {
		return defaultAppState, err
	}
	if closeErr != nil {
		return defaultAppState, fmt.Errorf("close app state: %w", closeErr)
	}
	return s.ensureDeviceName(appState)
}

// ensureDeviceName initializes the local name once and saves it with the settings.
func (s *FileStorage) ensureDeviceName(state *model.AppState) (*model.AppState, error) {
	if state.Settings == nil {
		state.Settings = model.NewDefaultSettings()
	}
	if state.Settings.DeviceName == "" {
		state.Settings.DeviceName = devicename.Detect()
		if err := s.StoreAppState(state); err != nil {
			return state, fmt.Errorf("save device name: %w", err)
		}
	}
	return state, nil
}

// StoreAppState store the configuration into the underlying storage
func (s *FileStorage) StoreAppState(appState *model.AppState) error {
	return atomicWrite(s.root, appStateFileName, func(w io.Writer) error {
		return json.MarshalWrite(w, appState, json.Deterministic(true), json.FormatNilSliceAsNull(true))
	})
}
