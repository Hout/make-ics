package pipeline

import (
	"testing"

	"github.com/xuri/excelize/v2"
)

func newSheet(rows [][]string) *excelize.File {
	f := excelize.NewFile()
	s := f.GetSheetName(0)
	cols := "ABCDEFGH"
	for r, row := range rows {
		for c, val := range row {
			f.SetCellValue(s, string(cols[c])+string(rune('1'+r)), val)
		}
	}
	return f
}

func TestParseRows_HeaderSkipped(t *testing.T) {
	f := newSheet([][]string{
		{"Datum", "Dienst", "Tijd"},
		{"03-apr-26", "A", "10:00"},
	})
	rows, err := parseRows(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row got %d", len(rows))
	}
	if rows[0].Code != "A" {
		t.Fatalf("expected code A got %q", rows[0].Code)
	}
}

func TestParseRows_UnparseableTimeSkipped(t *testing.T) {
	f := newSheet([][]string{
		{"03-apr-26", "A", "geen-tijd"},
		{"03-apr-26", "B", "10:00"},
	})
	rows, err := parseRows(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row (bad-time row skipped) got %d", len(rows))
	}
	if rows[0].Code != "B" {
		t.Fatalf("expected B got %q", rows[0].Code)
	}
}

func TestParseRows_UnparseableDateSkipped(t *testing.T) {
	f := newSheet([][]string{
		{"not-a-date", "A", "10:00"},
		{"03-apr-26", "B", "10:00"},
	})
	rows, err := parseRows(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row (bad-date row skipped) got %d", len(rows))
	}
}

func TestParseRows_CodeTrimsWhitespace(t *testing.T) {
	f := newSheet([][]string{{"03-apr-26", "  HRm_  ", "10:00"}})
	rows, err := parseRows(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row got %d", len(rows))
	}
	if rows[0].Code != "HRm_" {
		t.Fatalf("expected trimmed code got %q", rows[0].Code)
	}
}

func TestParseRows_AfspraakFallback(t *testing.T) {
	f := newSheet([][]string{{"03-apr-26", "", "10:00"}})
	rows, err := parseRows(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row got %d", len(rows))
	}
	if rows[0].Code != "Afspraak" {
		t.Fatalf("expected Afspraak got %q", rows[0].Code)
	}
}
