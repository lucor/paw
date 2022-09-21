package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"lucor.dev/paw/internal/paw"
)

func (a *app) makeAddItemView() fyne.CanvasObject {
	c := container.NewVBox()
	for _, item := range a.makeEmptyItems() {
		i := item
		metadata := i.GetMetadata()
		fyneItem := NewFyneItem(i)
		o := widget.NewButtonWithIcon(metadata.Type.String(), fyneItem.Icon(), func() {
			a.showEditItemView(fyneItem)
		})
		o.Alignment = widget.ButtonAlignLeading
		c.Add(o)
	}

	return container.NewBorder(a.makeCancelHeaderButton(), nil, nil, nil, container.NewCenter(c))
}

// makeEmptyItems returns a slice of empty paw.Item ready to use as template for
// item's creation
func (a *app) makeEmptyItems() []paw.Item {
	note := paw.NewNote()
	password := paw.NewPassword()
	website := paw.NewLogin()
	website.TOTP = &paw.TOTP{
		Digits:   TOTPDigits(),
		Hash:     paw.TOTPHash(TOTPHash()),
		Interval: TOTPInverval(),
	}
	sshkey := paw.NewSSHKey()

	return []paw.Item{
		note,
		password,
		website,
		sshkey,
	}
}
