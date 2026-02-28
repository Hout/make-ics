package schedule

import (
	"testing"

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
