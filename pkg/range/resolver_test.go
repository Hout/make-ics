package drange

import (
	"testing"
	"time"

	"github.com/jeroen/make-ics-go/pkg/model"
)

func TestFindDateRange_GroupOverride(t *testing.T) {
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)
	trips := 3
	tripDur := 40
	grp := model.StartTimeGroup{Times: []string{"10:00"}, Trips: &trips, TripDuration: &tripDur}
	dr := model.DateRange{From: from, To: to, StartTimes: []model.StartTimeGroup{grp}}

	appt := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	rr := FindDateRange([]model.DateRange{dr}, appt, "10:00")
	if rr == nil {
		t.Fatalf("expected resolved range")
	}
	if rr.Trips == nil || *rr.Trips != 3 {
		t.Fatalf("expected trips=3 got %+v", rr.Trips)
	}
	if rr.TripDuration == nil || *rr.TripDuration != 40 {
		t.Fatalf("expected trip_duration=40 got %+v", rr.TripDuration)
	}
}

func TestFindDateRange_NoMatch(t *testing.T) {
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	dr := model.DateRange{From: from, To: to}
	appt := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	rr := FindDateRange([]model.DateRange{dr}, appt, "10:00")
	if rr != nil {
		t.Fatalf("expected nil for out-of-range date")
	}
}

func TestFindDateRange_BoundaryFirstDay(t *testing.T) {
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC)
	dr := model.DateRange{From: from, To: to}
	if FindDateRange([]model.DateRange{dr}, from, "") == nil {
		t.Fatalf("expected match on first day")
	}
}

func TestFindDateRange_BoundaryLastDay(t *testing.T) {
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC)
	dr := model.DateRange{From: from, To: to}
	if FindDateRange([]model.DateRange{dr}, to, "") == nil {
		t.Fatalf("expected match on last day")
	}
}

func TestFindDateRange_EmptyList(t *testing.T) {
	if FindDateRange([]model.DateRange{}, time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), "") != nil {
		t.Fatalf("expected nil for empty list")
	}
}

func TestFindDateRange_StartTimeMismatchReturnsEntry(t *testing.T) {
	adv := 30
	groupTrips := 3
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)
	grp := model.StartTimeGroup{Times: []string{"14:40"}, Trips: &groupTrips}
	dr := model.DateRange{From: from, To: to, FirstAdvance: &adv, StartTimes: []model.StartTimeGroup{grp}}

	rr := FindDateRange([]model.DateRange{dr}, time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC), "10:00")
	if rr == nil {
		t.Fatalf("expected entry-level result on startTime mismatch")
	}
	if rr.Trips != nil {
		t.Fatalf("expected Trips=nil (no group match) got %v", *rr.Trips)
	}
}

func TestFindDateRange_GroupFieldOverridesEntry(t *testing.T) {
	entryTrips := 2
	groupTrips := 5
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)
	grp := model.StartTimeGroup{Times: []string{"09:00"}, Trips: &groupTrips}
	dr := model.DateRange{From: from, To: to, Trips: &entryTrips, StartTimes: []model.StartTimeGroup{grp}}

	rr := FindDateRange([]model.DateRange{dr}, time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC), "09:00")
	if rr == nil || rr.Trips == nil || *rr.Trips != 5 {
		t.Fatalf("expected group trips=5 to override entry trips=2")
	}
}
