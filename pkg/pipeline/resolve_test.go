package pipeline

import (
	"testing"
	"time"

	"github.com/jeroen/make-ics-go/pkg/model"
)

func makeRow(code, date, hhmm string) parsedRow {
	d, _ := time.Parse("2006-01-02", date)
	t, _ := time.Parse("15:04", hhmm)
	return parsedRow{Code: code, Date: d, Hour: t.Hour(), Min: t.Minute()}
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

func TestResolveRows_ArriveFromRangeEntry(t *testing.T) {
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
	rows := []parsedRow{makeRow("A", "2026-04-03", "10:00")}
	var warnings []string
	resolved, err := resolveRows(rows, 30, shifts, seasons, nil, nil, make(map[string]bool), &warnings)
	if err != nil {
		t.Fatal(err)
	}
	// arrive=9:15, departure=10:00 → advance=45
	if resolved[0].advance != 45 {
		t.Errorf("advance: want 45 got %d", resolved[0].advance)
	}
	// leave=14:20, departure=10:00 → durationMinutes=260
	if resolved[0].durationMinutes != 260 {
		t.Errorf("durationMinutes: want 260 got %d", resolved[0].durationMinutes)
	}
}

func TestResolveRows_ArriveAtOrAfterDeparture_Error(t *testing.T) {
	arrive := "10:00" // equal to departure → must error
	leave := "14:00"
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
							TripTimes: []model.TripTime{{Start: "10:00", Duration: 50}, {Start: "11:30", Duration: 50}},
							Arrive:    &arrive,
							Leave:     &leave,
						},
					},
				}},
			}},
		},
	}
	rows := []parsedRow{makeRow("A", "2026-04-03", "10:00")}
	var warnings []string
	_, err := resolveRows(rows, 30, shifts, seasons, nil, nil, make(map[string]bool), &warnings)
	if err == nil {
		t.Fatal("expected error when arrive >= departure")
	}
}

func TestResolveRows_SingleTripNoLeave_Error(t *testing.T) {
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
	rows := []parsedRow{makeRow("A", "2026-04-03", "10:00")}
	var warnings []string
	_, err := resolveRows(rows, 30, shifts, seasons, nil, nil, make(map[string]bool), &warnings)
	if err == nil {
		t.Fatal("expected error for single-trip shift without leave")
	}
}

func TestResolveRows_MultiTripAutoLeave(t *testing.T) {
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
							// No explicit leave: auto-compute from last trip end
						},
					},
				}},
			}},
		},
	}
	rows := []parsedRow{makeRow("A", "2026-04-03", "10:00")}
	var warnings []string
	resolved, err := resolveRows(rows, 30, shifts, seasons, nil, nil, make(map[string]bool), &warnings)
	if err != nil {
		t.Fatal(err)
	}
	// lastTrip = 11:30 + 50min = 12:20 → leaveMinutes=740
	// departure = 10:00 = 600 → durationMinutes=140
	if resolved[0].durationMinutes != 140 {
		t.Errorf("durationMinutes: want 140 got %d", resolved[0].durationMinutes)
	}
}
