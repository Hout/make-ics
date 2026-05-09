package schedule

import (
	"testing"

	"github.com/jeroen/make-ics-go/pkg/i18n"
	"github.com/jeroen/make-ics-go/pkg/model"
	dr "github.com/jeroen/make-ics-go/pkg/range"
)

func intp(v int) *int { return &v }

func TestGetTrips_FromTripTimes(t *testing.T) {
	rr := &dr.ResolvedRange{
		TripTimes: []model.TripTime{{Start: "10:00"}, {Start: "11:00"}},
	}
	got := GetTrips(model.ShiftType{}, rr)
	if got == nil || *got != 2 {
		t.Fatalf("expected 2, got %v", got)
	}
}

func TestGetTrips_FromResolvedRangeTrips(t *testing.T) {
	rr := &dr.ResolvedRange{Trips: intp(3)}
	got := GetTrips(model.ShiftType{}, rr)
	if got == nil || *got != 3 {
		t.Fatalf("expected 3, got %v", got)
	}
}

func TestGetTrips_FromShiftType(t *testing.T) {
	sh := model.ShiftType{Trips: intp(4)}
	got := GetTrips(sh, nil)
	if got == nil || *got != 4 {
		t.Fatalf("expected 4, got %v", got)
	}
}

func TestGetTrips_Nil(t *testing.T) {
	got := GetTrips(model.ShiftType{}, nil)
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestBuildProgramFromTripTimes_English(t *testing.T) {
	loc, err := i18n.NewLocalizer("en")
	if err != nil {
		t.Fatalf("failed to load English localizer: %v", err)
	}
	trips := []model.TripTime{
		{Start: "10:20", Duration: 50},
		{Start: "11:40", Duration: 50},
	}
	got := BuildProgramFromTripTimes(trips, 30, 20, loc)
	if got == "" {
		t.Fatalf("expected non-empty program")
	}
	// Should contain preparation, trip 1, trip 2, aftercare
	want := "09:50 - Preparation\n10:20 - Trip 1\n11:10 - Break 1\n11:40 - Trip 2\naftercare → 12:50"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
