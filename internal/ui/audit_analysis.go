// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"runtime"
	"sort"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/ui/assets/icons"
	"go.lucor.dev/zxcvbn"
)

// AuditEntry identifies an item without retaining its credentials.
type AuditEntry struct {
	// ID is the stable vault item identifier.
	ID string
	// Title is the display name shown in findings.
	Title string
	// Subtitle is the optional secondary item label.
	Subtitle string
	// Icon is the item type or favicon resource shown in findings.
	Icon fyne.Resource
}

// AuditFinding records password problems without storing the password itself.
type AuditFinding struct {
	// AuditEntry identifies the affected vault item.
	AuditEntry
	// Score is the zxcvbn score from 0 (weakest) to 4 (strongest).
	Score int
	// ReuseCount is the number of audited items sharing this password.
	ReuseCount int
	// ReuseGroup identifies items sharing a password within this audit; zero means no group.
	ReuseGroup int
	// Exposed reports whether HIBP found this password in breach data.
	Exposed bool
	// ExposedCount is the number of HIBP occurrences for this password.
	ExposedCount int
}

// AuditResult distinguishes completed checks from unavailable or cancelled checks.
type AuditResult struct {
	// Findings contains distinct items with at least one detected problem.
	Findings []AuditFinding
	// Passwords is the number of non-empty passwords loaded for analysis.
	Passwords int
	// ExposedChecked is the number of passwords with a successful HIBP response.
	ExposedChecked int
	// LoadErrors is the number of item credentials that could not be loaded.
	LoadErrors int
	// LocalComplete reports whether all candidate passwords were loaded for local checks.
	LocalComplete bool
	// ExposedFailed reports that at least one HIBP request failed.
	ExposedFailed bool
	// Cancelled reports that the audit context was cancelled before all checks completed.
	Cancelled bool
}

// AuditStore loads decrypted items from the selected vault.
type AuditStore interface {
	LoadItem(*model.Vault, string) (*model.Item, error)
}

// BreachChecker checks a password's SHA-1 digest against breach data.
type BreachChecker interface {
	SearchHash(context.Context, [sha1.Size]byte) (bool, int, error)
}

// AuditProgress reports the active audit phase and completed breach checks.
type AuditProgress struct {
	// Done is the number of successful HIBP lookups.
	Done int
	// Total is the number of non-empty passwords loaded for analysis.
	Total int
	// Phase identifies which audit phase is reporting progress.
	Phase AuditProgressPhase
}

// AuditProgressPhase identifies the active stage of a password audit.
type AuditProgressPhase uint8

const (
	// AuditPhaseBreaches reports progress during HIBP lookups.
	AuditPhaseBreaches AuditProgressPhase = iota + 1
)

// PasswordAuditor analyzes the supported password fields in one vault.
type PasswordAuditor struct {
	vault    *model.Vault
	store    AuditStore
	breaches BreachChecker
}

// NewPasswordAuditor binds the audit to a vault, item store, and breach client.
func NewPasswordAuditor(vault *model.Vault, store AuditStore, breaches BreachChecker) *PasswordAuditor {
	return &PasswordAuditor{vault: vault, store: store, breaches: breaches}
}

// Run performs local checks before looking up breach exposure.
// If progress is non-nil, its receiver must drain it until Run closes it.
// Cancellation also unblocks progress delivery.
func (a *PasswordAuditor) Run(ctx context.Context, progress chan<- AuditProgress) AuditResult {
	if progress != nil {
		defer close(progress)
	}
	summaries, err := a.vault.FilterItems("", 0)
	if err != nil {
		return AuditResult{LoadErrors: 1}
	}
	type candidate struct {
		finding AuditFinding
		hash    [sha1.Size]byte
	}
	var candidates []candidate
	defer func() { clear(candidates) }()
	result := AuditResult{}
	counts := map[[sha1.Size]byte]int{}
	seen := map[string]bool{}
	for _, summary := range summaries {
		if ctx.Err() != nil {
			result.Cancelled = true
			break
		}
		kind, err := model.ItemTypeOf(&summary)
		if err != nil {
			result.LoadErrors++
			continue
		}
		if kind != model.PasswordItemType && kind != model.LoginItemType {
			continue
		}
		if seen[summary.ID] {
			continue
		}
		seen[summary.ID] = true
		item, err := a.store.LoadItem(a.vault, summary.ID)
		if err != nil {
			result.LoadErrors++
			continue
		}
		password := ""
		for _, raw := range item.Credentials {
			var credential cxf.BasicAuth
			if json.Unmarshal(raw, &credential) == nil && credential.Password != nil {
				password = credential.Password.Value
				break
			}
		}
		if password == "" {
			continue
		}
		// Reuse HIBP's SHA-1 digest for grouping; do not retain plaintext for remote checks.
		hash := sha1.Sum([]byte(password))
		counts[hash]++
		entry := AuditEntry{ID: summary.ID, Title: summary.Title, Icon: icons.Password}
		if summary.Subtitle != nil {
			entry.Subtitle = *summary.Subtitle
		}
		if kind == model.LoginItemType {
			entry.Icon = icons.Login
			if item.Metadata.Favicon != nil {
				entry.Icon = item.Metadata.Favicon
			}
		}
		finding := AuditFinding{AuditEntry: entry, Score: zxcvbn.PasswordStrength(password, nil).Score}
		candidates = append(candidates, candidate{finding: finding, hash: hash})
	}
	result.Passwords = len(candidates)
	result.LocalComplete = !result.Cancelled && result.LoadErrors == 0
	groups := map[[sha1.Size]byte]int{}
	indices := make([]int, len(candidates))
	for index := range indices {
		indices[index] = index
	}
	sort.Slice(indices, func(i, j int) bool {
		left, right := candidates[indices[i]].finding, candidates[indices[j]].finding
		if leftTitle, rightTitle := strings.ToLower(left.Title), strings.ToLower(right.Title); leftTitle != rightTitle {
			return leftTitle < rightTitle
		}
		if leftSubtitle, rightSubtitle := strings.ToLower(left.Subtitle), strings.ToLower(right.Subtitle); leftSubtitle != rightSubtitle {
			return leftSubtitle < rightSubtitle
		}
		return left.ID < right.ID
	})
	for _, i := range indices {
		if result.LocalComplete {
			hash := candidates[i].hash
			candidates[i].finding.ReuseCount = counts[hash]
			if counts[hash] > 1 {
				if groups[hash] == 0 {
					groups[hash] = len(groups) + 1
				}
				candidates[i].finding.ReuseGroup = groups[hash]
			}
		}
	}
	// Stop scheduling remote checks after a failure, retaining completed lookups.
	if progress != nil && len(candidates) > 0 {
		select {
		case progress <- AuditProgress{Total: len(candidates), Phase: AuditPhaseBreaches}:
		case <-ctx.Done():
		}
	}
	var mu sync.Mutex
	next := 0
	var workers sync.WaitGroup
	for range min(runtime.NumCPU(), len(candidates)) {
		workers.Go(func() {
			for {
				mu.Lock()
				if ctx.Err() != nil || result.ExposedFailed || next == len(candidates) {
					mu.Unlock()
					return
				}
				i := next
				next++
				mu.Unlock()
				exposed, occurrences, err := a.breaches.SearchHash(ctx, candidates[i].hash)
				mu.Lock()
				if err != nil {
					result.ExposedFailed = true
				} else {
					result.ExposedChecked++
					candidates[i].finding.Exposed = exposed
					candidates[i].finding.ExposedCount = occurrences
				}
				if progress != nil {
					select {
					case progress <- AuditProgress{Done: result.ExposedChecked, Total: len(candidates), Phase: AuditPhaseBreaches}:
					case <-ctx.Done():
					}
				}
				mu.Unlock()
			}
		})
	}
	workers.Wait()
	result.Cancelled = result.Cancelled || ctx.Err() != nil
	for _, candidate := range candidates {
		finding := candidate.finding
		if finding.Exposed || finding.Score <= 1 || finding.ReuseCount > 1 {
			result.Findings = append(result.Findings, finding)
		}
	}
	return result
}
