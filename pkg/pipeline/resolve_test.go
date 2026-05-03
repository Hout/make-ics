package pipeline

import (
	"testing"
	"time"

	"github.com/jeroen/make-ics-go/pkg/model"
)

func makeRow(code, date, hhmm string) parsedRow {
	d, _ := time.Parse("2006-01-02", date)
	var h, m int
	_, _ = time.Parse("15:04", hhmm) // parse into h/m below
	t, _ := time.Parse("15:04", hhmm)
	h = t.Hour()
	m = t.Minute()
	return parsedRow{Code: code, Date: d, Hour: h, Min: m}
}

func TestResolveRows_UnknownCode_DefaultAdvanceAndDuration(t *testing.T) {
	rows := []parsedRow{makeRow("UNKNOWN", "2026-04-03", "10:00")}
	resolved, err := resolveRows(rows, 30, nil, nil, nil, nil, make(map[string]bool))
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved) != 1 {
		t.Fatalf("expected 1 resolved row got %d", len(resolved))
	}
	r := resolved[0]
	if r.advance != 30 {
		t.Errorf("advance: want 30 got %d", r.advance)
	}
	if r.durationMinutes != defaultAppointmentMinutes {
		t.Errorf("durationMinutes: want %d got %d", defaultAppointmentMinutes, r.durationMinutes)
	}
}

func TestResolveRows_FirstShiftPreparationDuration(t *testing.T) {
	adv := 45
	shifts := map[string]model.ShiftType{"A": {FirstShiftPreparationDuration: &adv}}
	rows := []parsedRow{makeRow("A", "2026-04-03", "10:00")}
	resolved, err := resolveRows(rows, 10, shifts, nil, nil, nil, make(map[string]bool))
	if err != nil {
		t.Fatal(err)
	}
	if resolved[0].advance != 45 {
		t.Errorf("advance: want 45 got %d", resolved[0].advance)
	}
}

func TestResolveRows_FirstShiftPreparationTime(t *testing.T) {
	ft := "09:15"
	shifts := map[string]model.ShiftType{"A": {FirstShiftPreparationTime: &ft}}
	// departure 10:00; prep time 09:15 → advance = 45 min
	rows := []parsedRow{makeRow("A", "2026-04-03", "10:00")}
	resolved, err := resolveRows(rows, 10, shifts, nil, nil, nil, make(map[string]bool))
	if err != nil {
		t.Fatal(err)
	}
	if resolved[0].advance != 45 {
		t.Errorf("advance: want 45 got %d", resolved[0].advance)
	}
}

func TestResolveRows_FirstShiftPreparationTime_AtDeparture_Error(t *testing.T) {
	ft := "10:00"
	shifts := map[string]model.ShiftType{"A": {FirstShiftPreparationTime: &ft}}
	rows := []parsedRow{makeRow("A", "2026-04-03", "10:00")}
	_, err := resolveRows(rows, 10, shifts, nil, nil, nil, make(map[string]bool))
	if err == nil {
		t.Fatal("expected error for prep time == departure")
	}
}

func TestResolveRows_LastShiftRemains(t *testing.T) {
	rem := 30
	shifts := map[string]model.ShiftType{"A": {LastShiftAftercare: &rem}}
	rows := []parsedRow{
		makeRow("A", "2026-04-03", "10:00"),
		makeRow("A", "2026-04-03", "12:00"),
	}
	resolved, err := resolveRows(rows, 10, shifts, nil, nil, nil, make(map[string]bool))
	if err != nil {
		t.Fatal(err)
	}
	if resolved[0].remains != 0 {
		t.Errorf("first row: remains should be 0 got %d", resolved[0].remains)
	}
	if resolved[1].remains != 30 {
		t.Errorf("last row: remains want 30 got %d", resolved[1].remains)
	}
	if resolved[1].durationMinutes != defaultAppointmentMinutes+30 {
		t.Errorf("last row: durationMinutes want %d got %d", defaultAppointmentMinutes+30, resolved[1].durationMinutes)
	}
}

func TestResolveRows_CrossLevelWarningDeduped(t *testing.T) {
	ft := "09:15"
	dur := 45
	shifts := map[string]model.ShiftType{"A": {
		FirstShiftPreparationTime:     &ft,
		FirstShiftPreparationDuration: &dur,
	}}
	rows := []parsedRow{
		makeRow("A", "2026-04-03", "10:00"),
		makeRow("A", "2026-04-04", "10:00"),
	}
	warned := make(map[string]bool)
	_, err := resolveRows(rows, 10, shifts, nil, nil, nil, warned)
	if err != nil {
		t.Fatal(err)
	}
	if !warned["A"] {
		t.Error("expected warnedCrossLevel[A] to be set")
	}
	// warning should only be registered once despite two rows for the same code
	if len(warned) != 1 {
		t.Errorf("expected 1 entry in warned map got %d", len(warned))
	}
}

func TestResolveRows_PositionalFallback_NoStartTimes(t *testing.T) {
	adv := 45
	count := 1
	shifts := map[string]model.ShiftType{"A": {
		FirstShiftPreparationDuration: &adv,
		FirstShiftPreparationCount:    &count,
	}}
	rows := []parsedRow{
		makeRow("A", "2026-04-03", "10:00"), // position 0 → first shift
		makeRow("A", "2026-04-03", "12:00"), // position 1 → not first
	}
	resolved, err := resolveRows(rows, 10, shifts, nil, nil, nil, make(map[string]bool))
	if err != nil {
		t.Fatal(err)
	}
	if resolved[0].advance != 45 {
		t.Errorf("position 0: advance want 45 got %d", resolved[0].advance)
	}
	if resolved[1].advance != 10 {
		t.Errorf("position 1: advance want 10 got %d", resolved[1].advance)
	}
}

func TestResolveRows_SummaryAndDescriptionPrefix(t *testing.T) {
	shifts := map[string]model.ShiftType{
		"A": {Summary: "Alpha Shift", Description: "Route detail"},
	}
	rows := []parsedRow{makeRow("A", "2026-04-03", "10:00")}
	resolved, err := resolveRows(rows, 10, shifts, nil, nil, nil, make(map[string]bool))
	if err != nil {
		t.Fatal(err)
	}
	if resolved[0].summary != "Alpha Shift" {
		t.Errorf("summary: want %q got %q", "Alpha Shift", resolved[0].summary)
	}
	if resolved[0].description != "Route detail\n" {
		t.Errorf("description prefix: want %q got %q", "Route detail\n", resolved[0].description)
	}
}
