// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package v2

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

type auditTestStore struct {
	items map[string]*model.Item
	fail  bool
}

func (s *auditTestStore) LoadItem(_ *model.Vault, id string) (*model.Item, error) {
	if s.fail {
		return nil, errors.New("missing item")
	}
	return s.items[id], nil
}

type auditTestChecker struct {
	exposed [sha1.Size]byte
	err     error
	started chan struct{}
}

func (c *auditTestChecker) SearchHash(ctx context.Context, hash [sha1.Size]byte) (bool, int, error) {
	if c.started != nil {
		c.started <- struct{}{}
		<-ctx.Done()
		return false, 0, ctx.Err()
	}
	return hash == c.exposed, 1, c.err
}

func auditServiceFixture(t *testing.T, passwords ...string) (*PasswordAuditor, *auditTestStore, *auditTestChecker) {
	t.Helper()
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, "Personal")
	require.NoError(t, err)
	store := &auditTestStore{items: map[string]*model.Item{}}
	for _, password := range passwords {
		item, err := model.NewPassword("Item", password, model.PasswordSettings{Mode: paw.CustomPassword})
		require.NoError(t, err)
		summary, err := item.Summary()
		require.NoError(t, err)
		vault.Accounts[0].Items = append(vault.Accounts[0].Items, *summary)
		store.items[item.ID] = item
	}
	checker := &auditTestChecker{}
	return NewPasswordAuditor(vault, store, checker), store, checker
}

func TestPasswordAuditorDistinctItems(t *testing.T) {
	strong := "9g&TQ2#zK7!mR8$wV5@xL3"
	auditor, _, checker := auditServiceFixture(t, "password", "password", strong, "")
	checker.exposed = sha1.Sum([]byte(strong))
	progress := make(chan AuditProgress, 4)
	result := auditor.Run(context.Background(), progress)
	require.Equal(t, 3, result.Passwords)
	require.Equal(t, 3, result.ExposedChecked)
	require.True(t, result.LocalComplete)
	require.Len(t, result.Findings, 3, "an item with several problems is counted once")
	require.Equal(t, 0, result.Findings[0].Score)
	require.Equal(t, 2, result.Findings[0].ReuseCount)
	require.NotZero(t, result.Findings[0].ReuseGroup)
	require.Equal(t, result.Findings[0].ReuseGroup, result.Findings[1].ReuseGroup)
	require.Zero(t, result.Findings[2].ReuseGroup)
	require.True(t, result.Findings[2].Exposed)
	var updates []AuditProgress
	for update := range progress {
		updates = append(updates, update)
	}
	require.Equal(t, []AuditProgress{{1, 3}, {2, 3}, {3, 3}}, updates)
	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "password")
	require.NotContains(t, string(encoded), strong)
}

func TestPasswordAuditorOfflineAndCancellation(t *testing.T) {
	auditor, _, checker := auditServiceFixture(t, "password", "password")
	checker.err = errors.New("offline")
	result := auditor.Run(context.Background(), nil)
	require.True(t, result.ExposedFailed)
	require.True(t, result.LocalComplete)
	require.Zero(t, result.ExposedChecked)
	require.Len(t, result.Findings, 2)
	require.Equal(t, 2, result.Findings[0].ReuseCount)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	progress := make(chan AuditProgress)
	result = auditor.Run(ctx, progress)
	require.True(t, result.Cancelled)
	require.False(t, result.LocalComplete)
	require.Empty(t, result.Findings)
	_, open := <-progress
	require.False(t, open)
}

func TestPasswordAuditorKeepsEveryReuseGroupMember(t *testing.T) {
	strong := "9g&TQ2#zK7!mR8$wV5@xL3"
	auditor, _, _ := auditServiceFixture(t, "password", strong, "password", strong)
	result := auditor.Run(context.Background(), nil)
	require.Len(t, result.Findings, 4)
	first, second := result.Findings[0].ReuseGroup, result.Findings[1].ReuseGroup
	require.NotZero(t, first)
	require.NotZero(t, second)
	require.NotEqual(t, first, second)
	require.Equal(t, first, result.Findings[2].ReuseGroup)
	require.Equal(t, second, result.Findings[3].ReuseGroup)
	for _, finding := range result.Findings {
		require.Equal(t, 2, finding.ReuseCount)
	}
}

func TestPasswordAuditorMissingAndEmpty(t *testing.T) {
	auditor, store, _ := auditServiceFixture(t, "password")
	store.fail = true
	result := auditor.Run(context.Background(), nil)
	require.Equal(t, 1, result.LoadErrors)
	require.False(t, result.LocalComplete)
	auditor, _, _ = auditServiceFixture(t, "")
	result = auditor.Run(context.Background(), nil)
	require.True(t, result.LocalComplete)
	require.Zero(t, result.Passwords)
}

func TestCancelDuringBreachLookupKeepsLocalFindings(t *testing.T) {
	auditor, _, checker := auditServiceFixture(t, "password")
	checker.started = make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan AuditResult, 1)
	go func() { finished <- auditor.Run(ctx, nil) }()
	<-checker.started
	cancel()
	result := <-finished
	require.True(t, result.Cancelled)
	require.True(t, result.LocalComplete)
	require.Len(t, result.Findings, 1)
	require.Equal(t, 0, result.Findings[0].Score)
}

func TestCancellationUnblocksProgressDelivery(t *testing.T) {
	auditor, _, checker := auditServiceFixture(t, "password")
	checker.started = make(chan struct{}, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	progress := make(chan AuditProgress)
	finished := make(chan AuditResult, 1)
	go func() { finished <- auditor.Run(ctx, progress) }()
	// Cancel an active lookup without receiving subsequent progress updates.
	<-checker.started
	cancel()
	select {
	case result := <-finished:
		require.True(t, result.Cancelled)
	case <-time.After(time.Second):
		t.Fatal("audit did not stop after cancellation")
	}
	_, open := <-progress
	require.False(t, open)
}
