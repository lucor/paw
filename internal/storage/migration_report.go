package storage

import (
	"errors"
	"fmt"
	"strings"

	"lucor.dev/paw/internal/paw"
)

// MigrationPhase identifies the migration step that failed for an item.
type MigrationPhase string

const (
	// MigrationPhaseRead marks legacy item loading or decoding errors.
	MigrationPhaseRead MigrationPhase = "read"
	// MigrationPhaseConvert marks legacy item conversion errors.
	MigrationPhaseConvert MigrationPhase = "convert"
	// MigrationPhaseValidate marks item or staged data validation errors.
	MigrationPhaseValidate MigrationPhase = "validate"
)

// MigrationReport contains item conversion failures from one migration attempt.
type MigrationReport struct {
	// TotalItems is the number of legacy items considered.
	TotalItems int
	// Failures contains one entry for each item that failed.
	Failures []MigrationItemFailure

	failureByID map[string]int
}

// MigrationItemFailure identifies one legacy item that could not be migrated.
type MigrationItemFailure struct {
	// LegacyID identifies the item in the old vault index.
	LegacyID string
	// Title is the item's legacy title.
	Title string
	// ItemType is the item's Paw category.
	ItemType paw.ItemType
	// Phases lists the migration steps that reported errors for this item.
	Phases []MigrationPhase
	// Err contains the cause or causes without formatting them into the report summary.
	Err error
}

// Error returns a summary without item values or credential data.
func (report *MigrationReport) Error() string {
	if report == nil {
		return "migration failed"
	}
	var message strings.Builder
	fmt.Fprintf(&message, "migration failed: %d/%d items", len(report.Failures), report.TotalItems)
	for _, failure := range report.Failures {
		fmt.Fprintf(&message, "\n- item %s", failure.LegacyID)
		if failure.Title != "" {
			fmt.Fprintf(&message, " (%s)", failure.Title)
		}
		if failure.ItemType != 0 {
			fmt.Fprintf(&message, " [%s]", failure.ItemType)
		}
		if len(failure.Phases) > 0 {
			fmt.Fprintf(&message, " during %s", strings.Join(migrationPhaseNames(failure.Phases), ", "))
		}
		if failure.Err != nil {
			fmt.Fprintf(&message, ": %s", failure.Err)
		}
	}
	return message.String()
}

// migrationPhaseNames returns phase labels without exposing item values.
func migrationPhaseNames(phases []MigrationPhase) []string {
	names := make([]string, len(phases))
	for index, phase := range phases {
		names[index] = string(phase)
	}
	return names
}

// Unwrap returns the item errors for errors.Is and errors.As.
func (report *MigrationReport) Unwrap() []error {
	if report == nil {
		return nil
	}
	errs := make([]error, 0, len(report.Failures))
	for _, failure := range report.Failures {
		errs = append(errs, failure.Err)
	}
	return errs
}

// add records an item failure and combines later errors for that same item.
func (report *MigrationReport) add(failure MigrationItemFailure, phase MigrationPhase) {
	if report == nil || failure.Err == nil {
		return
	}
	if report.failureByID == nil {
		report.failureByID = make(map[string]int)
	}
	if index, ok := report.failureByID[failure.LegacyID]; failure.LegacyID != "" && ok {
		existing := &report.Failures[index]
		existing.Err = errors.Join(existing.Err, failure.Err)
		for _, existingPhase := range existing.Phases {
			if existingPhase == phase {
				return
			}
		}
		existing.Phases = append(existing.Phases, phase)
		return
	}
	failure.Phases = []MigrationPhase{phase}
	report.Failures = append(report.Failures, failure)
	if failure.LegacyID != "" {
		report.failureByID[failure.LegacyID] = len(report.Failures) - 1
	}
}
