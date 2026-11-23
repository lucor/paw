// SPDX-FileCopyrightText: 2022-2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"io"

	urfave "github.com/urfave/cli/v3"

	"go.lucor.dev/paw/internal/model"
)

// newAddNoteCommand registers add note.
func newAddNoteCommand(base *command) *urfave.Command {
	cmd := &AddCmd{command: base, itemPath: itemPath{itemType: model.NoteItemType}}
	return &urfave.Command{
		Name:        "note",
		Usage:       "Add a " + model.NoteItemType.Label() + " item",
		ArgsUsage:   "VAULT NAME",
		Description: vaultReferenceHelp,
		Action:      cmd.Run,
		Flags: []urfave.Flag{
			&urfave.StringFlag{Name: "input", Usage: "Read note content from FILE, or use - for stdin"},
		},
		OnUsageError:  onUsageError,
		ArgValidator:  completionArgValidator(cmd.validateNote),
		ShellComplete: base.completeVaults(false),
	}
}

// validateNote checks the arguments and the optional note input source.
func (cmd *AddCmd) validateNote(ctx context.Context, c *urfave.Command) (context.Context, error) {
	if err := cmd.validateItem(c); err != nil {
		return ctx, err
	}
	if c.IsSet("input") {
		if c.String("input") != "-" {
			return ctx, invalidArgumentsError("use --input - to read a note from stdin")
		}
		cmd.input = c.Reader
	}
	return ctx, nil
}

// newNote reads the note from stdin or the terminal prompt.
func (cmd *AddCmd) newNote() (*model.Item, error) {
	if cmd.input != nil {
		data, err := io.ReadAll(cmd.input)
		if err != nil {
			return nil, err
		}
		return model.NewNote(cmd.itemName, string(data))
	}
	content, err := cmd.editNote()
	if err != nil {
		return nil, err
	}
	return model.NewNote(cmd.itemName, content)
}

// editNote prompts for the body of a new note.
func (cmd *AddCmd) editNote() (string, error) {
	return cmd.prompter.editMultiline("Note", "")
}
