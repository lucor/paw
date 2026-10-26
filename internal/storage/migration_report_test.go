package storage

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"lucor.dev/paw/internal/paw"
)

// TestMigrationReportCountsItems checks repeated errors count once per legacy item.
func TestMigrationReportCountsItems(t *testing.T) {
	firstErr := errors.New("invalid credential")
	secondErr := errors.New("unsupported format")
	report := &MigrationReport{TotalItems: 120}
	failure := MigrationItemFailure{
		LegacyID: "legacy-42",
		Title:    "GitHub",
		ItemType: paw.LoginItemType,
		Err:      firstErr,
	}
	report.add(failure, MigrationPhaseConvert)
	report.add(MigrationItemFailure{LegacyID: "legacy-42", Err: secondErr}, MigrationPhaseValidate)

	require.Contains(t, report.Error(), "migration failed: 1/120 items")
	require.Contains(t, report.Error(), `item legacy-42 (GitHub) [login] during convert, validate`)
	require.Contains(t, report.Error(), "invalid credential")
	require.Len(t, report.Failures, 1)
	require.Equal(t, "legacy-42", report.Failures[0].LegacyID)
	require.Equal(t, "GitHub", report.Failures[0].Title)
	require.Equal(t, paw.LoginItemType, report.Failures[0].ItemType)
	require.Equal(t, []MigrationPhase{MigrationPhaseConvert, MigrationPhaseValidate}, report.Failures[0].Phases)
	require.ErrorIs(t, report, firstErr)
	require.ErrorIs(t, report, secondErr)
	require.Contains(t, report.Error(), "GitHub")
}

// TestMigrationReportSeparatesItems checks distinct legacy IDs produce distinct failures.
func TestMigrationReportSeparatesItems(t *testing.T) {
	report := &MigrationReport{TotalItems: 2}
	report.add(MigrationItemFailure{LegacyID: "id-1", Err: errors.New("bad note")}, MigrationPhaseRead)
	report.add(MigrationItemFailure{LegacyID: "id-2", Err: errors.New("bad login")}, MigrationPhaseConvert)

	require.Contains(t, report.Error(), "migration failed: 2/2 items")
	require.Contains(t, report.Error(), "item id-1 during read: bad note")
	require.Contains(t, report.Error(), "item id-2 during convert: bad login")
	require.Len(t, report.Failures, 2)
	require.Equal(t, MigrationPhaseRead, report.Failures[0].Phases[0])
	require.Equal(t, MigrationPhaseConvert, report.Failures[1].Phases[0])
}
