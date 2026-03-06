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
	st := model.ShiftType{Summary: "A Shift", FirstShiftAdvanceDuration: &adv, LastShiftRem: &rem}
	shifts := map[string]model.ShiftType{"A": st}

	loc, _ := i18n.NewLocalizer("nl")
	events, err := IterEvents(f, 30, "Europe/Amsterdam", shifts, nil, nil, nil, loc)
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
	events, err := IterEvents(f, 30, "Europe/Amsterdam", shifts, nil, nil, nil, loc)
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
	events, err := IterEvents(f, 30, "Europe/Amsterdam", map[string]model.ShiftType{}, nil, nil, nil, loc)
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
	events, err := IterEvents(f, 30, "Europe/Amsterdam", map[string]model.ShiftType{}, nil, nil, nil, loc)
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
	events, err := IterEvents(f, 30, "Europe/Amsterdam", map[string]model.ShiftType{}, nil, nil, nil, loc)
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
	events, err := IterEvents(f, 30, "Europe/Amsterdam", shifts, nil, nil, nil, loc)
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
	events, err := IterEvents(f, 30, "Europe/Amsterdam", map[string]model.ShiftType{}, nil, nil, nil, loc)
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
			Summary:                   "KHR",
			FirstShiftAdvanceDuration: &adv,
			LastShiftRem:              &rem,
			Schedules:                 []model.Schedule{sunOnlySched},
		},
	}
	exceptions := map[string]model.Exception{
		"2026-04-06": {Description: "Pasen", Weekday: "Sun"},
	}

	loc, _ := i18n.NewLocalizer("en")

	// Without exception: 2026-04-06 is Monday → Sat/Sun slot doesn't match →
	// rangeEntry=nil → trips=nil → 4h (240min) default duration + 30min remains = 270min.
	events, err := IterEvents(f, 30, "Europe/Amsterdam", shifts, seasons, nil, nil, loc)
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
	events, err = IterEvents(f, 30, "Europe/Amsterdam", shifts, seasons, exceptions, nil, loc)
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

func TestIterEvents_FirstShiftCount(t *testing.T) {
	f := excelize.NewFile()
	s := f.GetSheetName(0)
	f.SetCellValue(s, "A1", "03-apr-26")
	f.SetCellValue(s, "B1", "A")
	f.SetCellValue(s, "C1", "10:00")
	f.SetCellValue(s, "A2", "03-apr-26")
	f.SetCellValue(s, "B2", "A")
	f.SetCellValue(s, "C2", "12:00")
	f.SetCellValue(s, "A3", "03-apr-26")
	f.SetCellValue(s, "B3", "A")
	f.SetCellValue(s, "C3", "14:00")

	adv := 45
	count := 2
	st := model.ShiftType{FirstShiftAdvanceDuration: &adv, FirstShiftCount: &count}
	loc, _ := i18n.NewLocalizer("en")
	events, err := IterEvents(f, 10, "Europe/Amsterdam", map[string]model.ShiftType{"A": st}, nil, nil, nil, loc)
	if err != nil {
		t.Fatalf("IterEvents error: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events got %d", len(events))
	}
	tz := events[0].DtStart.Location()
	// positions 0 and 1 (10:00, 12:00) get 45m advance; position 2 (14:00) gets default 10m
	want0 := time.Date(2026, 4, 3, 10, 0, 0, 0, tz).Add(-45 * time.Minute)
	want1 := time.Date(2026, 4, 3, 12, 0, 0, 0, tz).Add(-45 * time.Minute)
	want2 := time.Date(2026, 4, 3, 14, 0, 0, 0, tz).Add(-10 * time.Minute)
	if !events[0].DtStart.Equal(want0) {
		t.Fatalf("shift 0: DtStart want %v got %v", want0, events[0].DtStart)
	}
	if !events[1].DtStart.Equal(want1) {
		t.Fatalf("shift 1: DtStart want %v got %v", want1, events[1].DtStart)
	}
	if !events[2].DtStart.Equal(want2) {
		t.Fatalf("shift 2: DtStart want %v got %v", want2, events[2].DtStart)
	}
}

func TestIterEvents_FirstShiftTime(t *testing.T) {
	f := excelize.NewFile()
	s := f.GetSheetName(0)
	f.SetCellValue(s, "A1", "03-apr-26")
	f.SetCellValue(s, "B1", "A")
	f.SetCellValue(s, "C1", "10:00")
	f.SetCellValue(s, "A2", "03-apr-26")
	f.SetCellValue(s, "B2", "A")
	f.SetCellValue(s, "C2", "12:00")

	ft := "09:15" // advance for 10:00 departure = 45m; 12:00 uses default
	st := model.ShiftType{FirstShiftAdvanceTime: &ft}
	loc, _ := i18n.NewLocalizer("en")
	events, err := IterEvents(f, 10, "Europe/Amsterdam", map[string]model.ShiftType{"A": st}, nil, nil, nil, loc)
	if err != nil {
		t.Fatalf("IterEvents error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events got %d", len(events))
	}
	tz := events[0].DtStart.Location()
	// first shift: 10:00 - 09:15 = 45m advance → DtStart 09:15
	want0 := time.Date(2026, 4, 3, 9, 15, 0, 0, tz)
	// second shift: default 10m advance
	want1 := time.Date(2026, 4, 3, 12, 0, 0, 0, tz).Add(-10 * time.Minute)
	if !events[0].DtStart.Equal(want0) {
		t.Fatalf("first shift DtStart want %v got %v", want0, events[0].DtStart)
	}
	if !events[1].DtStart.Equal(want1) {
		t.Fatalf("second shift DtStart want %v got %v", want1, events[1].DtStart)
	}
}

func TestIterEvents_FirstShiftTimeAtOrAfterDeparture_Error(t *testing.T) {
	f := excelize.NewFile()
	s := f.GetSheetName(0)
	f.SetCellValue(s, "A1", "03-apr-26")
	f.SetCellValue(s, "B1", "A")
	f.SetCellValue(s, "C1", "10:00")

	ft := "10:00" // equal to departure → must error
	st := model.ShiftType{FirstShiftAdvanceTime: &ft}
	loc, _ := i18n.NewLocalizer("en")
	_, err := IterEvents(f, 10, "Europe/Amsterdam", map[string]model.ShiftType{"A": st}, nil, nil, nil, loc)
	if err == nil {
		t.Fatalf("expected error when first_shift_time >= departure")
	}
}

func TestIterEvents_FirstShiftTimeAfterDeparture_Error(t *testing.T) {
	f := excelize.NewFile()
	s := f.GetSheetName(0)
	f.SetCellValue(s, "A1", "03-apr-26")
	f.SetCellValue(s, "B1", "A")
	f.SetCellValue(s, "C1", "10:00")

	ft := "11:00" // after departure → must error
	st := model.ShiftType{FirstShiftAdvanceTime: &ft}
	loc, _ := i18n.NewLocalizer("en")
	_, err := IterEvents(f, 10, "Europe/Amsterdam", map[string]model.ShiftType{"A": st}, nil, nil, nil, loc)
	if err == nil {
		t.Fatalf("expected error when first_shift_time > departure")
	}
}

func TestIterEvents_FirstShiftDeterminedByScheduleNotPosition(t *testing.T) {
	// The slot defines morning times (10:20, 10:40) as the only scheduled start
	// times. A person assigned only 14:40 is NOT the "first shift of the day"
	// according to the schedule — 10:20 and 10:40 are — so they should NOT get
	// the first_shift_advance_time prep; they get the default advance instead.
	from := model.DateRange{
		From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC),
	}
	seasons := map[string]model.Season{"s": {from}}
	ft := "09:15"
	shifts := map[string]model.ShiftType{
		"A": {
			FirstShiftAdvanceTime: &ft,
			Schedules: []model.Schedule{{
				Seasons: []string{"s"},
				Slots: []model.Slot{{
					StartTimes: []model.StartTimeGroup{
						{Times: []string{"10:20", "10:40"}},
						{Times: []string{"14:40", "15:00"}},
					},
				}},
			}},
		},
	}

	// Afternoon-only assignment: 14:40 is not in the first scheduled times
	f := excelize.NewFile()
	s := f.GetSheetName(0)
	f.SetCellValue(s, "A1", "03-apr-26")
	f.SetCellValue(s, "B1", "A")
	f.SetCellValue(s, "C1", "14:40")

	loc, _ := i18n.NewLocalizer("en")
	events, err := IterEvents(f, 15, "Europe/Amsterdam", shifts, seasons, nil, nil, loc)
	if err != nil {
		t.Fatalf("IterEvents error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event got %d", len(events))
	}
	tz := events[0].DtStart.Location()
	// 14:40 is NOT a first scheduled time → default 15m advance
	wantStart := time.Date(2026, 4, 3, 14, 25, 0, 0, tz)
	if !events[0].DtStart.Equal(wantStart) {
		t.Fatalf("afternoon-only shift: DtStart want %v got %v", wantStart, events[0].DtStart)
	}

	// Morning assignment: 10:20 IS the first scheduled time → gets 9:15 prep
	f2 := excelize.NewFile()
	s2 := f2.GetSheetName(0)
	f2.SetCellValue(s2, "A1", "03-apr-26")
	f2.SetCellValue(s2, "B1", "A")
	f2.SetCellValue(s2, "C1", "10:20")

	events2, err := IterEvents(f2, 15, "Europe/Amsterdam", shifts, seasons, nil, nil, loc)
	if err != nil {
		t.Fatalf("IterEvents error: %v", err)
	}
	if len(events2) != 1 {
		t.Fatalf("expected 1 event got %d", len(events2))
	}
	// 10:20 IS a first scheduled time → first_shift_advance_time "9:15"
	wantStart2 := time.Date(2026, 4, 3, 9, 15, 0, 0, tz)
	if !events2[0].DtStart.Equal(wantStart2) {
		t.Fatalf("morning shift: DtStart want %v got %v", wantStart2, events2[0].DtStart)
	}
}
