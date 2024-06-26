// Copyright 2021 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ui

import (
	"context"
	"crypto/sha1"
	"fmt"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lucor.dev/paw/internal/otp"
	"lucor.dev/paw/internal/paw"
)

type TOTP struct {
	*paw.TOTP
}

func (t *TOTP) Edit(ctx context.Context, w fyne.Window) (fyne.CanvasObject, *paw.TOTP) {
	totp := &paw.TOTP{}
	*totp = *t.TOTP

	if totp == nil || (*totp == paw.TOTP{}) {
		totp = paw.NewDefaultTOTP()
	}

	keyBind := binding.BindString(&totp.Secret)
	keyEntry := widget.NewPasswordEntry()
	keyEntry.Bind(keyBind)
	keyEntry.Validator = func(string) error {
		_, err := otp.TOTPFromBase32(totp.Hasher(), totp.Secret, time.Now().UTC(), totp.Interval, totp.Digits)
		return err
	}

	settingsButton := widget.NewButtonWithIcon("", theme.SettingsIcon(), func() {
		copy := totp
		form := container.New(layout.NewFormLayout())

		hashOptions := []string{string(paw.SHA1), string(paw.SHA256), string(paw.SHA512)}
		hashSelect := widget.NewSelect(hashOptions, func(s string) {
			copy.Hash = paw.TOTPHash(s)
		})
		hashSelect.Selected = string(copy.Hash)
		form.Add(labelWithStyle("Hash Algorithm"))
		form.Add(hashSelect)

		digitsOptions := []string{"5", "6", "7", "8", "9", "10"}
		digitsSelect := widget.NewSelect(digitsOptions, func(s string) {
			copy.Digits, _ = strconv.Atoi(s)
		})
		digitsSelect.Selected = strconv.Itoa(copy.Digits)
		form.Add(labelWithStyle("Digits"))
		form.Add(digitsSelect)

		intervalBind := binding.BindInt(&copy.Interval)
		intervalSlider := widget.NewSlider(5, 60)
		intervalSlider.Step = 5
		intervalSlider.OnChanged = func(f float64) {
			intervalBind.Set(int(f))
		}
		intervalSlider.Value = float64(copy.Interval)
		intervalEntry := widget.NewLabelWithData(binding.IntToString(intervalBind))
		form.Add(labelWithStyle("Interval"))
		form.Add(container.NewBorder(nil, nil, nil, intervalEntry, intervalSlider))

		dialog.ShowCustomConfirm("2FA key (TOTP) custom settings", "OK", "Cancel", container.NewStack(form), func(b bool) {
			if b {
				totp = copy
			}
		}, w)
	})

	form := container.New(layout.NewFormLayout())

	form.Add(labelWithStyle("2FA key"))
	form.Add(container.NewBorder(nil, nil, nil, settingsButton, keyEntry))

	return form, totp
}

func (t *TOTP) Show(ctx context.Context, w fyne.Window) []fyne.CanvasObject {

	totp := binding.NewString()

	keyLabel := widget.NewLabel("")
	totp.AddListener(binding.NewDataListener(func() {
		v, _ := totp.Get()
		m := len(v) / 2
		keyLabel.SetText(v[0:m] + " " + v[m:])
	}))

	now := time.Now().UTC()
	v, _ := otp.TOTPFromBase32(t.Hasher(), t.Secret, now, t.Interval, t.Digits)
	totp.Set(v)

	progressbar := widget.NewProgressBar()
	progressbar.Min = 0
	progressbar.Max = float64(t.Interval)

	progressbar.SetValue(float64(t.Interval - (now.Second() % t.Interval)))
	progressbar.TextFormatter = func() string {
		return fmt.Sprintf("%.0f", progressbar.Value)
	}

	ticker := time.NewTicker(1 * time.Second)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				v := progressbar.Value
				if v == 1 {
					v, _ := otp.TOTPFromBase32(sha1.New, t.Secret, time.Now().UTC(), t.Interval, t.Digits)
					totp.Set(v)
					progressbar.SetValue(progressbar.Max)
				} else {
					progressbar.SetValue(v - 1)
				}
			}
		}
	}()

	b := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
		v, _ := totp.Get()
		w.Clipboard().SetContent(v)
		fyne.CurrentApp().SendNotification(&fyne.Notification{
			Title:   "paw",
			Content: "2FA key copied",
		})
	})

	return []fyne.CanvasObject{labelWithStyle("2FA key"), container.NewBorder(nil, nil, keyLabel, b, progressbar)}
}
