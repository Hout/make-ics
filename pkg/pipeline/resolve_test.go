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
	var warnings []string
	resolved, err := resolveRows(rows, 30, nil, nil, nil, nil, make(map[string]bool), &warnings)
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
	var warnings []string
	resolved, err := resolveRows(rows, 10, shifts, nil, nil, nil, make(map[string]bool), &warnings)
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
	var warnings []string
	resolved, err := resolveRows(rows, 10, shifts, nil, nil, nil, make(map[string]bool), &warnings)
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
	var warnings []string
	_, err := resolveRows(rows, 10, shifts, nil, nil, nil, make(map[string]bool), &warnings)
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
	var warnings []string
	resolved, err := resolveRows(rows, 10, shifts, nil, nil, nil, make(map[string]bool), &warnings)
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
	var warnings []string
	_, err := resolveRows(rows, 10, shifts, nil, nil, nil, warned, &warnings)
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
	// the warning message should also appear in the returned warnings slice
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning string, got %d: %v", len(warnings), warnings)
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
	var warnings []string
	resolved, err := resolveRows(rows, 10, shifts, nil, nil, nil, make(map[string]bool), &warnings)
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
	var warnings []string
	resolved, err := resolveRows(rows, 10, shifts, nil, nil, nil, make(map[string]bool), &warnings)
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

func TestResolveRows_GeneralPreparationDuration_AndFirstShiftOverride(t *testing.T) {
	prep := 20
	firstPrep := 45
	count := 1
	shifts := map[string]model.ShiftType{"A": {
		PreparationDuration:           &prep,
		FirstShiftPreparationDuration: &firstPrep,
		FirstShiftPreparationCount:    &count,
	}}
	rows := []parsedRow{
		makeRow("A", "2026-04-03", "10:00"),
		makeRow("A", "2026-04-03", "12:00"),
	}
	var warnings []string
	resolved, err := resolveRows(rows, 10, shifts, nil, nil, nil, make(map[string]bool), &warnings)
	if err != nil {
		t.Fatal(err)
	}
	if resolved[0].advance != 45 {
		t.Errorf("first row advance: want 45 got %d", resolved[0].advance)
	}
	if resolved[1].advance != 20 {
		t.Errorf("second row advance: want 20 got %d", resolved[1].advance)
	}
}

func TestResolveRows_GeneralAftercare_AndLastShiftOverrides(t *testing.T) {
	aftercare := 15
	lastAftercare := 30
	shifts := map[string]model.ShiftType{"A": {
		AftercareDuration:          &aftercare,
		LastShiftAftercareDuration: &lastAftercare,
	}}
	rows := []parsedRow{
		makeRow("A", "2026-04-03", "10:00"),
		makeRow("A", "2026-04-03", "12:00"),
	}
	var warnings []string
	resolved, err := resolveRows(rows, 10, shifts, nil, nil, nil, make(map[string]bool), &warnings)
	if err != nil {
		t.Fatal(err)
	}
	if resolved[0].remains != 15 {
		t.Errorf("first row remains: want 15 got %d", resolved[0].remains)
	}
	if resolved[0].durationMinutes != defaultAppointmentMinutes+15 {
		t.Errorf("first row durationMinutes: want %d got %d", defaultAppointmentMinutes+15, resolved[0].durationMinutes)
	}
	if resolved[1].remains != 30 {
		t.Errorf("last row remains: want 30 got %d", resolved[1].remains)
	}
	if resolved[1].durationMinutes != defaultAppointmentMinutes+30 {
		t.Errorf("last row durationMinutes: want %d got %d", defaultAppointmentMinutes+30, resolved[1].durationMinutes)
	}
}

func TestResolveRows_LastShiftAftercareTime(t *testing.T) {
	aftercareTime := "16:30"
	shifts := map[string]model.ShiftType{"A": {
		LastShiftAftercareTime: &aftercareTime,
	}}
	rows := []parsedRow{makeRow("A", "2026-04-03", "12:00")}
	var warnings []string
	resolved, err := resolveRows(rows, 10, shifts, nil, nil, nil, make(map[string]bool), &warnings)
	if err != nil {
		t.Fatal(err)
	}
	if resolved[0].remains != 30 {
		t.Errorf("remains: want 30 got %d", resolved[0].remains)
	}
	if resolved[0].durationMinutes != defaultAppointmentMinutes+30 {
		t.Errorf("durationMinutes: want %d got %d", defaultAppointmentMinutes+30, resolved[0].durationMinutes)
	}
}

func TestResolveRows_LastShiftAftercareTime_BeforeComputedEnd_Error(t *testing.T) {
	aftercareTime := "15:30"
	shifts := map[string]model.ShiftType{"A": {
		LastShiftAftercareTime: &aftercareTime,
	}}
	rows := []parsedRow{makeRow("A", "2026-04-03", "12:00")}
	var warnings []string
	_, err := resolveRows(rows, 10, shifts, nil, nil, nil, make(map[string]bool), &warnings)
	if err == nil {
		t.Fatal("expected error when last_shift_aftercare_time is before computed end")
	}
}
