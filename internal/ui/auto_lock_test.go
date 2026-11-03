package ui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
)

func TestAutoLockOptionsUseSupportedKeys(t *testing.T) {
	for _, label := range autoLockLabels() {
		key := autoLockKey(label)
		require.Equal(t, label, autoLockLabel(key))
		if label != "Never" {
			_, ok := autoLockByKey(key)
			require.True(t, ok, label)
		}
	}
}

func TestAutoLockAcceptsOnlyStableKeys(t *testing.T) {
	delay, enabled := autoLockByKey("1m0s")
	require.False(t, enabled)
	require.Zero(t, delay)
	require.Equal(t, settingsAutoLockNever, autoLockLabel("1m0s"))
}

func TestBackgroundAutoLockRejectsStaleTimer(t *testing.T) {
	now := time.Now()
	locked := 0
	c := newBackgroundAutoLock(func() { locked++ })
	c.now = func() time.Time { return now }
	t.Cleanup(c.close)
	c.configure(model.AutoLockOneMinute)
	c.exitedForeground()
	old := c.generation
	c.enteredForeground()
	c.exitedForeground()
	now = now.Add(2 * time.Minute)
	c.expire(old)
	require.Zero(t, locked)
	c.expire(c.generation)
	require.Equal(t, 1, locked)
}

func TestBackgroundAutoLockRecalculatesFromBackgroundEntry(t *testing.T) {
	now := time.Now()
	locked := 0
	c := newBackgroundAutoLock(func() { locked++ })
	c.now = func() time.Time { return now }
	t.Cleanup(c.close)
	c.configure(model.AutoLockFiveMinutes)
	c.exitedForeground()
	now = now.Add(2 * time.Minute)
	c.configure(model.AutoLockOneMinute)
	require.Equal(t, 1, locked)
	c.close()
	c.expire(c.generation)
	require.Equal(t, 1, locked)
}

func TestBackgroundAutoLockLocksAfterConfiguredDeadline(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	locked := 0
	controller := newBackgroundAutoLock(func() { locked++ })
	controller.now = func() time.Time { return now }
	controller.configure(model.AutoLockOneMinute)
	controller.exitedForeground()
	now = now.Add(59 * time.Second)
	controller.enteredForeground()
	require.Zero(t, locked)

	controller.exitedForeground()
	now = now.Add(61 * time.Second)
	controller.enteredForeground()
	require.Equal(t, 1, locked)
}

func TestBackgroundAutoLockNeverCancelsPendingLock(t *testing.T) {
	now := time.Now()
	locked := 0
	controller := newBackgroundAutoLock(func() { locked++ })
	controller.now = func() time.Time { return now }
	controller.configure(model.AutoLockOneMinute)
	controller.exitedForeground()
	controller.configure(model.AutoLockNever)
	now = now.Add(2 * time.Minute)
	controller.enteredForeground()
	require.Zero(t, locked)
}
