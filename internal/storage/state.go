// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	json "encoding/json/v2"
	"fmt"
	"io"
	"os"
	"time"

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
		return defaultAppState, nil
	}
	if err != nil {
		return defaultAppState, fmt.Errorf("could not read URI: %w", err)
	}
	defer r.Close()
	appState := &model.AppState{}
	err = json.UnmarshalRead(r, appState)
	if err != nil {
		return defaultAppState, err
	}
	return appState, nil
}

// StoreAppState store the configuration into the underlying storage
func (s *FileStorage) StoreAppState(appState *model.AppState) error {
	return atomicWrite(s.root, appStateFileName, func(w io.Writer) error {
		return json.MarshalWrite(w, appState, json.Deterministic(true), json.FormatNilSliceAsNull(true))
	})
}
