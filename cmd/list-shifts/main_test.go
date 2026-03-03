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
	out := renderShiftTable(minimalConfig())

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
	out := renderShiftTable(minimalConfig())

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
	out := renderShiftTable(minimalConfig())
	// In the Jul–Aug window AAA_ lists three times; 09:00 should appear
	if !strings.Contains(out, "09:00") {
		t.Errorf("expected time 09:00 in output")
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
