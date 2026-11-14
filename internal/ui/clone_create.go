// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"errors"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	pawsync "go.lucor.dev/paw/internal/sync"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

type cloneCreateView struct {
	view     fyne.CanvasObject
	back     *widget.Button
	name     *widget.Entry
	password *PasswordField
	confirm  *widget.Entry
	create   *widget.Button
	error    *widget.Label
	scroll   *container.Scroll
}

func newCloneCreateView(deviceName string, info pawsync.VaultReference, back func(), create func(string, string) error) *cloneCreateView {
	view := &cloneCreateView{
		back:     widget.NewButtonWithIcon("", icons.Back, back),
		name:     widget.NewEntry(),
		password: NewPasswordField(),
		confirm:  widget.NewPasswordEntry(),
		error:    newErrorText(""),
	}
	view.back.Importance = widget.LowImportance
	header := newPageHeader(view.back, newText("Review vault", titleStyle), nil)
	view.name.SetText(info.Name)
	view.password.Entry.Validator = func(value string) error {
		if value == "" {
			return errors.New("master password is required")
		}
		return nil
	}
	view.confirm.Validator = func(value string) error {
		if value == "" {
			return errors.New("password confirmation is required")
		}
		if value != view.password.Entry.Text {
			return errors.New("passwords do not match")
		}
		return nil
	}
	updateSubmit := func() {
		if strings.TrimSpace(view.name.Text) == "" || view.password.Entry.Text == "" || view.confirm.Text == "" || view.confirm.Text != view.password.Entry.Text {
			view.create.Disable()
			return
		}
		view.create.Enable()
	}
	showError := func(err error) {
		if err == nil {
			view.error.Hide()
			return
		}
		view.error.SetText(err.Error())
		view.error.Show()
	}
	view.create = widget.NewButton("Clone vault", func() {
		if strings.TrimSpace(view.name.Text) == "" {
			return
		}
		if err := view.password.Entry.Validate(); err != nil {
			return
		}
		if err := view.confirm.Validate(); err != nil {
			return
		}
		if create == nil {
			return
		}
		view.SetBusy(true)
		if err := create(strings.TrimSpace(view.name.Text), view.password.Entry.Text); err != nil {
			showError(err)
			view.SetBusy(false)
		}
	})
	view.create.Importance = widget.HighImportance
	view.create.Disable()
	var form *widget.Form
	passwordWasEmpty := true
	view.password.OnChanged = func(string) {
		passwordIsEmpty := view.password.Entry.Text == ""
		if passwordIsEmpty != passwordWasEmpty {
			form.Refresh()
			passwordWasEmpty = passwordIsEmpty
		}
		if view.confirm.Text != "" {
			view.confirm.Validate()
		}
		updateSubmit()
	}
	view.name.OnChanged = func(string) { updateSubmit() }
	view.confirm.OnChanged = func(string) { updateSubmit() }
	form = widget.NewForm(
		widget.NewFormItem("Vault name", view.name),
		widget.NewFormItem("Master password", view.password),
		widget.NewFormItem("Confirm password", view.confirm),
	)
	form.Orientation = widget.Vertical
	deviceName = strings.TrimSuffix(deviceName, ".localdomain")
	deviceSummary := newText("Clone from "+deviceName, bodyStyle)
	deviceSummary.Wrapping = fyne.TextWrapBreak
	itemLabel := "items"
	if info.Items == 1 {
		itemLabel = "item"
	}
	attachmentLabel := "attachments"
	if info.Attachments == 1 {
		attachmentLabel = "attachment"
	}
	counts := newText(fmt.Sprintf("%d %s · %d %s", info.Items, itemLabel, info.Attachments, attachmentLabel), captionStyle)
	content := container.New(layout.NewCustomPaddedVBoxLayout(pawSize(pawSizeNameFormSpacing)),
		deviceSummary,
		counts,
		form,
	)
	footer := container.NewVBox(view.error, container.NewBorder(widget.NewSeparator(), nil, nil, view.create, nil))
	view.scroll = container.NewVScroll(content)
	view.scroll.Direction = container.ScrollVerticalOnly
	view.view = newSecondaryPage(header, footer, view.scroll)
	return view
}

func (v *cloneCreateView) SetBusy(busy bool) {
	if busy {
		v.create.SetText("Creating vault…")
		v.create.Disable()
		v.back.Disable()
		v.name.Disable()
		v.password.Entry.Disable()
		v.confirm.Disable()
		return
	}
	v.create.SetText("Clone vault")
	v.back.Enable()
	v.name.Enable()
	v.password.Entry.Enable()
	v.confirm.Enable()
	if strings.TrimSpace(v.name.Text) != "" && v.password.Entry.Text != "" && v.confirm.Text == v.password.Entry.Text {
		v.create.Enable()
	} else {
		v.create.Disable()
	}
}

func (v *cloneCreateView) SetError(err error) {
	if err == nil {
		v.error.Hide()
		return
	}
	v.error.SetText(err.Error())
	v.error.Show()
}
