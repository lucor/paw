// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/ui/assets/icons"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// PasswordAudit owns transient audit views for one unlocked vault.
type PasswordAudit struct {
	View                              *fyne.Container
	name                              string
	auditor                           *PasswordAuditor
	show                              func(fyne.CanvasObject)
	open                              func(string)
	back                              func()
	cancel                            context.CancelFunc
	closed, running, hasResult, stale bool
	canceling                         bool
	result                            AuditResult
	checkedAt                         time.Time
	progress                          *widget.ProgressBar
	progressInfinite                  *widget.ProgressBarInfinite
	progressSlot                      *fyne.Container
	progressSummary                   *widget.Label
	progressSummarySlot               *fyne.Container
	progressPhase                     *widget.Label
	progressPhaseSlot                 *fyne.Container
	cancelButton                      *widget.Button
	progressPhaseValue                AuditProgressPhase
	runID                             uint64
	banner                            *widget.Label
	returnToFindings                  func()
}

// NewPasswordAudit creates the overview without starting any remote checks.
func NewPasswordAudit(name string, auditor *PasswordAuditor, show func(fyne.CanvasObject), open func(string), back func()) *PasswordAudit {
	a := &PasswordAudit{View: container.NewStack(), name: name, auditor: auditor, show: show, open: open, back: back}
	a.overview()
	return a
}

func auditHeader(title string, back func()) fyne.CanvasObject {
	button := widget.NewButtonWithIcon("", icons.Back, back)
	button.Importance = widget.LowImportance
	return newPageHeader(button, newText(title, titleStyle), nil)
}

func auditPage(title string, back func(), content fyne.CanvasObject) fyne.CanvasObject {
	return newSecondaryPage(auditHeader(title, back), nil, content)
}

func (a *PasswordAudit) staleBanner() *widget.Label {
	a.banner = newText("Items changed. Audit results may be outdated. Run audit again.", captionStyle)
	if !a.stale {
		a.banner.Hide()
	}
	return a.banner
}

func (a *PasswordAudit) overview() {
	var content fyne.CanvasObject
	switch {
	case a.running:
		content = a.runningOverview()
	case !a.hasResult:
		content = a.initialOverview()
	default:
		content = a.resultOverview()
	}
	a.View.Objects = []fyne.CanvasObject{auditPage("Password Audit", func() { a.Close(); a.back() }, content)}
	a.View.Refresh()
}

func (a *PasswordAudit) initialOverview() fyne.CanvasObject {
	button := widget.NewButton("Run audit", a.start)
	button.Importance = widget.HighImportance
	icon := widget.NewIcon(icons.Audit)
	iconBox := container.NewGridWrap(fyne.NewSize(theme.Size(theme.SizeNameHeadingText)*2, theme.Size(theme.SizeNameHeadingText)*2), icon)
	return container.NewVScroll(container.New(layout.NewCustomPaddedVBoxLayout(theme.Size(theme.SizeNamePadding)*2),
		newText(a.name, headingStyle), a.staleBanner(), layout.NewSpacer(),
		container.NewCenter(iconBox), centeredAuditText("Check your passwords", headingStyle), centeredAuditText("Find exposed, weak and reused passwords in this vault.", bodyStyle),
		container.NewCenter(button), layout.NewSpacer(),
	))
}

func (a *PasswordAudit) runningOverview() fyne.CanvasObject {
	a.progressInfinite = widget.NewProgressBarInfinite()
	a.progress = nil
	a.progressSlot = container.NewStack(a.progressInfinite)
	a.progressSummary = newText("Analyzing passwords locally…", bodyStyle)
	a.progressPhase = newText("This may take a moment.", captionStyle)
	a.progressSummarySlot = container.NewStack(a.progressSummary)
	a.progressPhaseSlot = container.NewStack(a.progressPhase)
	a.cancelButton = widget.NewButton("Cancel", a.cancelAudit)
	iconBox := container.NewGridWrap(fyne.NewSize(theme.Size(theme.SizeNameHeadingText)*2, theme.Size(theme.SizeNameHeadingText)*2), widget.NewIcon(icons.Audit))
	return container.New(layout.NewCustomPaddedVBoxLayout(theme.Size(theme.SizeNamePadding)*2),
		newText(a.name, headingStyle), a.staleBanner(),
		container.NewCenter(iconBox), centeredAuditText("Checking passwords…", headingStyle),
		centeredAuditText(a.name+" vault", captionStyle),
		a.progressSummarySlot, a.progressSlot, a.progressPhaseSlot,
		container.NewCenter(a.cancelButton),
	)
}

func (a *PasswordAudit) resultOverview() fyne.CanvasObject {
	r := a.result
	complete := auditChecksComplete(r)
	if complete && r.Passwords == 0 {
		content := container.NewVBox(newText(a.name, headingStyle), a.staleBanner(),
			widget.NewSeparator(), centeredAuditText("No passwords to audit", headingStyle),
			centeredAuditText("There are no passwords in this vault to check.", bodyStyle),
		)
		return container.NewVScroll(content)
	}

	runAgain := widget.NewButton("Run again", a.start)
	runAgain.Importance = widget.MediumImportance
	vaultRow := container.NewBorder(nil, nil, nil, runAgain, auditVaultName(a.name))
	content := container.NewVBox(vaultRow, a.staleBanner())
	if !a.checkedAt.IsZero() {
		content.Add(newText("Last checked: "+a.checkedAt.Local().Format("2 Jan 2006, 15:04"), captionStyle))
	}

	if !complete {
		content.Add(widget.NewSeparator())
		warning := newText("Audit partially completed", bodyStyle)
		warning.Importance = widget.WarningImportance
		content.Add(warning)
		if r.Cancelled {
			content.Add(newText("Audit cancelled. Results include completed checks only.", captionStyle))
		} else {
			if !r.LocalComplete || r.LoadErrors > 0 {
				content.Add(newText("Some items could not be read. Local results may be incomplete.", captionStyle))
			}
			if r.ExposedFailed || r.Cancelled || r.ExposedChecked < r.Passwords {
				content.Add(newText("Breach checks could not be completed.", captionStyle))
			}
		}
	}

	if complete && len(r.Findings) == 0 {
		content.Add(widget.NewSeparator())
		content.Add(container.NewCenter(widget.NewIcon(icons.Audit)))
		content.Add(centeredAuditText("No issues found", headingStyle))
		content.Add(centeredAuditText(fmt.Sprintf("%s analyzed.\nNo exposed, weak or reused passwords were found.", passwordCount(r.Passwords)), bodyStyle))
		return container.NewVScroll(content)
	}

	attention := distinctAuditItems(r.Findings)
	if attention > 0 {
		attentionLabel := fmt.Sprintf("%d items need attention", attention)
		if attention == 1 {
			attentionLabel = "1 item needs attention"
		}
		content.Add(newText(attentionLabel, headingStyle))
	} else if complete {
		content.Add(newText("0 items need attention", headingStyle))
	} else {
		content.Add(newText("No confirmed findings in completed checks.", bodyStyle))
	}
	content.Add(newText(fmt.Sprintf("%s analyzed locally", passwordCount(r.Passwords)), bodyStyle))
	if !auditBreachChecksComplete(r) {
		content.Add(newText(fmt.Sprintf("%d of %d passwords checked for breaches", r.ExposedChecked, r.Passwords), captionStyle))
	}
	if attention > 0 {
		content.Add(newText("An item may have multiple findings.", captionStyle))
	}
	content.Add(widget.NewSeparator())
	for index, category := range auditCategories {
		count, description, actionable := a.categorySummary(category.id)
		row := NewActionRow(category.title, description, category.icon, nil)
		row.SetFlat(true)
		row.SetDetailStyle(captionStyle)
		row.UseDefaultTextTheme()
		row.SetTrailingText(count)
		if actionable {
			id := category.id
			row.SetTapped(func() { a.showFindings(id) })
		} else {
			row.SetInteractive(false)
		}
		content.Add(row)
		if index < len(auditCategories)-1 {
			content.Add(widget.NewSeparator())
		}
	}
	return container.NewVScroll(content)
}

func centeredAuditText(text string, style textStyle) *widget.Label {
	label := newText(text, style)
	label.Alignment = fyne.TextAlignCenter
	return label
}

func auditVaultName(name string) *widget.Label {
	label := newText(name, bodyStyle)
	label.Wrapping = fyne.TextWrapOff
	label.Truncation = fyne.TextTruncateEllipsis
	return label
}

func passwordCount(count int) string {
	if count == 1 {
		return "1 password"
	}
	return fmt.Sprintf("%d passwords", count)
}

func distinctAuditItems(findings []AuditFinding) int {
	seen := make(map[string]struct{}, len(findings))
	for _, finding := range findings {
		seen[finding.ID] = struct{}{}
	}
	return len(seen)
}

func auditBreachChecksComplete(result AuditResult) bool {
	return result.LocalComplete && result.LoadErrors == 0 && auditBreachLookupsComplete(result)
}

func auditBreachLookupsComplete(result AuditResult) bool {
	return !result.ExposedFailed && !result.Cancelled && result.ExposedChecked >= result.Passwords
}

func auditChecksComplete(result AuditResult) bool {
	return result.LocalComplete && result.LoadErrors == 0 && !result.ExposedFailed && !result.Cancelled && result.ExposedChecked >= result.Passwords
}

func (a *PasswordAudit) categorySummary(category string) (string, string, bool) {
	count := distinctAuditItems(a.findings(category))
	complete := a.result.LocalComplete
	description := ""
	switch category {
	case "exposed":
		complete = auditBreachChecksComplete(a.result)
		description = auditCategories[0].description
		if !complete {
			description = "Not checked"
			if a.result.ExposedChecked > 0 {
				description = "No findings in checked passwords"
				if count > 0 {
					description = "Found in completed checks"
				}
			}
		}
	case "weak":
		description = "Identified by zxcvbn"
	case "reused":
		description = "Shared between items"
	}
	if !complete && category == "exposed" && count == 0 {
		return "—", description, false
	}
	if !complete && count == 0 {
		return "—", "Not fully checked", false
	}
	return fmt.Sprint(count), description, count > 0
}

var auditCategories = []struct {
	id, title, description string
	icon                   fyne.Resource
}{
	{"exposed", "Exposed passwords", "Found in known data breaches", icons.Audit},
	{"weak", "Weak passwords", "Identified by zxcvbn", icons.Password},
	{"reused", "Reused passwords", "Same password in multiple items", icons.Password},
}

func (a *PasswordAudit) start() {
	if a.closed || a.running || a.auditor == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.running = true
	a.canceling = false
	a.progressPhaseValue = 0
	a.stale = false
	a.runID++
	runID := a.runID
	a.overview()
	go func() {
		progress := make(chan AuditProgress)
		results := make(chan AuditResult, 1)
		go func() { results <- a.auditor.Run(ctx, progress) }()
		for update := range progress {
			update := update
			fyne.Do(func() { a.updateProgress(runID, update) })
		}
		result := <-results
		cancel()
		fyne.Do(func() {
			if a.closed || runID != a.runID {
				return
			}
			a.running = false
			a.canceling = false
			a.cancel = nil
			a.hasResult = true
			a.result = result
			a.checkedAt = time.Now()
			a.overview()
		})
	}()
}

func (a *PasswordAudit) updateProgress(runID uint64, update AuditProgress) {
	if a.closed || !a.running || a.canceling || runID != a.runID || update.Total <= 0 {
		return
	}
	if update.Phase == AuditPhaseBreaches && a.progressPhaseValue != AuditPhaseBreaches {
		a.progressPhaseValue = AuditPhaseBreaches
		a.progress = widget.NewProgressBar()
		a.progress.TextFormatter = func() string { return "" }
		a.progressInfinite = nil
		a.progressSlot.Objects = []fyne.CanvasObject{a.progress}
		a.progressSlot.Refresh()
		replaceAuditLabel(a.progressPhaseSlot, &a.progressPhase, "Checking known password breaches…", captionStyle)
	}
	if update.Phase != AuditPhaseBreaches || a.progress == nil {
		return
	}
	done := min(update.Done, update.Total)
	percentage := (done*100 + update.Total/2) / update.Total
	replaceAuditLabel(a.progressSummarySlot, &a.progressSummary, fmt.Sprintf("Checking %d of %d passwords (%d%%)", done, update.Total, percentage), bodyStyle)
	a.progress.SetValue(float64(done) / float64(update.Total))
}

func replaceAuditLabel(slot *fyne.Container, current **widget.Label, text string, style textStyle) {
	*current = newText(text, style)
	slot.Objects = []fyne.CanvasObject{*current}
	slot.Refresh()
}

func (a *PasswordAudit) cancelAudit() {
	if a.cancel == nil || a.canceling {
		return
	}
	a.canceling = true
	if a.cancelButton != nil {
		a.cancelButton.Disable()
	}
	if a.progressPhase != nil {
		replaceAuditLabel(a.progressPhaseSlot, &a.progressPhase, "Canceling audit…", captionStyle)
	}
	a.cancel()
}

func (a *PasswordAudit) findings(category string) []AuditFinding {
	var rows []AuditFinding
	for _, finding := range a.result.Findings {
		if category == "exposed" && finding.Exposed || category == "weak" && finding.Score <= 1 || category == "reused" && finding.ReuseCount > 1 {
			rows = append(rows, finding)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		left, right := rows[i], rows[j]
		switch category {
		case "exposed":
			if left.ExposedCount != right.ExposedCount {
				return left.ExposedCount > right.ExposedCount
			}
		case "weak":
			if left.Score != right.Score {
				return left.Score < right.Score
			}
		}
		return auditFindingLess(left, right)
	})
	return rows
}

func auditFindingLess(left, right AuditFinding) bool {
	if leftTitle, rightTitle := strings.ToLower(left.Title), strings.ToLower(right.Title); leftTitle != rightTitle {
		return leftTitle < rightTitle
	}
	if leftSubtitle, rightSubtitle := strings.ToLower(left.Subtitle), strings.ToLower(right.Subtitle); leftSubtitle != rightSubtitle {
		return leftSubtitle < rightSubtitle
	}
	return left.ID < right.ID
}

func formatAuditOccurrences(count int) string {
	return message.NewPrinter(language.English).Sprintf("%d", count)
}

func (a *PasswordAudit) showFindings(category string) {
	rows := a.findings(category)
	if category == "reused" {
		a.showReusedFindings(rows)
		return
	}
	a.showFindingRows(category, rows, func() { a.returnToFindings = nil; a.overview() })
}

func (a *PasswordAudit) showFindingRows(category string, rows []AuditFinding, back func()) {
	if a.closed {
		return
	}
	title := ""
	for _, value := range auditCategories {
		if value.id == category {
			title = value.title
		}
	}
	var list *widget.List
	list = widget.NewList(func() int { return len(rows) }, func() fyne.CanvasObject {
		row := NewActionRow("Item", "Summary", icons.Password, nil)
		row.SetFlat(true)
		indicator := NewPasswordStrengthIndicator()
		indicator.SetCompact()
		row.SetSupplemental(indicator)
		if category == "weak" {
			row.UseDefaultTextTheme()
		}
		return container.New(layout.NewCustomPaddedLayout(0, 0, theme.Size(theme.SizeNameInnerPadding), theme.Size(theme.SizeNameInnerPadding)), row)
	}, func(id widget.ListItemID, object fyne.CanvasObject) {
		finding := rows[id]
		status := ""
		if category == "exposed" {
			status = "Seen " + formatAuditOccurrences(finding.ExposedCount) + " times"
		}
		if category == "weak" {
			status = "Weak"
			if finding.Score == 0 {
				status = "Very weak"
			}
		}
		detail := status
		if finding.Subtitle != "" {
			detail = finding.Subtitle
			if status != "" && category != "weak" {
				detail += "\n" + status
			}
		}
		row := object.(*fyne.Container).Objects[0].(*ActionRow)
		row.SetContent(finding.Title, detail, finding.Icon)
		if indicator, ok := row.supplemental.(*PasswordStrengthIndicator); ok {
			if category == "weak" {
				indicator.SetScore(finding.Score)
			} else {
				indicator.Clear()
			}
		}
		row.SetTapped(func() { list.Select(id) })
		list.SetItemHeight(id, object.MinSize().Height)
	})
	list.HideSeparators = false
	header := container.NewVBox(newText(itemCountLabel(len(rows))+" · "+a.name+" vault", bodyStyle), a.staleBanner())
	banner := a.banner
	body := fyne.CanvasObject(list)
	if len(rows) == 0 {
		body = container.NewCenter(newText("No findings in this category", bodyStyle))
	}
	page := auditPage(title, back, container.NewBorder(header, nil, nil, nil, body))
	a.View.Objects = []fyne.CanvasObject{page}
	a.View.Refresh()
	var restore func()
	restore = func() {
		if a.closed {
			return
		}
		a.banner = banner
		if a.stale {
			banner.Show()
		}
		a.returnToFindings = restore
		a.View.Objects = []fyne.CanvasObject{page}
		a.View.Refresh()
		a.show(a.View)
	}
	a.returnToFindings = restore
	list.OnSelected = func(id widget.ListItemID) {
		list.UnselectAll()
		if a.closed {
			return
		}
		a.open(rows[id].ID)
	}
}

type auditReuseGroup struct {
	members []AuditFinding
}

type auditReuseListRow struct {
	groupIndex int
	members    []AuditFinding
	finding    AuditFinding
	isGroup    bool
}

func sortedAuditReuseGroups(rows []AuditFinding) []auditReuseGroup {
	byID := make(map[int][]AuditFinding)
	for _, row := range rows {
		byID[row.ReuseGroup] = append(byID[row.ReuseGroup], row)
	}
	groups := make([]auditReuseGroup, 0, len(byID))
	for _, members := range byID {
		sort.SliceStable(members, func(i, j int) bool { return auditFindingLess(members[i], members[j]) })
		groups = append(groups, auditReuseGroup{members: members})
	}
	sort.SliceStable(groups, func(i, j int) bool {
		return auditFindingLess(groups[i].members[0], groups[j].members[0])
	})
	return groups
}

func (a *PasswordAudit) showReusedFindings(rows []AuditFinding) {
	if a.closed {
		return
	}
	groups := sortedAuditReuseGroups(rows)
	listRows := make([]auditReuseListRow, 0, len(rows)+len(groups))
	for index, group := range groups {
		listRows = append(listRows, auditReuseListRow{groupIndex: index + 1, members: group.members, isGroup: true})
		for _, finding := range group.members {
			listRows = append(listRows, auditReuseListRow{finding: finding})
		}
	}
	var list *widget.List
	list = widget.NewList(func() int { return len(listRows) }, func() fyne.CanvasObject {
		row := NewActionRow("Item", "Summary", icons.Password, nil)
		row.SetFlat(true)
		return container.New(layout.NewCustomPaddedLayout(0, 0, theme.Size(theme.SizeNameInnerPadding), theme.Size(theme.SizeNameInnerPadding)), row)
	}, func(id widget.ListItemID, object fyne.CanvasObject) {
		item := listRows[id]
		row := object.(*fyne.Container).Objects[0].(*ActionRow)
		if item.isGroup {
			row.SetContent(fmt.Sprintf("Group %d", item.groupIndex), itemCountLabel(len(item.members)), icons.Password)
			row.SetInteractive(false)
		} else {
			row.SetInteractive(true)
			row.SetContent(item.finding.Title, item.finding.Subtitle, item.finding.Icon)
			row.SetTapped(func() { list.Select(id) })
		}
		list.SetItemHeight(id, row.MinSize().Height)
	})
	list.HideSeparators = false
	groupCount := fmt.Sprintf("%d groups", len(groups))
	if len(groups) == 1 {
		groupCount = "1 group"
	}
	header := container.NewVBox(newText(fmt.Sprintf("%s · %s · %s vault", itemCountLabel(len(rows)), groupCount, a.name), bodyStyle), a.staleBanner())
	banner := a.banner
	page := auditPage("Reused passwords", func() { a.returnToFindings = nil; a.overview() }, container.NewBorder(header, nil, nil, nil, list))
	a.View.Objects = []fyne.CanvasObject{page}
	a.View.Refresh()
	var restore func()
	restore = func() {
		if a.closed {
			return
		}
		a.banner = banner
		if a.stale {
			banner.Show()
		}
		a.returnToFindings = restore
		a.View.Objects = []fyne.CanvasObject{page}
		a.View.Refresh()
		a.show(a.View)
	}
	a.returnToFindings = restore
	list.OnSelected = func(id widget.ListItemID) {
		list.UnselectAll()
		if a.closed || listRows[id].isGroup {
			return
		}
		a.open(listRows[id].finding.ID)
	}
}

// DetailBack returns the existing findings view with its list position intact.
func (a *PasswordAudit) DetailBack() {
	if !a.closed && a.returnToFindings != nil {
		a.returnToFindings()
	}
}

// BackHeader lets the existing item detail and editor return to findings.
func (a *PasswordAudit) BackHeader() fyne.CanvasObject { return auditHeader("", a.DetailBack) }

// MarkStale warns after an item changes while keeping its findings view intact.
func (a *PasswordAudit) MarkStale() {
	if a.closed {
		return
	}
	a.stale = true
	if a.banner != nil {
		a.banner.Show()
	}
}

// Close cancels work and removes result metadata from the session's views.
func (a *PasswordAudit) Close() {
	a.closed = true
	a.runID++
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
	clear(a.result.Findings)
	a.result = AuditResult{}
	a.checkedAt = time.Time{}
	a.progress = nil
	a.progressInfinite = nil
	a.progressSlot = nil
	a.progressSummary = nil
	a.progressSummarySlot = nil
	a.progressPhase = nil
	a.progressPhaseSlot = nil
	a.cancelButton = nil
	a.returnToFindings = nil
	a.View.Objects = nil
	a.View.Refresh()
}
