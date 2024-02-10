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
