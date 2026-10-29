// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package v2

import (
	"fmt"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/ui/v2/assets/icons"
)

func auditFixture() AuditResult {
	return AuditResult{Passwords: 3, ExposedChecked: 3, LocalComplete: true, Findings: []AuditFinding{
		{AuditEntry: AuditEntry{ID: "github", Title: "GitHub", Subtitle: "dev@example.com", Icon: icons.Login}, Score: 0, ReuseCount: 2, ReuseGroup: 1, Exposed: true},
		{AuditEntry: AuditEntry{ID: "forum", Title: "Old forum", Subtitle: "user@example.com", Icon: icons.Login}, Score: 1, ReuseCount: 2, ReuseGroup: 1},
	}}
}

func TestPasswordAuditGolden(t *testing.T) {
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		for _, state := range []string{"idle", "running", "completed", "partial", "findings", "clear", "error"} {
			t.Run(state+"/"+variantName(variant), func(t *testing.T) {
				assertViewGolden(t, "audit-"+state+"-"+variantName(variant)+".png", variant, fyne.NewSize(440, 650), func() fyne.CanvasObject {
					a := NewPasswordAudit("Personal", nil, func(fyne.CanvasObject) {}, func(string) {}, func() {})
					a.result = auditFixture()
					a.hasResult = state != "idle"
					a.running = state == "running"
					if state == "partial" {
						a.result.ExposedFailed = true
						a.result.ExposedChecked = 1
					}
					if state == "clear" || state == "error" {
						a.result.Findings = nil
					}
					if state == "error" {
						a.result.LoadErrors = 1
						a.result.LocalComplete = false
					}
					a.overview()
					if state == "findings" {
						a.showFindings("weak")
					}
					return a.View
				})
			})
		}
	}
}

func auditTexts(view fyne.CanvasObject) []string {
	var labels []string
	for _, object := range test.LaidOutObjects(view) {
		if label, ok := object.(*widget.Label); ok && label.Visible() {
			labels = append(labels, label.Text)
		}
	}
	return labels
}

func TestPasswordAuditStates(t *testing.T) {
	test.NewTempApp(t)
	for _, value := range []struct {
		result  AuditResult
		message string
	}{
		{AuditResult{LocalComplete: true}, "No passwords to audit"},
		{AuditResult{LocalComplete: true, Passwords: 1, ExposedChecked: 1}, "No issues found in completed checks"},
		{AuditResult{LocalComplete: true, Passwords: 1, ExposedFailed: true}, "Some checks were not completed."},
		{AuditResult{Cancelled: true}, "Audit cancelled. Some checks were not completed."},
		{AuditResult{LoadErrors: 1}, "Some items could not be read. Results are incomplete."},
	} {
		t.Run(value.message, func(t *testing.T) {
			a := NewPasswordAudit("Personal", nil, func(fyne.CanvasObject) {}, func(string) {}, func() {})
			a.result = value.result
			a.hasResult = true
			a.overview()
			require.Contains(t, auditTexts(a.View), value.message)
			require.Contains(t, auditTexts(a.View), fmt.Sprintf("Passwords analyzed: %d", value.result.Passwords))
		})
	}
}

func TestPasswordAuditFindingsBackAndClose(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	opened := ""
	a := NewPasswordAudit("Personal", nil, window.SetContent, func(id string) { opened = id }, func() {})
	a.result = auditFixture()
	a.hasResult = true
	for i := 0; i < 30; i++ {
		a.result.Findings = append(a.result.Findings, AuditFinding{AuditEntry: AuditEntry{ID: fmt.Sprint(i), Title: "Service", Icon: icons.Login}, Score: 0})
	}
	a.showFindings("weak")
	window.SetContent(a.View)
	window.Resize(fyne.NewSize(440, 650))
	var list *widget.List
	for _, object := range test.LaidOutObjects(a.View) {
		if value, ok := object.(*widget.List); ok {
			list = value
		}
	}
	require.NotNil(t, list)
	list.ScrollTo(15)
	list.Select(15)
	require.NotEmpty(t, opened)
	position := list.GetScrollOffset()
	a.MarkStale()
	a.DetailBack()
	require.Equal(t, position, list.GetScrollOffset())
	require.Contains(t, auditTexts(a.View), "Items changed. Audit results may be outdated. Run audit again.")
	a.Close()
	a.DetailBack()
	require.Empty(t, a.result.Findings)
	require.Empty(t, a.View.Objects)
}

func TestAuditCategoriesWithoutFindingsHaveNoAction(t *testing.T) {
	test.NewTempApp(t)
	a := NewPasswordAudit("Personal", nil, func(fyne.CanvasObject) {}, func(string) {}, func() {})
	a.hasResult = true
	a.result = AuditResult{Passwords: 1, ExposedChecked: 1, LocalComplete: true}
	a.overview()
	window := test.NewTempWindow(t, a.View)
	window.Resize(fyne.NewSize(440, 650))
	var rows []*ActionRow
	for _, object := range test.LaidOutObjects(a.View) {
		if row, ok := object.(*ActionRow); ok {
			rows = append(rows, row)
			require.Nil(t, row.button.OnTapped)
			require.False(t, row.button.Visible())
			require.False(t, row.trailing.Visible())
			require.True(t, row.leading.Visible())
			require.True(t, row.title.Visible())
			require.True(t, row.detail.Visible())
		}
	}
	require.Len(t, rows, 3)
	texts := auditTexts(a.View)
	for _, category := range auditCategories {
		require.Contains(t, texts, category.title+" · 0")
	}
}

func auditList(t *testing.T, view fyne.CanvasObject) *widget.List {
	t.Helper()
	for _, object := range test.LaidOutObjects(view) {
		if list, ok := object.(*widget.List); ok {
			return list
		}
	}
	t.Fatal("missing audit list")
	return nil
}

func TestReusedPasswordGroupsOpenAllMembers(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	var opened []string
	a := NewPasswordAudit("Personal", nil, window.SetContent, func(id string) { opened = append(opened, id) }, func() {})
	a.result = auditFixture()
	a.result.Findings = append(a.result.Findings,
		AuditFinding{AuditEntry: AuditEntry{ID: "third", Title: "Third", Icon: icons.Login}, Score: 4, ReuseCount: 2, ReuseGroup: 2},
		AuditFinding{AuditEntry: AuditEntry{ID: "fourth", Title: "Fourth", Icon: icons.Login}, Score: 4, ReuseCount: 2, ReuseGroup: 2},
	)
	a.showFindings("reused")
	window.SetContent(a.View)
	window.Resize(fyne.NewSize(440, 650))
	groups := auditList(t, a.View)
	require.Equal(t, 2, groups.Length())
	groups.Select(0)
	require.Empty(t, opened, "selecting a group must open its item list")
	members := auditList(t, a.View)
	require.Equal(t, 2, members.Length())
	members.Select(0)
	a.DetailBack()
	require.Same(t, members, auditList(t, a.View))
	members.Select(1)
	a.DetailBack()
	require.Equal(t, []string{"github", "forum"}, opened)
	// Return from members to groups through the native Back button.
	for _, object := range test.LaidOutObjects(a.View) {
		if button, ok := object.(*widget.Button); ok && button.Icon == icons.Back {
			test.Tap(button)
			break
		}
	}
	require.Same(t, groups, auditList(t, a.View))
	groups.Select(1)
	members = auditList(t, a.View)
	require.Equal(t, 2, members.Length())
	members.Select(0)
	a.DetailBack()
	members.Select(1)
	require.Equal(t, []string{"github", "forum", "third", "fourth"}, opened)
}
