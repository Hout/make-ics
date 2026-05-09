package pipeline

import (
	"bytes"
	"testing"
	"time"

	"github.com/jeroen/make-ics-go/pkg/i18n"
	"github.com/jeroen/make-ics-go/pkg/model"
	"github.com/xuri/excelize/v2"
)

func TestIterEvents_DefaultAdvance(t *testing.T) {
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	f.SetCellValue(sheet, "A1", "03-apr-26")
	f.SetCellValue(sheet, "B1", "A")
	f.SetCellValue(sheet, "C1", "10:00")

	shifts := map[string]model.ShiftType{"A": {Summary: "A Shift"}}
	loc, _ := i18n.NewLocalizer("nl")
	events, _, err := IterEvents(f, 30, "Europe/Amsterdam", shifts, nil, nil, nil, loc)
	if err != nil {
		t.Fatalf("IterEvents error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event got %d", len(events))
	}
	e := events[0]
	tz := e.DtStart.Location()
	// default advance = 30min; default duration = 240min
	wantStart := time.Date(2026, 4, 3, 9, 30, 0, 0, tz)
	wantEnd := time.Date(2026, 4, 3, 14, 0, 0, 0, tz)
	if !e.DtStart.Equal(wantStart) {
		t.Fatalf("DtStart want %v got %v", wantStart, e.DtStart)
	}
	if !e.DtEnd.Equal(wantEnd) {
		t.Fatalf("DtEnd want %v got %v", wantEnd, e.DtEnd)
	}
	if e.Summary != "A Shift" {
		t.Fatalf("expected summary %q got %q", "A Shift", e.Summary)
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
	events, _, err := IterEvents(f, 30, "Europe/Amsterdam", shifts, nil, nil, nil, loc)
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

func TestIterEvents_LegacyCodeAliasMatchesConfiguredShift(t *testing.T) {
	f := excelize.NewFile()
	s := f.GetSheetName(0)
	f.SetCellValue(s, "A1", "05-mei-26")
	f.SetCellValue(s, "B1", "HRm_")
	f.SetCellValue(s, "C1", "11:20 uur")

	leave := "13:45"
	arrive := "10:50"
	shifts := map[string]model.ShiftType{
		"HRM": {
			Summary:     "Binnendieze HRM",
			Description: "Binnendieze; Historische route Molenstraat",
			Aliases:     []string{"HRm_"},
			Schedules: []model.Schedule{{
				Seasons: []string{"mid"},
				Slots: []model.Slot{{
					Weekdays: []string{"Tue", "Wed", "Thu", "Fri", "Sat", "Sun"},
					Shifts: map[string]model.Shift{
						"1": {
							TripTimes: []model.TripTime{{Start: "11:20"}, {Start: "12:40"}},
							Arrive:    &arrive,
							Leave:     &leave,
						},
					},
				}},
			}},
		},
	}
	seasons := map[string]model.Season{
		"mid": {{
			From: time.Date(2026, 4, 18, 0, 0, 0, 0, time.UTC),
			To:   time.Date(2026, 6, 28, 0, 0, 0, 0, time.UTC),
		}},
	}

	loc, _ := i18n.NewLocalizer("en")
	events, _, err := IterEvents(f, 30, "Europe/Amsterdam", shifts, seasons, nil, nil, loc)
	if err != nil {
		t.Fatalf("IterEvents error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event got %d", len(events))
	}
	e := events[0]
	if e.Summary != "Binnendieze HRM" {
		t.Fatalf("expected summary %q got %q", "Binnendieze HRM", e.Summary)
	}
	if !bytes.Contains([]byte(e.Description), []byte("Historische route Molenstraat")) {
		t.Fatalf("expected full route description, got %q", e.Description)
	}
	if !bytes.Contains([]byte(e.Description), []byte("11:20")) || !bytes.Contains([]byte(e.Description), []byte("12:40")) {
		t.Fatalf("expected trip times in description, got %q", e.Description)
	}
	wantStart := time.Date(2026, 5, 5, 10, 50, 0, 0, e.DtStart.Location())
	wantEnd := time.Date(2026, 5, 5, 13, 45, 0, 0, e.DtEnd.Location())
	if !e.DtStart.Equal(wantStart) {
		t.Fatalf("DtStart want %v got %v", wantStart, e.DtStart)
	}
	if !e.DtEnd.Equal(wantEnd) {
		t.Fatalf("DtEnd want %v got %v", wantEnd, e.DtEnd)
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
	events, _, err := IterEvents(f, 30, "Europe/Amsterdam", map[string]model.ShiftType{}, nil, nil, nil, loc)
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
	events, _, err := IterEvents(f, 30, "Europe/Amsterdam", map[string]model.ShiftType{}, nil, nil, nil, loc)
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
	events, _, err := IterEvents(f, 30, "Europe/Amsterdam", map[string]model.ShiftType{}, nil, nil, nil, loc)
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
	events, _, err := IterEvents(f, 30, "Europe/Amsterdam", shifts, nil, nil, nil, loc)
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
	events, _, err := IterEvents(f, 30, "Europe/Amsterdam", map[string]model.ShiftType{}, nil, nil, nil, loc)
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
	// Sat/Sun with explicit shifts. Without an exception it produces a
	// default-duration event; with the exception remapping to Sunday it
	// matches the slot and uses the trip times to compute duration.
	f := excelize.NewFile()
	s := f.GetSheetName(0)
	f.SetCellValue(s, "A1", "06-apr-26")
	f.SetCellValue(s, "B1", "KHR_")
	f.SetCellValue(s, "C1", "12:10 uur")

	leave := "14:20"
	seasons := map[string]model.Season{
		"full": {{
			From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
			To:   time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC),
		}},
	}
	sunOnlySched := model.Schedule{
		Seasons: []string{"full"},
		Slots: []model.Slot{{
			Weekdays: []string{"Sat", "Sun"},
			Shifts: map[string]model.Shift{
				"1": {
					TripTimes: []model.TripTime{
						{Start: "12:10", Duration: 50},
						{Start: "13:30", Duration: 50},
					},
					Leave: &leave,
				},
			},
		}},
	}
	shifts := map[string]model.ShiftType{
		"KHR_": {
			Summary:   "KHR",
			Schedules: []model.Schedule{sunOnlySched},
		},
	}
	exceptions := map[string]model.Exception{
		"2026-04-06": {Description: "Pasen", Weekday: "Sun"},
	}

	loc, _ := i18n.NewLocalizer("en")

	// Without exception: 2026-04-06 is Monday → Sat/Sun slot doesn't match →
	// no tripTimes, no leave → default duration 240min.
	events, _, err := IterEvents(f, 30, "Europe/Amsterdam", shifts, seasons, nil, nil, loc)
	if err != nil {
		t.Fatalf("IterEvents error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	defaultEnd := time.Date(2026, 4, 6, 12, 10, 0, 0, events[0].DtEnd.Location()).Add(240 * time.Minute)
	if !events[0].DtEnd.Equal(defaultEnd) {
		t.Fatalf("without exception: DtEnd want %v got %v", defaultEnd, events[0].DtEnd)
	}

	// With exception remapping to Sunday: Sat/Sun slot matches → trips from TripTimes,
	// explicit leave 14:20 → duration = 14:20 - 12:10 = 130min.
	events, _, err = IterEvents(f, 30, "Europe/Amsterdam", shifts, seasons, exceptions, nil, loc)
	if err != nil {
		t.Fatalf("IterEvents error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	tripEnd := time.Date(2026, 4, 6, 14, 20, 0, 0, events[0].DtEnd.Location())
	if !events[0].DtEnd.Equal(tripEnd) {
		t.Fatalf("with exception: DtEnd want %v got %v", tripEnd, events[0].DtEnd)
	}
}

func TestIterEvents_ArriveFromShift(t *testing.T) {
	// A shift with explicit arrive time; must compute advance from it.
	f := excelize.NewFile()
	s := f.GetSheetName(0)
	f.SetCellValue(s, "A1", "03-apr-26")
	f.SetCellValue(s, "B1", "A")
	f.SetCellValue(s, "C1", "10:00")

	arrive := "09:15"
	leave := "14:20"
	seasons := map[string]model.Season{
		"s": {{
			From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
			To:   time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC),
		}},
	}
	shifts := map[string]model.ShiftType{
		"A": {
			Schedules: []model.Schedule{{
				Seasons: []string{"s"},
				Slots: []model.Slot{{
					Shifts: map[string]model.Shift{
						"1": {
							TripTimes: []model.TripTime{
								{Start: "10:00", Duration: 50},
								{Start: "11:30", Duration: 50},
							},
							Arrive: &arrive,
							Leave:  &leave,
						},
					},
				}},
			}},
		},
	}
	loc, _ := i18n.NewLocalizer("en")
	events, _, err := IterEvents(f, 30, "Europe/Amsterdam", shifts, seasons, nil, nil, loc)
	if err != nil {
		t.Fatalf("IterEvents error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event got %d", len(events))
	}
	tz := events[0].DtStart.Location()
	// arrive=9:15, departure=10:00 → advance=45min → DtStart=9:15
	wantStart := time.Date(2026, 4, 3, 9, 15, 0, 0, tz)
	// leave=14:20 → DtEnd=14:20
	wantEnd := time.Date(2026, 4, 3, 14, 20, 0, 0, tz)
	if !events[0].DtStart.Equal(wantStart) {
		t.Fatalf("DtStart want %v got %v", wantStart, events[0].DtStart)
	}
	if !events[0].DtEnd.Equal(wantEnd) {
		t.Fatalf("DtEnd want %v got %v", wantEnd, events[0].DtEnd)
	}
}

func TestIterEvents_SingleTripNoLeave_Error(t *testing.T) {
	// A shift with only 1 trip and no explicit leave must produce an error.
	f := excelize.NewFile()
	s := f.GetSheetName(0)
	f.SetCellValue(s, "A1", "03-apr-26")
	f.SetCellValue(s, "B1", "A")
	f.SetCellValue(s, "C1", "10:00")

	seasons := map[string]model.Season{
		"s": {{
			From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
			To:   time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC),
		}},
	}
	shifts := map[string]model.ShiftType{
		"A": {
			Schedules: []model.Schedule{{
				Seasons: []string{"s"},
				Slots: []model.Slot{{
					Shifts: map[string]model.Shift{
						"1": {TripTimes: []model.TripTime{{Start: "10:00"}}},
					},
				}},
			}},
		},
	}
	loc, _ := i18n.NewLocalizer("en")
	_, _, err := IterEvents(f, 30, "Europe/Amsterdam", shifts, seasons, nil, nil, loc)
	if err == nil {
		t.Fatalf("expected error for single-trip shift without leave")
	}
}

func TestIterEvents_WarningsReturnedNotWrittenToStderr(t *testing.T) {
	// A row with an unparseable time should produce a warning in the returned
	// slice, not write to os.Stderr.
	f := excelize.NewFile()
	s := f.GetSheetName(0)
	f.SetCellValue(s, "A1", "03-apr-26")
	f.SetCellValue(s, "B1", "A")
	f.SetCellValue(s, "C1", "geen-tijd") // bad time

	loc, _ := i18n.NewLocalizer("en")
	events, warnings, err := IterEvents(f, 30, "Europe/Amsterdam", map[string]model.ShiftType{}, nil, nil, nil, loc)
	if err != nil {
		t.Fatalf("IterEvents error: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
	if len(warnings) == 0 {
		t.Fatal("expected at least 1 warning for bad time row, got none")
	}
}
