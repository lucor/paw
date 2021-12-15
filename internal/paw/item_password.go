package paw

import (
	"encoding/gob"
	"fmt"
	"log"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
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

// Declare conformity to FyneObject interface
var _ FyneObject = (*Password)(nil)

type PasswordMode uint32

const (
	CustomPassword    PasswordMode = 0
	RandomPassword    PasswordMode = 1
	StatelessPassword PasswordMode = 2
	PinPassword       PasswordMode = 3
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
	}
	return fmt.Sprintf("Unknown password mode (%d)", pm)
}

type Password struct {
	secretMaker SecretMaker
	options     PasswordOptions

	Metadata
	Password string

	// to store only stateless mode
	Format Format
	Length int
	Mode   PasswordMode
}

type PasswordOptions struct {
	DefaultFormat Format
	DefaultMode   PasswordMode
	DefaultLength int
	MinLength     int
	MaxLength     int
}

func NewPassword(secretMaker SecretMaker, opts PasswordOptions) *Password {
	return &Password{
		secretMaker: secretMaker,
		options:     opts,
	}
}

func (p *Password) SetOptions(opts PasswordOptions) {
	p.options = opts
}

func (p *Password) SetSecretMaker(sm SecretMaker) {
	p.secretMaker = sm
}

func (p *Password) ID() string {
	return fmt.Sprintf("password/%s", strings.ToLower(p.Title))
}

func (p *Password) Icon() *widget.Icon {
	return widget.NewIcon(icon.PasswordOutlinedIconThemed)
}

func (p *Password) Type() ItemType {
	return PasswordItemType
}

func (p *Password) Edit(w fyne.Window) (fyne.CanvasObject, Item) {

	item := *p
	passwordBind := binding.BindString(&item.Password)
	titleEntry := widget.NewEntryWithData(binding.BindString(&item.Title))
	titleEntry.Validator = nil
	titleEntry.PlaceHolder = "Untitled password"

	// the note field
	noteEntry := widget.NewEntryWithData(binding.BindString(&item.Note))
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
		copy := item
		d := dialog.NewCustomConfirm("Generate password", "Ok", "Cancel", copy.makePasswordDialog(), func(b bool) {
			if b {
				passwordBind.Set(copy.Password)
			}
		}, w)
		d.Show()
	})

	form := container.New(layout.NewFormLayout())
	form.Add(p.Icon())
	form.Add(titleEntry)

	form.Add(labelWithStyle("Password"))

	form.Add(container.NewBorder(nil, nil, nil, container.NewHBox(passwordCopyButton, passwordMakeButton), passwordEntry))

	form.Add(labelWithStyle("Note"))
	form.Add(noteEntry)

	return form, &item
}

func (p *Password) Show(w fyne.Window) fyne.CanvasObject {
	obj := titleRow(p.Icon(), p.Title)
	obj = append(obj, copiablePasswordRow("Password", p.Password, w)...)
	obj = append(obj, copiableRow("Note", p.Note, w)...)
	return container.New(layout.NewFormLayout(), obj...)
}

func (p *Password) makePasswordDialog() fyne.CanvasObject {

	passwordBind := binding.BindString(&p.Password)
	passwordEntry := widget.NewEntryWithData(passwordBind)
	passwordEntry.Validator = nil
	refreshButton := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		secret, err := p.makePassword()
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		passwordBind.Set(secret)
	})

	lengthBind := binding.BindInt(&p.Length)
	if p.Length == 0 {
		lengthBind.Set(p.options.DefaultLength)
	}

	if p.Format == 0 {
		p.Format = p.options.DefaultFormat
	}

	lengthEntry := widget.NewEntryWithData(binding.IntToString(lengthBind))
	lengthEntry.Disabled()
	lengthEntry.Validator = nil
	lengthEntry.OnChanged = func(s string) {
		if s == "" {
			return
		}
		l, err := strconv.Atoi(s)
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		if l < p.options.MinLength || l > p.options.MaxLength {
			log.Printf("password lenght must be between %d and %d, got %d", p.options.MinLength, p.options.MaxLength, l)
			return
		}
		lengthBind.Set(l)
		secret, err := p.makePassword()
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		passwordBind.Set(secret)
	}

	lengthSlider := widget.NewSlider(float64(p.options.MinLength), float64(p.options.MaxLength))
	lengthSlider.OnChanged = func(f float64) {
		lengthBind.Set(int(f))
		secret, err := p.makePassword()
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		passwordBind.Set(secret)
	}

	lowercaseButton := widget.NewCheck("a-z", func(isChecked bool) {
		if isChecked {
			p.Format |= LowercaseFormat
		} else {
			p.Format &^= LowercaseFormat
		}
		secret, err := p.makePassword()
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		passwordBind.Set(secret)
	})
	if (p.Format & LowercaseFormat) != 0 {
		lowercaseButton.SetChecked(true)
	} else {
		lowercaseButton.SetChecked(false)
	}

	uppercaseButton := widget.NewCheck("A-Z", func(isChecked bool) {
		if isChecked {
			p.Format |= UppercaseFormat
		} else {
			p.Format &^= UppercaseFormat
		}
		secret, err := p.makePassword()
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		passwordBind.Set(secret)
	})
	if (p.Format & UppercaseFormat) != 0 {
		uppercaseButton.SetChecked(true)
	} else {
		uppercaseButton.SetChecked(false)
	}

	digitsButton := widget.NewCheck("0-9", func(isChecked bool) {
		if isChecked {
			p.Format |= DigitsFormat
		} else {
			p.Format &^= DigitsFormat
		}
		secret, err := p.makePassword()
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		passwordBind.Set(secret)
	})
	if (p.Format & DigitsFormat) != 0 {
		digitsButton.SetChecked(true)
	} else {
		digitsButton.SetChecked(false)
	}

	symbolsButton := widget.NewCheck("!%$", func(isChecked bool) {
		if isChecked {
			p.Format |= SymbolsFormat
		} else {
			p.Format &^= SymbolsFormat
		}
		secret, err := p.makePassword()
		if err != nil {
			// TODO show dialog
			log.Println(err)
			return
		}
		passwordBind.Set(secret)
	})
	if (p.Format & SymbolsFormat) != 0 {
		symbolsButton.SetChecked(true)
	} else {
		symbolsButton.SetChecked(false)
	}

	optionsForm := widget.NewForm()
	optionsForm.Append(
		"Password",
		container.NewBorder(nil, nil, nil, refreshButton, passwordEntry),
	)

	optionsForm.Append(
		"Length",
		container.NewBorder(nil, nil, nil, lengthEntry, lengthSlider),
	)

	optionsForm.Append(
		"",
		container.NewGridWithColumns(4, lowercaseButton, uppercaseButton, digitsButton, symbolsButton),
	)

	return container.NewMax(optionsForm)
}

func (p *Password) makePassword() (string, error) {
	seeder, err := p.makePasswordSeeder()
	if err != nil {
		return "", fmt.Errorf("could not make password seeder: %w", err)
	}
	secret, err := p.secretMaker.Secret(seeder)
	if err != nil {
		return "", fmt.Errorf("could not generate password: %w", err)
	}
	return secret, nil
}

type passwordSeeder struct {
	password *Password
	Ruler
}

func (p *Password) makePasswordSeeder() (*passwordSeeder, error) {
	ruler, err := NewRule(p.Length, p.Format)
	if err != nil {
		return nil, err
	}
	seeder := &passwordSeeder{
		password: p,
		Ruler:    ruler,
	}
	return seeder, nil
}

func (ps *passwordSeeder) Salt() []byte {
	if ps.password.Mode == StatelessPassword {
		return []byte(ps.password.ID())
	}
	return nil
}

func (ps *passwordSeeder) Len() int {
	return ps.password.Length
}

func (ps *passwordSeeder) Info() []byte {
	if ps.password.Mode == StatelessPassword {
		return []byte(strconv.Itoa(ps.password.Revision))
	}
	return nil
}
