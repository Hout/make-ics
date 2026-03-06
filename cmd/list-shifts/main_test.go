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
	out, err := renderShiftTable(minimalConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, want := range []string{
		"## 01-Apr-26 > 30-Jun-26",
		"## 01-Jul-26 > 31-Aug-26",
		"Binnendieze AAA",
		"Binnendieze BBB",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not contain %q\ngot:\n%s", want, out)
		}
	}
}

func TestFormatDateRange_SameMonth(t *testing.T) {
	start := date(2026, time.April, 1)
	end := date(2026, time.April, 17)
	if got := formatDateRange(start, end); got != "01 > 17 Apr-26" {
		t.Errorf("got %q, want \"01 > 17 Apr-26\"", got)
	}
}

func TestFormatDateRange_CrossMonth(t *testing.T) {
	start := date(2026, time.April, 18)
	end := date(2026, time.June, 28)
	if got := formatDateRange(start, end); got != "18-Apr-26 > 28-Jun-26" {
		t.Errorf("got %q, want \"18-Apr-26 > 28-Jun-26\"", got)
	}
}

func TestRenderShiftTable_WeekdayFilter(t *testing.T) {
	out, err := renderShiftTable(minimalConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the BBB_ section (Sat/Sun only).
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
			if strings.HasPrefix(l, "| Mon–Fri |") || strings.HasPrefix(l, "| Mon |") {
				monLine = l
			}
			if strings.HasPrefix(l, "| Sat–Sun |") || strings.HasPrefix(l, "| Sat |") {
				satLine = l
			}
		}
	}

	// BBB_ is Sat/Sun only — Mon–Fri row should be suppressed entirely.
	if monLine != "" {
		t.Errorf("Mon–Fri row should be hidden for BBB_; got: %s", monLine)
	}
	// Sat–Sun row for BBB_ should show the time.
	if !strings.Contains(satLine, "11:00") {
		t.Errorf("Sat–Sun row should show 11:00 for BBB_; got: %s", satLine)
	}
}

func TestRenderShiftTable_SortedTimes(t *testing.T) {
	out, err := renderShiftTable(minimalConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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

	out, err := renderShiftTable(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Must appear exactly once.
	count := strings.Count(out, "### Binnendieze BBB")
	if count != 1 {
		t.Errorf("expected exactly 1 occurrence of ### Binnendieze BBB, got %d\noutput:\n%s", count, out)
	}

	// Times from both weekday sub-ranges must be present.
	lines := strings.Split(out, "\n")
	var tueLine, satLine string
	for _, l := range lines {
		if strings.HasPrefix(l, "| Tue–Fri |") || strings.HasPrefix(l, "| Tue |") {
			tueLine = l
		}
		if strings.HasPrefix(l, "| Sat–Sun |") || strings.HasPrefix(l, "| Sat |") {
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

// --- formatDayRange ---

func TestFormatDayRange(t *testing.T) {
	tests := []struct {
		days []time.Weekday
		want string
	}{
		{[]time.Weekday{time.Monday}, "Mon"},
		{[]time.Weekday{time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday, time.Sunday}, "Tue–Sun"},
		{[]time.Weekday{time.Monday, time.Saturday, time.Sunday}, "Mon, Sat–Sun"},
		{[]time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday, time.Sunday}, "Mon–Sun"},
		{[]time.Weekday{time.Monday, time.Wednesday, time.Friday}, "Mon, Wed, Fri"},
		{[]time.Weekday{time.Monday, time.Tuesday, time.Thursday}, "Mon–Tue, Thu"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			if got := formatDayRange(tc.days); got != tc.want {
				t.Errorf("formatDayRange(%v) = %q, want %q", tc.days, got, tc.want)
			}
		})
	}
}

// --- groupWeekdaysByContent ---

func TestGroupWeekdays_AllSame(t *testing.T) {
	var c [7]string
	for i := range c {
		c[i] = "10:00"
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
	// Mon: "–", Tue-Sun: "10:00"
	var c [7]string
	c[0] = "–"
	for i := 1; i < 7; i++ {
		c[i] = "10:00"
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

// --- mergedTimesForWeekday ---

func TestMergedTimesForWeekday_TripsAnnotation(t *testing.T) {
	tests := []struct {
		name    string
		group   []windowEntry
		wd      time.Weekday
		want    string
		wantErr bool
	}{
		{
			name: "trips from ShiftType",
			group: []windowEntry{{
				code: "AAA_",
				dr: model.DateRange{
					StartTimes: []model.StartTimeGroup{
						{Times: []string{"14:00", "10:00"}},
					},
				},
				shiftType: model.ShiftType{Trips: intPtr(2)},
			}},
			wd:   time.Monday,
			want: "10:00(2), 14:00(2)",
		},
		{
			name: "trips from DateRange overrides ShiftType",
			group: []windowEntry{{
				code: "AAA_",
				dr: model.DateRange{
					Trips: intPtr(3),
					StartTimes: []model.StartTimeGroup{
						{Times: []string{"10:00"}},
					},
				},
				shiftType: model.ShiftType{Trips: intPtr(1)},
			}},
			wd:   time.Monday,
			want: "10:00(3)",
		},
		{
			name: "trips from StartTimeGroup overrides DateRange and ShiftType",
			group: []windowEntry{{
				code: "AAA_",
				dr: model.DateRange{
					Trips: intPtr(2),
					StartTimes: []model.StartTimeGroup{
						{Times: []string{"10:00"}, Trips: intPtr(5)},
					},
				},
				shiftType: model.ShiftType{Trips: intPtr(1)},
			}},
			wd:   time.Monday,
			want: "10:00(5)",
		},
		{
			name: "trips nil at all levels returns error",
			group: []windowEntry{{
				code: "AAA_",
				dr: model.DateRange{
					StartTimes: []model.StartTimeGroup{
						{Times: []string{"10:00"}},
					},
				},
				shiftType: model.ShiftType{},
			}},
			wd:      time.Monday,
			wantErr: true,
		},
		{
			name: "weekday excluded returns dash",
			group: []windowEntry{{
				code: "AAA_",
				dr: model.DateRange{
					Weekdays: []string{"Sat"},
					StartTimes: []model.StartTimeGroup{
						{Times: []string{"10:00"}},
					},
				},
				shiftType: model.ShiftType{Trips: intPtr(2)},
			}},
			wd:   time.Monday,
			want: "\u2013",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := mergedTimesForWeekday(tc.group, tc.wd)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
