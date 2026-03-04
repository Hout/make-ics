package main

import (
	"strings"
	"testing"
	"time"

	"github.com/jeroen/make-ics-go/pkg/model"
)

func intPtr(v int) *int { return &v }

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// minimalConfig builds a Config with two shift types and two distinct windows.
func minimalConfig() model.Config {
	return model.Config{
		Timezone: "Europe/Amsterdam",
		Locale:   "nl_NL",
		ShiftType: map[string]model.ShiftType{
			"AAA_": {
				Summary: "Binnendieze AAA",
				Trips:   intPtr(2),
				DateRanges: []model.DateRange{
					{
						From: date(2026, time.April, 1),
						To:   date(2026, time.June, 30),
						StartTimes: []model.StartTimeGroup{
							{Times: []string{"10:00", "14:00"}},
						},
					},
					{
						From: date(2026, time.July, 1),
						To:   date(2026, time.August, 31),
						StartTimes: []model.StartTimeGroup{
							{Times: []string{"09:00", "13:00", "17:00"}},
						},
					},
				},
			},
			// BBB_ uses the same Apr–Jun window as AAA_, Sat/Sun only.
			"BBB_": {
				Summary: "Binnendieze BBB",
				Trips:   intPtr(1),
				DateRanges: []model.DateRange{
					{
						From:     date(2026, time.April, 1),
						To:       date(2026, time.June, 30),
						Weekdays: []string{"Sat", "Sun"},
						StartTimes: []model.StartTimeGroup{
							{Times: []string{"11:00"}},
						},
					},
				},
			},
		},
	}
}

func TestRenderShiftTable_ContainsSections(t *testing.T) {
	out := renderShiftTable(minimalConfig(), false)

	for _, want := range []string{
		"## 2026-04-01",
		"## 2026-07-01",
		"Binnendieze AAA",
		"Binnendieze BBB",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not contain %q\ngot:\n%s", want, out)
		}
	}
}

func TestRenderShiftTable_WeekdayFilter(t *testing.T) {
	out := renderShiftTable(minimalConfig(), false)

	// Find the BBB_ section (Sat/Sun only) and extract its Mon and Sat rows.
	lines := strings.Split(out, "\n")
	inBBB := false
	var monLine, satLine string
	for _, l := range lines {
		if strings.Contains(l, "### Binnendieze BBB") {
			inBBB = true
			continue
		}
		if inBBB && strings.HasPrefix(l, "###") {
			break // entered next shift section
		}
		if inBBB {
			if strings.HasPrefix(l, "| Mon |") {
				monLine = l
			}
			if strings.HasPrefix(l, "| Sat |") {
				satLine = l
			}
		}
	}

	// BBB_ is Sat/Sun only — Monday should show "–"
	if !strings.Contains(monLine, "–") {
		t.Errorf("Monday row should show – for BBB_; got: %s", monLine)
	}
	// Saturday row for BBB_ should show the time
	if !strings.Contains(satLine, "11:00") {
		t.Errorf("Saturday row should show 11:00 for BBB_; got: %s", satLine)
	}
}

func TestRenderShiftTable_SortedTimes(t *testing.T) {
	out := renderShiftTable(minimalConfig(), false)
	// In the Jul–Aug window AAA_ lists three times; 09:00 should appear
	if !strings.Contains(out, "09:00") {
		t.Errorf("expected time 09:00 in output")
	}
}

func TestRenderShiftTable_NoDuplicateHeadings(t *testing.T) {
	// BBB_ has two DateRanges for the same dates, split by weekday — a common
	// config pattern that previously produced two identical ### headings.
	cfg := model.Config{
		Timezone: "Europe/Amsterdam",
		Locale:   "nl_NL",
		ShiftType: map[string]model.ShiftType{
			"BBB_": {
				Summary: "Binnendieze BBB",
				Trips:   intPtr(1),
				DateRanges: []model.DateRange{
					{
						From:     date(2026, time.April, 1),
						To:       date(2026, time.June, 30),
						Weekdays: []string{"Tue", "Wed", "Thu", "Fri"},
						StartTimes: []model.StartTimeGroup{
							{Times: []string{"13:00", "15:00"}},
						},
					},
					{
						From:     date(2026, time.April, 1),
						To:       date(2026, time.June, 30),
						Weekdays: []string{"Sat", "Sun"},
						StartTimes: []model.StartTimeGroup{
							{Times: []string{"11:00"}},
						},
					},
				},
			},
		},
	}

	out := renderShiftTable(cfg, false)

	// Must appear exactly once.
	count := strings.Count(out, "### Binnendieze BBB")
	if count != 1 {
		t.Errorf("expected exactly 1 occurrence of ### Binnendieze BBB, got %d\noutput:\n%s", count, out)
	}

	// Times from both weekday sub-ranges must be present.
	lines := strings.Split(out, "\n")
	var tueLine, satLine string
	for _, l := range lines {
		if strings.HasPrefix(l, "| Tue |") {
			tueLine = l
		}
		if strings.HasPrefix(l, "| Sat |") {
			satLine = l
		}
	}
	if !strings.Contains(tueLine, "13:00") {
		t.Errorf("Tue row should contain 13:00; got: %s", tueLine)
	}
	if !strings.Contains(satLine, "11:00") {
		t.Errorf("Sat row should contain 11:00; got: %s", satLine)
	}
	if strings.Contains(tueLine, "11:00") {
		t.Errorf("Tue row should not contain Sat-only time 11:00; got: %s", tueLine)
	}
}

func TestTimesForWeekday_NoFilter(t *testing.T) {
	dr := model.DateRange{
		StartTimes: []model.StartTimeGroup{
			{Times: []string{"14:00", "10:00"}},
		},
	}
	got := timesForWeekday(dr, time.Monday)
	want := "10:00, 14:00"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestTimesForWeekday_FilterExcludes(t *testing.T) {
	dr := model.DateRange{
		Weekdays: []string{"Sat", "Sun"},
		StartTimes: []model.StartTimeGroup{
			{Times: []string{"11:00"}},
		},
	}
	if got := timesForWeekday(dr, time.Monday); got != "–" {
		t.Errorf("expected – for excluded weekday, got %q", got)
	}
	if got := timesForWeekday(dr, time.Saturday); got == "–" {
		t.Errorf("expected time for Saturday, got –")
	}
}

func TestTimesForWeekday_Empty(t *testing.T) {
	dr := model.DateRange{}
	if got := timesForWeekday(dr, time.Monday); got != "–" {
		t.Errorf("expected – for empty date range, got %q", got)
	}
}

// --- helpers for new tests ---

func singleEntry(st model.ShiftType, dr model.DateRange) []windowEntry {
	return []windowEntry{{code: "X", summary: st.Summary, dr: dr, shiftType: st}}
}

func tripsConfig() model.ShiftType {
	return model.ShiftType{
		Summary:       "Two-trip shift",
		Trips:         intPtr(2),
		TripDuration:  intPtr(50),
		BreakDuration: intPtr(30),
	}
}

// --- tripStartsForWeekday ---

func TestTripStarts_SingleTrip(t *testing.T) {
	st := model.ShiftType{Trips: intPtr(1), TripDuration: intPtr(90)}
	dr := model.DateRange{StartTimes: []model.StartTimeGroup{{Times: []string{"10:30"}}}}
	if got := tripStartsForWeekday(singleEntry(st, dr), time.Monday); got != "10:30(1)" {
		t.Errorf("got %q want 10:30(1)", got)
	}
}

func TestTripStarts_MultiTrip(t *testing.T) {
	// 10:00 + 50min trip + 30min break = 11:20 for trip 2
	st := tripsConfig()
	dr := model.DateRange{StartTimes: []model.StartTimeGroup{{Times: []string{"10:00"}}}}
	if got := tripStartsForWeekday(singleEntry(st, dr), time.Monday); got != "10:00(1), 11:20(2)" {
		t.Errorf("got %q want \"10:00(1), 11:20(2)\"", got)
	}
}

func TestTripStarts_MultiDeparture(t *testing.T) {
	// 10:00 → (1)10:00 (2)11:20;  14:00 → (1)14:00 (2)15:20
	st := tripsConfig()
	dr := model.DateRange{StartTimes: []model.StartTimeGroup{{Times: []string{"10:00", "14:00"}}}}
	want := "10:00(1), 11:20(2), 14:00(1), 15:20(2)"
	if got := tripStartsForWeekday(singleEntry(st, dr), time.Monday); got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestTripStarts_WeekdayExcluded(t *testing.T) {
	st := model.ShiftType{Trips: intPtr(1), TripDuration: intPtr(60)}
	dr := model.DateRange{
		Weekdays:   []string{"Sat", "Sun"},
		StartTimes: []model.StartTimeGroup{{Times: []string{"10:00"}}},
	}
	if got := tripStartsForWeekday(singleEntry(st, dr), time.Monday); got != "–" {
		t.Errorf("expected –, got %q", got)
	}
	if got := tripStartsForWeekday(singleEntry(st, dr), time.Saturday); got != "10:00(1)" {
		t.Errorf("expected 10:00(1) for Sat, got %q", got)
	}
}

func TestTripStarts_FallbackNoDuration(t *testing.T) {
	// No trip_duration — raw start time is emitted as trip 1.
	st := model.ShiftType{Trips: intPtr(2)}
	dr := model.DateRange{StartTimes: []model.StartTimeGroup{{Times: []string{"09:00"}}}}
	if got := tripStartsForWeekday(singleEntry(st, dr), time.Monday); got != "09:00(1)" {
		t.Errorf("got %q want 09:00(1)", got)
	}
}

func TestTripStarts_StartTimeGroupOverride(t *testing.T) {
	// StartTimeGroup.Trips=1 overrides ShiftType.Trips=3.
	one := 1
	st := model.ShiftType{Trips: intPtr(3), TripDuration: intPtr(50), BreakDuration: intPtr(30)}
	dr := model.DateRange{
		StartTimes: []model.StartTimeGroup{{Times: []string{"10:00"}, Trips: &one}},
	}
	if got := tripStartsForWeekday(singleEntry(st, dr), time.Monday); got != "10:00(1)" {
		t.Errorf("got %q want 10:00(1)", got)
	}
}

// --- groupWeekdaysByContent ---

func TestGroupWeekdays_AllSame(t *testing.T) {
	var c [7]string
	for i := range c {
		c[i] = "10:00(1)"
	}
	groups := groupWeekdaysByContent(c)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0].days) != 7 {
		t.Errorf("expected 7 days, got %d", len(groups[0].days))
	}
}

func TestGroupWeekdays_AllDifferent(t *testing.T) {
	var c [7]string
	for i := range c {
		c[i] = weekdayOrder[i].String()
	}
	groups := groupWeekdaysByContent(c)
	if len(groups) != 7 {
		t.Fatalf("expected 7 groups, got %d", len(groups))
	}
}

func TestGroupWeekdays_Mixed(t *testing.T) {
	// Mon: "–", Tue-Sun: "10:00(1)"
	var c [7]string
	c[0] = "–"
	for i := 1; i < 7; i++ {
		c[i] = "10:00(1)"
	}
	groups := groupWeekdaysByContent(c)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if groups[0].val != "–" || len(groups[0].days) != 1 {
		t.Errorf("unexpected first group: %+v", groups[0])
	}
	if len(groups[1].days) != 6 {
		t.Errorf("expected 6 days in second group, got %d", len(groups[1].days))
	}
}

// --- renderShiftTable in trips mode ---

func TestRenderShiftTable_TripsMode(t *testing.T) {
	cfg := model.Config{
		Timezone: "Europe/Amsterdam",
		Locale:   "nl_NL",
		ShiftType: map[string]model.ShiftType{
			"TR2_": {
				Summary:       "Two-trip",
				Trips:         intPtr(2),
				TripDuration:  intPtr(50),
				BreakDuration: intPtr(30),
				DateRanges: []model.DateRange{
					{
						From:       date(2026, time.April, 1),
						To:         date(2026, time.June, 30),
						StartTimes: []model.StartTimeGroup{{Times: []string{"10:00", "14:00"}}},
					},
				},
			},
		},
	}
	out := renderShiftTable(cfg, true)

	if !strings.Contains(out, "| Day | Trips |") {
		t.Error("expected '| Day | Trips |' header")
	}
	// 10:00 dep: trip1=10:00, trip2=11:20; 14:00 dep: trip1=14:00, trip2=15:20
	for _, want := range []string{"10:00(1)", "11:20(2)", "14:00(1)", "15:20(2)"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output", want)
		}
	}
	// All weekdays have same content — no weekday filter — so they collapse.
	if strings.Contains(out, "| Mon |") {
		t.Error("weekdays should be collapsed, unexpected '| Mon |' row")
	}
}

// --- renderMermaidCharts ---

func TestRenderMermaidCharts_Structure(t *testing.T) {
	cfg := model.Config{
		Timezone: "Europe/Amsterdam",
		Locale:   "nl_NL",
		ShiftType: map[string]model.ShiftType{
			"VRK_": {
				Summary:       "VRK",
				Trips:         intPtr(2),
				TripDuration:  intPtr(75),
				BreakDuration: intPtr(30),
				DateRanges: []model.DateRange{
					{
						From:       date(2026, time.April, 1),
						To:         date(2026, time.June, 30),
						Weekdays:   []string{"Tue", "Wed", "Thu", "Fri", "Sat", "Sun"},
						StartTimes: []model.StartTimeGroup{{Times: []string{"10:15", "13:45"}}},
					},
				},
			},
		},
	}
	out := renderMermaidCharts(cfg)

	for _, want := range []string{
		"```mermaid",
		"gantt",
		"dateFormat HH:mm",
		"section VRK",
		// 10:15 departure: 2 trips × 75m + 1 break × 30m = 180m total span
		"10h15 : 10:15, 180m",
		// 13:45 departure: same config = 180m
		"13h45 : 13:45, 180m",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in mermaid output\nfull output:\n%s", want, out)
		}
	}
}
