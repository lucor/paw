// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package v2

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/ui/v2/assets/icons"
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
	result                            AuditResult
	progress                          *widget.ProgressBar
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
	return container.NewBorder(nil, nil, button, nil, newText(title, viewTitleStyle))
}

func auditPage(title string, back func(), content fyne.CanvasObject) fyne.CanvasObject {
	return container.New(layout.NewCustomPaddedLayout(24, 24, 24, 24), container.NewBorder(auditHeader(title, back), nil, nil, nil, content))
}

func (a *PasswordAudit) staleBanner() *widget.Label {
	a.banner = newText("Items changed. Audit results may be outdated. Run audit again.", helpStyle)
	if !a.stale {
		a.banner.Hide()
	}
	return a.banner
}

func (a *PasswordAudit) overview() {
	content := container.NewVBox(newText(a.name, headingStyle), newText("Password security analysis", subtitleStyle), a.staleBanner())
	if a.running {
		a.progress = widget.NewProgressBar()
		a.progress.TextFormatter = func() string { return "Checking passwords…" }
		cancel := widget.NewButton("Cancel", func() {
			if a.cancel != nil {
				a.cancel()
			}
		})
		content.Add(a.progress)
		content.Add(cancel)
	} else {
		if a.hasResult {
			r := a.result
			content.Add(newText(fmt.Sprintf("Passwords analyzed: %d", r.Passwords), helpStyle))
			switch {
			case r.Cancelled:
				content.Add(newText("Audit cancelled. Some checks were not completed.", bodyStyle))
			case r.LoadErrors > 0:
				content.Add(newText("Some items could not be read. Results are incomplete.", errorStyle))
			case r.LocalComplete && r.Passwords == 0:
				content.Add(newText("No passwords to audit", bodyStyle))
			case r.ExposedFailed || r.ExposedChecked < r.Passwords:
				content.Add(newText("Some checks were not completed.", bodyStyle))
			}
			if len(r.Findings) == 0 && r.LocalComplete && r.Passwords > 0 && !r.Cancelled {
				content.Add(newText("No issues found in completed checks", bodyStyle))
			} else if len(r.Findings) > 0 {
				content.Add(newText(fmt.Sprint(len(r.Findings)), headingStyle))
				label := "items need attention"
				if len(r.Findings) == 1 {
					label = "item needs attention"
				}
				content.Add(newText(label, bodyStyle))
				content.Add(newText("One item may have multiple findings.", helpStyle))
			}
			for _, category := range auditCategories {
				count := len(a.findings(category.id))
				description := category.description
				if category.id == "exposed" && (r.ExposedFailed || r.ExposedChecked < r.Passwords || r.Cancelled) {
					description = "Not checked — breach lookup incomplete"
					if r.ExposedFailed {
						description = "Breach lookup unavailable. Try again when connected."
					}
				} else if category.id != "exposed" && !r.LocalComplete {
					description += " · Check incomplete"
				}
				id := category.id
				title := fmt.Sprintf("%s · %d", category.title, count)
				if count == 0 {
					row := NewActionRow(title, description, category.icon, nil)
					row.SetInteractive(false)
					content.Add(container.NewPadded(row))
				} else {
					content.Add(container.NewPadded(NewActionRow(title, description, category.icon, func() { a.showFindings(id) })))
				}
			}
		}
		label := "Run audit"
		if a.hasResult {
			label = "Run audit again"
		}
		button := widget.NewButton(label, a.start)
		button.Importance = widget.HighImportance
		content.Add(button)
	}
	a.View.Objects = []fyne.CanvasObject{auditPage("Password Audit", func() { a.Close(); a.back() }, container.NewVScroll(content))}
	a.View.Refresh()
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
	if a.closed || a.running {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.running = true
	a.stale = false
	a.overview()
	go func() {
		progress := make(chan AuditProgress)
		results := make(chan AuditResult, 1)
		go func() { results <- a.auditor.Run(ctx, progress) }()
		for update := range progress {
			fyne.Do(func() {
				if !a.closed && a.running && update.Total > 0 {
					a.progress.SetValue(float64(update.Done) / float64(update.Total))
				}
			})
		}
		result := <-results
		cancel()
		fyne.Do(func() {
			if a.closed {
				return
			}
			a.running = false
			a.hasResult = true
			a.result = result
			a.overview()
		})
	}()
}

func (a *PasswordAudit) findings(category string) []AuditFinding {
	var rows []AuditFinding
	for _, finding := range a.result.Findings {
		if category == "exposed" && finding.Exposed || category == "weak" && finding.Score <= 1 || category == "reused" && finding.ReuseCount > 1 {
			rows = append(rows, finding)
		}
	}
	return rows
}

func (a *PasswordAudit) showFindings(category string) {
	rows := a.findings(category)
	if category == "reused" {
		seen := map[int]bool{}
		var groups []AuditFinding
		for _, row := range rows {
			if !seen[row.ReuseGroup] {
				seen[row.ReuseGroup] = true
				groups = append(groups, row)
			}
		}
		rows = groups
	}
	a.showFindingRows(category, rows, category == "reused", func() { a.returnToFindings = nil; a.overview() })
}

func (a *PasswordAudit) showFindingRows(category string, rows []AuditFinding, grouped bool, back func()) {
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
		return container.New(layout.NewCustomPaddedLayout(0, 0, theme.Size(theme.SizeNameInnerPadding), theme.Size(theme.SizeNameInnerPadding)), NewActionRow("Item", "Summary\nFinding", icons.Password, nil))
	}, func(id widget.ListItemID, object fyne.CanvasObject) {
		finding := rows[id]
		status := "Exposed"
		if category == "weak" {
			status = "Weak"
			if finding.Score == 0 {
				status = "Very weak"
			}
		}
		if category == "reused" {
			status = fmt.Sprintf("Used in %d items", finding.ReuseCount)
		}
		detail := status
		if finding.Subtitle != "" {
			detail = finding.Subtitle + "\n" + status
		}
		row := object.(*fyne.Container).Objects[0].(*ActionRow)
		if grouped {
			row.SetContent(status, "", icons.Password)
		} else {
			row.SetContent(finding.Title, detail, finding.Icon)
		}
		row.SetTapped(func() { list.Select(id) })
		list.SetItemHeight(id, row.MinSize().Height)
	})
	list.HideSeparators = true
	count := itemCountLabel(len(rows))
	if grouped {
		count = fmt.Sprintf("%d password groups", len(rows))
		if len(rows) == 1 {
			count = "1 password group"
		}
	}
	header := container.NewVBox(newText(count+" · "+a.name+" vault", subtitleStyle), a.staleBanner())
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
		if grouped {
			var members []AuditFinding
			for _, finding := range a.findings("reused") {
				if finding.ReuseGroup == rows[id].ReuseGroup {
					members = append(members, finding)
				}
			}
			a.showFindingRows(category, members, false, restore)
			return
		}
		a.open(rows[id].ID)
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
	if a.cancel != nil {
		a.cancel()
	}
	clear(a.result.Findings)
	a.result = AuditResult{}
	a.returnToFindings = nil
	a.View.Objects = nil
	a.View.Refresh()
}
