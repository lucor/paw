// Copyright 2023 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ui

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHealthService(t *testing.T) {

	lockFile, err := os.CreateTemp("", "paw.lock")
	require.NoError(t, err)
	lockFile.Close()

	t.Run("service not available", func(t *testing.T) {
		status := HealthServiceCheck(lockFile.Name())
		require.False(t, status)
	})

	t.Run("service available", func(t *testing.T) {
		go HealthService(lockFile.Name())
		time.Sleep(5 * time.Millisecond)
		status := HealthServiceCheck(lockFile.Name())
		require.True(t, status)
	})
}
