// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

func auditFixture() AuditResult {
	return AuditResult{Passwords: 3, ExposedChecked: 3, LocalComplete: true, Findings: []AuditFinding{
		{AuditEntry: AuditEntry{ID: "github", Title: "GitHub", Subtitle: "dev@example.com", Icon: icons.Login}, Score: 0, ReuseCount: 2, ReuseGroup: 1, Exposed: true, ExposedCount: 12450},
		{AuditEntry: AuditEntry{ID: "forum", Title: "Old forum", Subtitle: "user@example.com", Icon: icons.Login}, Score: 1, ReuseCount: 2, ReuseGroup: 1},
	}}
}

func TestPasswordAuditGolden(t *testing.T) {
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		for _, state := range []string{"idle", "running", "completed", "partial", "no-passwords", "findings", "exposed-findings", "reused-findings", "clear", "error"} {
			t.Run(state+"/"+variantName(variant), func(t *testing.T) {
				assertViewGolden(t, "audit-"+state+"-"+variantName(variant)+".png", variant, fyne.NewSize(440, 650), func() fyne.CanvasObject {
					a := NewPasswordAudit("Personal", nil, func(fyne.CanvasObject) {}, func(string) {}, func() {})
					a.result = auditFixture()
					a.hasResult = state != "idle"
					a.running = state == "running"
					a.checkedAt = time.Date(2026, time.October, 30, 14, 15, 0, 0, time.UTC)
					if state == "partial" {
						a.result.ExposedFailed = true
						a.result.ExposedChecked = 1
					}
					if state == "no-passwords" {
						a.result = AuditResult{LocalComplete: true}
						a.checkedAt = time.Time{}
					}
					if state == "clear" || state == "error" {
						a.result.Findings = nil
					}
					if state == "error" {
						a.result.LoadErrors = 1
						a.result.LocalComplete = false
					}
					a.overview()
					if state == "running" {
						a.updateProgress(a.runID, AuditProgress{Done: 28, Total: 42, Phase: AuditPhaseBreaches})
					}
					if state == "findings" {
						a.showFindings("weak")
					}
					if state == "exposed-findings" {
						a.result.Findings = append(a.result.Findings,
							AuditFinding{AuditEntry: AuditEntry{ID: "forum", Title: "Old Forum", Subtitle: "user@example.com", Icon: icons.Login}, Exposed: true, ExposedCount: 832},
							AuditFinding{AuditEntry: AuditEntry{ID: "shop", Title: "Shopping Account", Subtitle: "shop@example.com", Icon: icons.Login}, Exposed: true, ExposedCount: 3},
						)
						a.showFindings("exposed")
					}
					if state == "reused-findings" {
						a.showFindings("reused")
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
	a := NewPasswordAudit("Personal", nil, func(fyne.CanvasObject) {}, func(string) {}, func() {})
	require.Contains(t, auditTexts(a.View), "Check your passwords")
	require.NotContains(t, auditTexts(a.View), "0 items need attention")
	require.NotContains(t, auditTexts(a.View), "Last checked")

	a.hasResult = true
	a.checkedAt = time.Date(2026, time.October, 30, 14, 15, 0, 0, time.UTC)
	a.result = AuditResult{LocalComplete: true}
	a.overview()
	require.Contains(t, auditTexts(a.View), "No passwords to audit")
	require.NotContains(t, auditTexts(a.View), "Last checked")

	a.result = AuditResult{LocalComplete: true, Passwords: 1, ExposedChecked: 1}
	a.overview()
	require.Contains(t, auditTexts(a.View), "No issues found")
	require.Contains(t, strings.Join(auditTexts(a.View), "\n"), "1 password analyzed.")
	require.Contains(t, auditTexts(a.View), "Last checked: "+a.checkedAt.Local().Format("2 Jan 2006, 15:04"))

	a.result = AuditResult{LocalComplete: true, Passwords: 3, ExposedFailed: true, Findings: []AuditFinding{
		{AuditEntry: AuditEntry{ID: "item-1"}, Score: 0},
		{AuditEntry: AuditEntry{ID: "item-1"}, Score: 1},
	}}
	a.overview()
	texts := auditTexts(a.View)
	require.Contains(t, texts, "Audit partially completed")
	require.Contains(t, texts, "1 item needs attention")
	require.Contains(t, texts, "3 passwords analyzed locally")
	require.Contains(t, texts, "0 of 3 passwords checked for breaches")
	require.NotContains(t, texts, "No issues found")
	exposedCount, exposedDescription, exposedAction := a.categorySummary("exposed")
	require.Equal(t, "—", exposedCount)
	require.Equal(t, "Not checked", exposedDescription)
	require.False(t, exposedAction)
	weakCount, _, _ := a.categorySummary("weak")
	require.Equal(t, "1", weakCount, "category counts are distinct item counts")

	a.result = AuditResult{Passwords: 2, LoadErrors: 1, ExposedChecked: 2, LocalComplete: false}
	a.overview()
	texts = auditTexts(a.View)
	require.Contains(t, texts, "Audit partially completed")
	require.Contains(t, texts, "Some items could not be read. Local results may be incomplete.")
	require.NotContains(t, texts, "No passwords to audit")
	require.NotContains(t, texts, "No issues found")

	a.result = AuditResult{Passwords: 1, ExposedChecked: 0, LocalComplete: true, Cancelled: true, Findings: []AuditFinding{
		{AuditEntry: AuditEntry{ID: "local"}, Score: 0},
	}}
	a.overview()
	texts = auditTexts(a.View)
	require.Contains(t, texts, "Audit cancelled. Results include completed checks only.")
	require.NotContains(t, texts, "No issues found")
	require.Contains(t, texts, "1 item needs attention")
}

func TestPasswordAuditProgressAndCancel(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	a := NewPasswordAudit("Personal", nil, func(fyne.CanvasObject) {}, func(string) {}, func() {})
	a.running = true
	a.overview()
	window.SetContent(a.View)
	window.Resize(fyne.NewSize(440, 650))
	require.NotNil(t, a.progressInfinite, "local analysis uses an indeterminate native progress bar")
	require.Nil(t, a.progress)
	require.Contains(t, auditTexts(a.View), "Analyzing passwords locally…")
	require.NotContains(t, auditTexts(a.View), "Last checked")

	a.updateProgress(a.runID, AuditProgress{Done: 28, Total: 42, Phase: AuditPhaseBreaches})
	require.Nil(t, a.progressInfinite)
	require.NotNil(t, a.progress)
	require.InDelta(t, 2.0/3.0, a.progress.Value, 0.001)
	require.Contains(t, auditTexts(a.View), "Checking 28 of 42 passwords (67%)")
	require.Contains(t, auditTexts(a.View), "Checking known password breaches…")
	require.Contains(t, auditTexts(a.View), "Password Audit", "the fixed audit header stays visible while progress updates")

	cancelled := false
	a.cancel = func() { cancelled = true }
	a.cancelButton.OnTapped()
	require.True(t, cancelled)
	require.True(t, a.cancelButton.Disabled())
	require.Contains(t, auditTexts(a.View), "Canceling audit…")
}

func TestPasswordAuditIgnoresStaleProgress(t *testing.T) {
	test.NewTempApp(t)
	a := NewPasswordAudit("Personal", nil, func(fyne.CanvasObject) {}, func(string) {}, func() {})
	a.running = true
	a.overview()
	a.runID = 2
	a.updateProgress(1, AuditProgress{Done: 1, Total: 2, Phase: AuditPhaseBreaches})
	require.Nil(t, a.progress)
	require.Contains(t, auditTexts(a.View), "Analyzing passwords locally…")
	a.Close()
	a.updateProgress(2, AuditProgress{Done: 1, Total: 2, Phase: AuditPhaseBreaches})
	require.Empty(t, a.View.Objects)
}

func TestPasswordAuditCancelsRunningAudit(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	auditor, _, checker := auditServiceFixture(t, "password")
	checker.started = make(chan struct{}, 1)
	a := NewPasswordAudit("Personal", auditor, window.SetContent, func(string) {}, func() {})
	window.SetContent(a.View)
	window.Resize(fyne.NewSize(440, 650))
	a.start()
	select {
	case <-checker.started:
	case <-time.After(time.Second):
		t.Fatal("audit did not reach the breach lookup")
	}
	test.Tap(a.cancelButton)
	require.Eventually(t, func() bool {
		var cancelled bool
		fyne.DoAndWait(func() {
			cancelled = !a.running && containsText(auditTexts(a.View), "Audit cancelled. Results include completed checks only.")
		})
		return cancelled
	}, time.Second, 10*time.Millisecond)
	require.True(t, a.result.Cancelled)
	require.True(t, a.result.LocalComplete)
	require.Len(t, a.result.Findings, 1)
	require.NotContains(t, auditTexts(a.View), "No issues found")
	require.Contains(t, auditTexts(a.View), "Audit cancelled. Results include completed checks only.")
	a.Close()
}

func containsText(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func TestDistinctAuditItemsCountsEachItemOnce(t *testing.T) {
	require.Equal(t, 2, distinctAuditItems([]AuditFinding{
		{AuditEntry: AuditEntry{ID: "same"}, Score: 0, Exposed: true},
		{AuditEntry: AuditEntry{ID: "same"}, Score: 1},
		{AuditEntry: AuditEntry{ID: "other"}, ReuseCount: 3},
	}))
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
	window.Show()
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

func TestAuditCategoryCountsAndActions(t *testing.T) {
	test.NewTempApp(t)
	a := NewPasswordAudit("Personal", nil, func(fyne.CanvasObject) {}, func(string) {}, func() {})
	a.hasResult = true
	a.result = AuditResult{Passwords: 1, ExposedChecked: 1, LocalComplete: true, Findings: []AuditFinding{
		{AuditEntry: AuditEntry{ID: "exposed"}, Score: 4, ReuseCount: 1, Exposed: true},
	}}
	a.overview()
	window := test.NewTempWindow(t, a.View)
	window.Resize(fyne.NewSize(440, 650))
	var rows []*ActionRow
	var visit func(fyne.CanvasObject)
	visit = func(object fyne.CanvasObject) {
		if row, ok := object.(*ActionRow); ok {
			rows = append(rows, row)
			return
		}
		switch value := object.(type) {
		case *fyne.Container:
			for _, child := range value.Objects {
				visit(child)
			}
		case *container.Scroll:
			visit(value.Content)
		}
	}
	visit(a.View)
	for _, row := range rows {
		expected := "0"
		interactive := false
		if row.title.Text == "Exposed passwords" {
			expected = "1"
			interactive = true
		}
		require.Equal(t, expected, row.count.Text)
		require.Equal(t, interactive, row.button.Visible())
		require.Equal(t, interactive, row.next.Visible())
		require.Equal(t, !interactive, row.button.OnTapped == nil)
		require.True(t, row.trailing.Visible())
		require.True(t, row.leading.Visible())
		require.True(t, row.title.Visible())
		require.True(t, row.detail.Visible())
	}
	require.Len(t, rows, 3)
	texts := auditTexts(a.View)
	require.Contains(t, texts, "Exposed passwords")
	require.Contains(t, texts, "Weak passwords")
	require.Contains(t, texts, "Reused passwords")
	for _, row := range rows {
		if row.title.Text == "Exposed passwords" {
			row.button.OnTapped()
			break
		}
	}
	require.Equal(t, 1, auditList(t, a.View).Length(), "selecting a category opens its matching findings")
}

func TestAuditFindingsOrderingAndExposureCount(t *testing.T) {
	test.NewTempApp(t)
	a := NewPasswordAudit("Personal", nil, func(fyne.CanvasObject) {}, func(string) {}, func() {})
	a.result = AuditResult{Findings: []AuditFinding{
		{AuditEntry: AuditEntry{ID: "weak", Title: "Weak", Subtitle: "b"}, Score: 1, Exposed: true, ExposedCount: 12},
		{AuditEntry: AuditEntry{ID: "exposed-high", Title: "Alpha"}, Score: 4, Exposed: true, ExposedCount: 12450},
		{AuditEntry: AuditEntry{ID: "very-weak", Title: "Zulu"}, Score: 0},
		{AuditEntry: AuditEntry{ID: "exposed-low", Title: "Beta"}, Score: 2, Exposed: true, ExposedCount: 3},
	}}
	require.Equal(t, []string{"exposed-high", "weak", "exposed-low"}, auditIDs(a.findings("exposed")))
	require.Equal(t, []string{"very-weak", "weak"}, auditIDs(a.findings("weak")))
	require.Equal(t, "12,450", formatAuditOccurrences(12450))
	a.hasResult = true
	a.showFindings("exposed")
	require.Contains(t, auditTexts(a.View), "Seen 12,450 times")
}

func TestWeakFindingsUseSharedStrengthIndicatorAndOpenItems(t *testing.T) {
	driver := test.NewApp()
	t.Cleanup(driver.Quit)
	window := driver.NewWindow("Audit")
	t.Cleanup(window.Close)
	opened := ""
	a := NewPasswordAudit("Personal", nil, window.SetContent, func(id string) { opened = id }, func() {})
	a.hasResult = true
	a.result.Findings = []AuditFinding{
		{AuditEntry: AuditEntry{ID: "weak", Title: "Weak", Subtitle: "very.long.username@example-company.test", Icon: icons.Login}, Score: 0},
		{AuditEntry: AuditEntry{ID: "very-weak", Title: "Very weak", Icon: icons.Login}, Score: 1},
	}
	a.showFindings("weak")
	window.SetContent(a.View)
	window.Resize(fyne.NewSize(440, 650))
	list := auditList(t, a.View)
	require.Equal(t, []string{"weak", "very-weak"}, auditIDs(a.findings("weak")))
	var indicator *PasswordStrengthIndicator
	for _, object := range test.LaidOutObjects(a.View) {
		if value, ok := object.(*PasswordStrengthIndicator); ok && value.Visible() {
			indicator = value
			break
		}
	}
	require.NotNil(t, indicator, "weak findings render the shared strength indicator")
	require.Equal(t, 0, indicator.score)
	var weakRow *ActionRow
	for _, object := range test.LaidOutObjects(a.View) {
		if row, ok := object.(*ActionRow); ok && row.detail.Text == "very.long.username@example-company.test" {
			weakRow = row
		}
	}
	require.NotNil(t, weakRow)
	require.Equal(t, fyne.TextTruncateEllipsis, weakRow.detail.Truncation, "long usernames truncate before the strength row or chevron")
	textBlock := weakRow.text.Objects[0].(*fyne.Container)
	detailBottom := textBlock.Position().Y + weakRow.detail.Position().Y + weakRow.detail.Size().Height
	indicatorTop := indicator.Position().Y
	require.GreaterOrEqual(t, indicatorTop, detailBottom+theme.Size(theme.SizeNameInnerPadding), "strength feedback has a visible gap below the username")
	rowBottom := weakRow.text.Size().Height
	indicatorBottom := indicatorTop + indicator.Size().Height
	require.LessOrEqual(t, indicatorBottom, rowBottom, "strength feedback stays within the entry row")
	list.Select(0)
	require.Equal(t, "weak", opened)
}

func TestPartialAuditCategoryRowsFitNarrowWindow(t *testing.T) {
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		t.Run(variantName(variant), func(t *testing.T) {
			driver := test.NewApp()
			t.Cleanup(driver.Quit)
			driver.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
			window := driver.NewWindow("Audit")
			t.Cleanup(window.Close)
			a := NewPasswordAudit("Personal", nil, window.SetContent, func(string) {}, func() {})
			a.hasResult = true
			a.checkedAt = time.Date(2026, time.October, 30, 14, 15, 0, 0, time.UTC)
			a.result = auditFixture()
			a.result.ExposedFailed = true
			a.result.ExposedChecked = 1
			a.overview()
			window.SetContent(a.View)
			window.Resize(fyne.NewSize(320, 540))
			window.Show()

			var rows []*ActionRow
			var dividers []*widget.Separator
			for _, object := range test.LaidOutObjects(a.View) {
				switch value := object.(type) {
				case *ActionRow:
					rows = append(rows, value)
				case *widget.Separator:
					dividers = append(dividers, value)
				}
			}
			require.Len(t, rows, 3)
			require.GreaterOrEqual(t, len(dividers), 3)
			for index, row := range rows {
				require.Equal(t, fyne.TextWrapWord, row.detail.Wrapping)
				rowBottom := driver.Driver().AbsolutePositionForObject(row).Y + row.Size().Height
				minimumTextHeight := row.title.MinSize().Height + row.detail.MinSize().Height
				require.GreaterOrEqual(t, row.Size().Height, minimumTextHeight, "category subtitle must fit inside its row")
				var divider *widget.Separator
				for _, candidate := range dividers {
					candidateTop := driver.Driver().AbsolutePositionForObject(candidate).Y
					if candidateTop >= rowBottom && (divider == nil || candidateTop < driver.Driver().AbsolutePositionForObject(divider).Y) {
						divider = candidate
					}
				}
				if index == len(rows)-1 {
					continue // The last category has no trailing separator.
				}
				require.NotNil(t, divider, "each preceding category has a following divider")
				dividerTop := driver.Driver().AbsolutePositionForObject(divider).Y
				require.LessOrEqual(t, rowBottom, dividerTop, "the separator must follow the complete row")
				for dividerIndex, candidate := range dividers {
					if candidate == divider {
						dividers = append(dividers[:dividerIndex], dividers[dividerIndex+1:]...)
						break
					}
				}
			}
			require.Contains(t, auditTexts(a.View), "Last checked: "+a.checkedAt.Local().Format("2 Jan 2006, 15:04"))
		})
	}
}

func auditIDs(rows []AuditFinding) []string {
	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	return ids
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
	require.Equal(t, 6, groups.Length(), "group headings and all member entries share one list")
	require.Contains(t, auditTexts(a.View), "4 items · 2 groups · Personal vault")
	groups.Select(0)
	require.Empty(t, opened, "group headings are not navigable")
	groups.Select(4)
	a.DetailBack()
	require.Same(t, groups, auditList(t, a.View))
	groups.Select(5)
	a.DetailBack()
	require.Equal(t, []string{"github", "forum"}, opened)
	groups.Select(1)
	a.DetailBack()
	groups.Select(2)
	require.Equal(t, []string{"github", "forum", "fourth", "third"}, opened)
}
