package pipeline

import (
	"bytes"
	"testing"
	"time"

	"github.com/jeroen/make-ics-go/pkg/i18n"
	"github.com/jeroen/make-ics-go/pkg/model"
	"github.com/xuri/excelize/v2"
)

func TestIterEvents_FirstLastAdvanceRemains(t *testing.T) {
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	// Row 1
	f.SetCellValue(sheet, "A1", "03-apr-26")
	f.SetCellValue(sheet, "B1", "A")
	f.SetCellValue(sheet, "C1", "10:00")
	// Row 2 same code/date (last)
	f.SetCellValue(sheet, "A2", "03-apr-26")
	f.SetCellValue(sheet, "B2", "A")
	f.SetCellValue(sheet, "C2", "12:00")

	// shift type with first_shift_advance=45 and last_shift_remains=30
	adv := 45
	rem := 30
	st := model.ShiftType{Summary: "A Shift", FirstShiftAdv: &adv, LastShiftRem: &rem}
	shifts := map[string]model.ShiftType{"A": st}

	loc, _ := i18n.NewLocalizer("nl")
	events, err := IterEvents(f, 30, "Europe/Amsterdam", shifts, nil, nil, loc)
	if err != nil {
		t.Fatalf("IterEvents error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events got %d", len(events))
	}
	// first event should have dtstart = 10:00 - 45min
	e1 := events[0]
	expectedStart := time.Date(2026, 4, 3, 10, 0, 0, 0, e1.DtStart.Location()).Add(-45 * time.Minute)
	if !e1.DtStart.Equal(expectedStart) {
		t.Fatalf("first event dtstart expected %v got %v", expectedStart, e1.DtStart)
	}
	// summary should come from the shift config, not the raw code
	if e1.Summary != "A Shift" {
		t.Fatalf("expected summary %q got %q", "A Shift", e1.Summary)
	}
	// last event end should include +30min
	e2 := events[1]
	// default duration 4h (240min) -> dtend = 12:00 + 240min + 30 = 16:30
	expectedEnd := time.Date(2026, 4, 3, 12, 0, 0, 0, e2.DtEnd.Location()).Add(240*time.Minute + 30*time.Minute)
	if !e2.DtEnd.Equal(expectedEnd) {
		t.Fatalf("last event dtend expected %v got %v", expectedEnd, e2.DtEnd)
	}
}

func TestIterEvents_CodeTrimsWhitespace(t *testing.T) {
	f := excelize.NewFile()
	s := f.GetSheetName(0)
	f.SetCellValue(s, "A1", "03-apr-26")
	f.SetCellValue(s, "B1", "HRm_ ") // trailing space — must still match config
	f.SetCellValue(s, "C1", "10:00 uur")

	shifts := map[string]model.ShiftType{"HRm_": {Summary: "Binnendieze HRM"}}
	loc, _ := i18n.NewLocalizer("en")
	events, err := IterEvents(f, 30, "Europe/Amsterdam", shifts, nil, nil, loc)
	if err != nil {
		t.Fatalf("IterEvents error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event got %d", len(events))
	}
	if events[0].Summary != "Binnendieze HRM" {
		t.Fatalf("expected summary %q got %q", "Binnendieze HRM", events[0].Summary)
	}
}

func TestIterEvents_SkipsNonDataRows(t *testing.T) {
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	f.SetCellValue(sheet, "A1", "Datum")
	f.SetCellValue(sheet, "B1", "Dienst")
	f.SetCellValue(sheet, "C1", "Tijd")
	f.SetCellValue(sheet, "A2", "03-apr-26")
	f.SetCellValue(sheet, "B2", "HRm_")
	f.SetCellValue(sheet, "C2", "14:40 uur")

	loc, _ := i18n.NewLocalizer("en")
	events, err := IterEvents(f, 30, "Europe/Amsterdam", map[string]model.ShiftType{}, nil, nil, loc)
	if err != nil {
		t.Fatalf("IterEvents error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event got %d", len(events))
	}
}

func TestIterEvents_SkipsRowWithUnparseableTime(t *testing.T) {
	f := excelize.NewFile()
	s := f.GetSheetName(0)
	f.SetCellValue(s, "A1", "03-apr-26")
	f.SetCellValue(s, "B1", "HRm_")
	f.SetCellValue(s, "C1", "geen-tijd")

	loc, _ := i18n.NewLocalizer("en")
	events, err := IterEvents(f, 30, "Europe/Amsterdam", map[string]model.ShiftType{}, nil, nil, loc)
	if err != nil {
		t.Fatalf("IterEvents error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events for bad time got %d", len(events))
	}
}

func TestIterEvents_AfspraakFallback(t *testing.T) {
	f := excelize.NewFile()
	s := f.GetSheetName(0)
	f.SetCellValue(s, "A1", "03-apr-26")
	f.SetCellValue(s, "B1", "")
	f.SetCellValue(s, "C1", "10:00 uur")

	loc, _ := i18n.NewLocalizer("en")
	events, err := IterEvents(f, 30, "Europe/Amsterdam", map[string]model.ShiftType{}, nil, nil, loc)
	if err != nil {
		t.Fatalf("IterEvents error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event got %d", len(events))
	}
}

func TestIterEvents_ShiftDescriptionAppended(t *testing.T) {
	f := excelize.NewFile()
	s := f.GetSheetName(0)
	f.SetCellValue(s, "A1", "03-apr-26")
	f.SetCellValue(s, "B1", "HRm_")
	f.SetCellValue(s, "C1", "10:00 uur")

	shifts := map[string]model.ShiftType{"HRm_": {Summary: "HRM", Description: "Some route detail"}}
	loc, _ := i18n.NewLocalizer("en")
	events, err := IterEvents(f, 30, "Europe/Amsterdam", shifts, nil, nil, loc)
	if err != nil {
		t.Fatalf("IterEvents error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event got %d", len(events))
	}
	if !bytes.Contains([]byte(events[0].Description), []byte("Some route detail")) {
		t.Fatalf("expected description to include route detail")
	}
}

func TestIterEvents_EventDatetimesTimezoneAware(t *testing.T) {
	f := excelize.NewFile()
	s := f.GetSheetName(0)
	f.SetCellValue(s, "A1", "03-apr-26")
	f.SetCellValue(s, "B1", "HRm_")
	f.SetCellValue(s, "C1", "14:40 uur")

	loc, _ := i18n.NewLocalizer("en")
	events, err := IterEvents(f, 30, "Europe/Amsterdam", map[string]model.ShiftType{}, nil, nil, loc)
	if err != nil {
		t.Fatalf("IterEvents error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event got %d", len(events))
	}
	if events[0].DtStart.Location().String() == "" {
		t.Fatalf("expected timezone-aware dtstart")
	}
	if events[0].DtEnd.Location().String() == "" {
		t.Fatalf("expected timezone-aware dtend")
	}
}

func TestIterEvents_ExceptionRemapsWeekday(t *testing.T) {
	// 2026-04-06 is a Monday. The shift type only has a schedule for
	// Sat/Sun. Without an exception it would produce no event; with the
	// exception remapping to Sunday it must produce one with different duration.
	f := excelize.NewFile()
	s := f.GetSheetName(0)
	f.SetCellValue(s, "A1", "06-apr-26")
	f.SetCellValue(s, "B1", "KHR_")
	f.SetCellValue(s, "C1", "12:10 uur")

	adv := 30
	rem := 30
	// Trips and TripDuration are only defined on the slot, NOT on the shift
	// itself. This means:
	//   - no slot match  → trips=nil → 4h (240min) default duration
	//   - slot matches   → trips=1, tripDur=50 → 50min + 30min remains = 80min
	slotTrips := 1
	slotTripDur := 50
	seasons := map[string]model.Season{
		"full": {{
			From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
			To:   time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC),
		}},
	}
	sunOnlySched := model.Schedule{
		Seasons: []string{"full"},
		Slots: []model.Slot{{
			Weekdays:     []string{"Sat", "Sun"},
			Trips:        &slotTrips,
			TripDuration: &slotTripDur,
			StartTimes:   []model.StartTimeGroup{{Times: []string{"12:10"}}},
		}},
	}
	shifts := map[string]model.ShiftType{
		"KHR_": {
			Summary:       "KHR",
			FirstShiftAdv: &adv,
			LastShiftRem:  &rem,
			Schedules:     []model.Schedule{sunOnlySched},
		},
	}
	exceptions := map[string]model.Exception{
		"2026-04-06": {Description: "Pasen", Weekday: "Sun"},
	}

	loc, _ := i18n.NewLocalizer("en")

	// Without exception: 2026-04-06 is Monday → Sat/Sun slot doesn't match →
	// rangeEntry=nil → trips=nil → 4h (240min) default duration + 30min remains = 270min.
	events, err := IterEvents(f, 30, "Europe/Amsterdam", shifts, seasons, nil, loc)
	if err != nil {
		t.Fatalf("IterEvents error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	defaultEnd := time.Date(2026, 4, 6, 12, 10, 0, 0, events[0].DtEnd.Location()).Add(270 * time.Minute)
	if !events[0].DtEnd.Equal(defaultEnd) {
		t.Fatalf("without exception: DtEnd want %v got %v", defaultEnd, events[0].DtEnd)
	}

	// With exception remapping to Sunday: Sat/Sun slot matches → trips=1, tripDur=50
	// → duration = 50min + 30min remains = 80min.
	events, err = IterEvents(f, 30, "Europe/Amsterdam", shifts, seasons, exceptions, loc)
	if err != nil {
		t.Fatalf("IterEvents error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	tripEnd := time.Date(2026, 4, 6, 12, 10, 0, 0, events[0].DtEnd.Location()).Add(80 * time.Minute)
	if !events[0].DtEnd.Equal(tripEnd) {
		t.Fatalf("with exception: DtEnd want %v got %v", tripEnd, events[0].DtEnd)
	}
}
