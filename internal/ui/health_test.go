package ui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHealthService(t *testing.T) {
	t.Run("service not available", func(t *testing.T) {
		status := HealthServiceCheck()
		require.False(t, status)
	})

	t.Run("service available", func(t *testing.T) {
		go HealthService()
		time.Sleep(5 * time.Millisecond)
		status := HealthServiceCheck()
		require.True(t, status)
	})
}
