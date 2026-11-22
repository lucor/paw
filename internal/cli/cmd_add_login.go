// SPDX-FileCopyrightText: 2022-2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"strings"

	urfave "github.com/urfave/cli/v3"
	"go.lucor.dev/cxf"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

// newAddLoginCommand registers add login.
func newAddLoginCommand(base *command) *urfave.Command {
	cmd := &AddCmd{command: base, itemPath: itemPath{itemType: model.LoginItemType}}
	return &urfave.Command{
		Name:        "login",
		Usage:       "Adds a " + model.LoginItemType.Label() + " item",
		ArgsUsage:   "VAULT NAME",
		Description: vaultReferenceHelp,
		Before:      cmd.validateLogin,
		Action:      cmd.Run,
		Flags: append(addPasswordFlags(),
			&urfave.StringFlag{Name: "username", Usage: "Login username"},
			&urfave.StringFlag{Name: "url", Usage: "Login website"},
			&urfave.BoolFlag{Name: "totp-stdin", Usage: "Read a Base32 authenticator secret from stdin"},
		),
		OnUsageError: onUsageError,
	}
}

// validateLogin checks the website and the sources for the password and authenticator.
func (cmd *AddCmd) validateLogin(ctx context.Context, c *urfave.Command) (context.Context, error) {
	if err := cmd.validateItem(c); err != nil {
		return ctx, err
	}
	cmd.totpStdin = c.Bool("totp-stdin")
	if c.Bool("password-stdin") && cmd.totpStdin {
		return ctx, invalidArgumentsError("--password-stdin and --totp-stdin cannot be combined")
	}
	if err := cmd.validatePasswordFlags(c); err != nil {
		return ctx, err
	}
	if cmd.totpStdin {
		if !cmd.randomPassword {
			return ctx, invalidArgumentsError("--totp-stdin requires --generate-password because stdin is reserved for the authenticator")
		}
		cmd.input = c.Reader
	}
	cmd.username = c.String("username")
	cmd.usernameSet = c.IsSet("username")
	cmd.urlSet = c.IsSet("url")
	if cmd.urlSet {
		website, err := normalizeLoginURL(c.String("url"))
		if err != nil {
			return ctx, invalidArgumentsError("%w", err)
		}
		cmd.loginURL = website
	}
	return ctx, nil
}

// newLogin prompts for login fields and builds a CXF item.
func (cmd *AddCmd) newLogin() (*model.Item, error) {
	website := cmd.loginURL
	username := cmd.username
	var err error
	if cmd.input == nil && cmd.prompter.interactive {
		if !cmd.urlSet {
			website, err = cmd.prompter.ask("URL")
			if err != nil {
				return nil, err
			}
			website, err = normalizeLoginURL(website)
			if err != nil {
				return nil, err
			}
		}
		if !cmd.usernameSet {
			username, err = cmd.prompter.ask("Username")
			if err != nil {
				return nil, err
			}
		}
	}
	password, err := cmd.generatePassword(paw.CustomPassword)
	if err != nil {
		return nil, err
	}
	item, err := model.NewLogin(cmd.itemName, username, password.Value, model.PasswordSettings{Mode: password.Mode, Length: password.Length, Format: password.Format})
	if err != nil {
		return nil, err
	}
	if website != "" {
		item.Scope = &cxf.CredentialScope{URLs: []string{website}, AndroidApps: []cxf.AndroidAppID{}}
	}
	if cmd.totpStdin {
		secret, err := readAddSecret(cmd.input)
		if err != nil {
			return nil, fmt.Errorf("read authenticator from stdin: %w", err)
		}
		credential, err := parseAddTOTP(secret)
		if err != nil {
			return nil, err
		}
		if err := item.SetTOTP(-1, credential); err != nil {
			return nil, err
		}
	}
	var note string
	if cmd.wizard {
		addNote, err := cmd.prompter.askYesNo("Add a note?", false)
		if err != nil {
			return nil, err
		}
		if addNote {
			note, err = cmd.editNote()
			if err != nil {
				return nil, err
			}
		}
	}
	if note != "" {
		credential := cxf.Note{Type: cxf.CredentialTypeNote, Content: cxf.EditableField{FieldType: cxf.FieldTypeString, Value: note}}
		if err := item.SetNote(-1, credential); err != nil {
			return nil, err
		}
	}
	return item, nil
}

// normalizeLoginURL adds HTTPS to a host-only URL and validates its scheme.
func normalizeLoginURL(value string) (string, error) {
	loginURL := paw.NewLoginURL()
	if err := loginURL.Set(value); err != nil {
		return "", fmt.Errorf("enter a valid website address")
	}
	if loginURL.String() != "" && loginURL.URL().Scheme != "https" {
		return "", fmt.Errorf("website must use HTTPS")
	}
	return loginURL.String(), nil
}

// addTOTP prompts for and attaches an authenticator credential.
func (cmd *AddCmd) addTOTP(item *model.Item) error {
	credentials, err := model.DecodeCredentials(item.Credentials)
	if err != nil {
		return err
	}
	if credentials.TOTP != nil {
		return fmt.Errorf("item already has an authenticator")
	}
	secret, err := cmd.prompter.askPassword("Authenticator secret")
	if err != nil {
		return err
	}
	if secret == "" {
		return nil
	}
	credential := cxf.TOTP{
		Type:      cxf.CredentialTypeTOTP,
		Secret:    normalizeTOTPSecret(secret),
		Algorithm: strings.ToLower(string(paw.TOTPHashDefault)),
		Period:    uint16(paw.TOTPIntervalDefault),
		Digits:    uint16(paw.TOTPDigitsDefault),
	}
	if err := configureTOTP(cmd.prompter, &credential); err != nil {
		return err
	}
	if err := credential.Validate(); err != nil {
		return fmt.Errorf("invalid authenticator secret: %w", err)
	}
	return item.SetTOTP(-1, credential)
}

// parseAddTOTP accepts a Base32 TOTP secret without echoing it in errors.
func parseAddTOTP(value string) (cxf.TOTP, error) {
	credential := cxf.TOTP{
		Type:      cxf.CredentialTypeTOTP,
		Algorithm: strings.ToLower(string(paw.TOTPHashDefault)),
		Digits:    paw.TOTPDigitsDefault,
		Period:    paw.TOTPIntervalDefault,
	}
	value = strings.TrimSpace(value)
	credential.Secret = normalizeTOTPSecret(value)
	if credential.Secret == "" || credential.Validate() != nil {
		return credential, fmt.Errorf("invalid Base32 authenticator secret")
	}
	return credential, nil
}
