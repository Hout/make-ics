package parser

import (
	"testing"
)

func TestParseDutchDate_Valid(t *testing.T) {
	dt, err := ParseDutchDate("03-apr-26")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dt.Year() != 2026 || dt.Month() != 4 || dt.Day() != 3 {
		t.Fatalf("parsed wrong date: %v", dt)
	}
}

func TestParseDutchDate_Invalid(t *testing.T) {
	_, err := ParseDutchDate("2026-04-03")
	if err == nil {
		t.Fatalf("expected error for invalid format")
	}
}

func TestParseDutchDate_Raises_on_Unparseable(t *testing.T) {
	_, err := ParseDutchDate("not-a-date")
	if err == nil {
		t.Fatalf("expected error for unparseable input")
	}
}

func TestParseTime_Valid(t *testing.T) {
	h, m, err := ParseTime("14:40 uur")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h != 14 || m != 40 {
		t.Fatalf("wrong time parsed: %02d:%02d", h, m)
	}
}

func TestParseTime_Invalid(t *testing.T) {
	_, _, err := ParseTime("1440")
	if err == nil {
		t.Fatalf("expected error for invalid time")
	}
}

func TestParseTime_NoColonFormat(t *testing.T) {
	_, _, err := ParseTime("geen tijd")
	if err == nil {
		t.Fatalf("expected error for non-time string")
	}
}

func TestIsDataRow_Valid(t *testing.T) {
	if !IsDataRow([]string{"03-apr-26", "A", "14:40"}) {
		t.Fatalf("expected data row")
	}
}

func TestIsDataRow_EmptyDate(t *testing.T) {
	if IsDataRow([]string{"", "A", "14:40"}) {
		t.Fatalf("expected false for empty date")
	}
}

func TestIsDataRow_TooFewColumns(t *testing.T) {
	if IsDataRow([]string{"03-apr-26", "HRm_"}) {
		t.Fatalf("expected false for too few columns")
	}
}

func TestIsDataRow_HeaderRow(t *testing.T) {
	if IsDataRow([]string{"Datum", "Dienst", "Tijd"}) {
		t.Fatalf("expected false for header row")
	}
}

func TestIsDataRow_TrailingColumns(t *testing.T) {
	if !IsDataRow([]string{"03-apr-26", "HRm_", "14:40 uur", "", ""}) {
		t.Fatalf("expected true with trailing columns")
	}
}
