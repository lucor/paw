package paw

import (
	"context"
	"encoding/gob"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lucor.dev/paw/internal/icon"
)

func init() {
	gob.Register((*Password)(nil))
}

// Declare conformity to Item interface
var _ Item = (*Password)(nil)

// Declare conformity to Seeder interface
var _ Seeder = (*Password)(nil)

// Declare conformity to FyneObject interface
var _ FyneObject = (*Password)(nil)

type PasswordMode uint32

const (
	CustomPassword     PasswordMode = 0
	RandomPassword     PasswordMode = 1
	PassphrasePassword PasswordMode = 2
	PinPassword        PasswordMode = 3
	StatelessPassword  PasswordMode = 4
)

func (pm PasswordMode) String() string {
	switch pm {
	case CustomPassword:
		return "Custom"
	case RandomPassword:
		return "Random"
	case StatelessPassword:
		return "Stateless"
	case PinPassword:
		return "Pin"
	case PassphrasePassword:
		return "Passphrase"
	}
	return fmt.Sprintf("Unknown password mode (%d)", pm)
}

type Password struct {
	Value  string       `json:"value,omitempty"`
	Format Format       `json:"format,omitempty"`
	Length int          `json:"length,omitempty"`
	Mode   PasswordMode `json:"mode,omitempty"`

	Metadata `json:"metadata,omitempty"`
	Note     `json:"note,omitempty"`

	fpg FynePasswordGenerator
}

func NewPassword() *Password {
	return &Password{
		Metadata: Metadata{
			Type: PasswordItemType,
		},
		Note: Note{},
	}
}

func (p *Password) SetPasswordGenerator(fpg FynePasswordGenerator) {
	p.fpg = fpg
}

func (p *Password) Edit(ctx context.Context, w fyne.Window) (fyne.CanvasObject, Item) {

	passwordItem := *p
	passwordBind := binding.BindString(&passwordItem.Value)
	titleEntry := widget.NewEntryWithData(binding.BindString(&passwordItem.Name))
	titleEntry.Validator = nil
	titleEntry.PlaceHolder = "Untitled password"

	// the note field
	noteEntry := widget.NewEntryWithData(binding.BindString(&passwordItem.Note.Value))
	noteEntry.MultiLine = true
	noteEntry.Validator = nil

	// center
	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.Bind(passwordBind)
	passwordEntry.Validator = nil
	passwordEntry.SetPlaceHolder("Password")

	passwordCopyButton := widget.NewButtonWithIcon("Copy", theme.ContentCopyIcon(), func() {
		w.Clipboard().SetContent(passwordEntry.Text)
		fyne.CurrentApp().SendNotification(&fyne.Notification{
			Title:   "paw",
			Content: "Password copied to clipboard",
		})
	})

	passwordMakeButton := widget.NewButtonWithIcon("Generate", icon.KeyOutlinedIconThemed, func() {
		p.fpg.ShowPasswordGenerator(passwordBind, &passwordItem, w)
	})

	form := container.New(layout.NewFormLayout())
	form.Add(widget.NewIcon(p.Icon()))
	form.Add(titleEntry)

	form.Add(labelWithStyle("Password"))

	form.Add(container.NewBorder(nil, nil, nil, container.NewHBox(passwordCopyButton, passwordMakeButton), passwordEntry))

	form.Add(labelWithStyle("Note"))
	form.Add(noteEntry)

	return form, &passwordItem
}

func (p *Password) Show(ctx context.Context, w fyne.Window) fyne.CanvasObject {
	obj := titleRow(p.Icon(), p.Name)
	obj = append(obj, copiablePasswordRow("Password", p.Value, w)...)
	if p.Note.Value != "" {
		obj = append(obj, copiableRow("Note", p.Note.Value, w)...)
	}
	return container.New(layout.NewFormLayout(), obj...)
}

// Implemets Seeder interface

func (p *Password) Salt() []byte {
	if p.Mode == StatelessPassword {
		return []byte(p.ID())
	}
	return nil
}

func (p *Password) Info() []byte {
	return nil
}

func (p *Password) Template() (string, error) {
	ruler, err := NewRule(p.Length, p.Format)
	if err != nil {
		return "", err
	}
	return ruler.Template()
}

func (p *Password) Len() int {
	return p.Length
}
