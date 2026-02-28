package schedule

import (
	"testing"

	"github.com/jeroen/make-ics-go/pkg/i18n"
	"github.com/jeroen/make-ics-go/pkg/model"
	dr "github.com/jeroen/make-ics-go/pkg/range"
)

func intp(v int) *int { return &v }

func TestGetShiftDurationMinutes_WithTrips(t *testing.T) {
	sh := model.ShiftType{TripDuration: intp(50), BreakDuration: intp(15)}
	rr := &dr.ResolvedRange{}
	trips := intp(2)
	got := GetShiftDurationMinutes(sh, rr, trips, 240)
	// 2*50 + (2-1)*15 = 115
	if got != 115 {
		t.Fatalf("expected 115 got %d", got)
	}
}

func TestGetShiftDurationMinutes_Fallback(t *testing.T) {
	sh := model.ShiftType{}
	got := GetShiftDurationMinutes(sh, nil, nil, 240)
	if got != 240 {
		t.Fatalf("expected 240 got %d", got)
	}
}

func TestGetDurationRationale(t *testing.T) {
	sh := model.ShiftType{TripDuration: intp(50), BreakDuration: intp(15)}
	trips := intp(2)
	r := GetDurationRationale(sh, nil, trips, 240, 30)
	if r != "2x50+1x15=115min+30min" {
		t.Fatalf("unexpected rationale: %s", r)
	}
}

func TestBuildTripTimes_FormatSchedule(t *testing.T) {
	times := BuildTripTimes(10, 0, 3, 50, 15)
	if len(times) != 3 {
		t.Fatalf("expected 3 segments")
	}
	s := FormatTripSchedule(times, nil)
	if s == "" {
		t.Fatalf("expected non-empty schedule")
	}
}

var buildProgramCases = []struct {
	name    string
	hour    int
	minute  int
	advance int
	trips   int
	tripDur int
	brkDur  int
	remains int
	wantEN  string
	wantNL  string
}{
	{
		name:    "1 trip",
		hour:    14, minute: 40, advance: 30,
		trips: 1, tripDur: 50, brkDur: 30, remains: 30,
		wantEN: "14:10 - Preparation\n14:40 - Trip 1\n15:30 - aftercare → 16:00",
		wantNL: "14:10 - Voorbereiding\n14:40 - Tocht 1\n15:30 - nazorg → 16:00",
	},
	{
		name:    "3 trips",
		hour:    11, minute: 0, advance: 30,
		trips: 3, tripDur: 50, brkDur: 30, remains: 30,
		wantEN: "10:30 - Preparation\n11:00 - Trip 1\n11:50 - Break 1\n12:20 - Trip 2\n13:10 - Break 2\n13:40 - Trip 3\n14:30 - aftercare → 15:00",
		wantNL: "10:30 - Voorbereiding\n11:00 - Tocht 1\n11:50 - Pauze 1\n12:20 - Tocht 2\n13:10 - Pauze 2\n13:40 - Tocht 3\n14:30 - nazorg → 15:00",
	},
}

func TestBuildProgram_English(t *testing.T) {
	loc, err := i18n.NewLocalizer("../../locales", "en")
	if err != nil {
		t.Fatalf("failed to load English localizer: %v", err)
	}
	for _, tc := range buildProgramCases {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildProgram(tc.hour, tc.minute, tc.advance, tc.trips, tc.tripDur, tc.brkDur, tc.remains, loc)
			if got != tc.wantEN {
				t.Errorf("got:\n%s\nwant:\n%s", got, tc.wantEN)
			}
		})
	}
}

func TestBuildProgram_Dutch(t *testing.T) {
	loc, err := i18n.NewLocalizer("../../locales", "nl_NL")
	if err != nil {
		t.Fatalf("failed to load Dutch localizer: %v", err)
	}
	for _, tc := range buildProgramCases {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildProgram(tc.hour, tc.minute, tc.advance, tc.trips, tc.tripDur, tc.brkDur, tc.remains, loc)
			if got != tc.wantNL {
				t.Errorf("got:\n%s\nwant:\n%s", got, tc.wantNL)
			}
		})
	}
}
