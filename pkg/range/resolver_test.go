package drange

import (
	"testing"
	"time"

	"github.com/jeroen/make-ics-go/pkg/model"
)

// testSeasons returns a map with a single season "s" covering [from, to].
func testSeasons(from, to time.Time) map[string]model.Season {
	return map[string]model.Season{"s": {{From: from, To: to}}}
}

// testSched builds a Schedule referencing season "s" with the provided slots.
func testSched(slots ...model.Slot) model.Schedule {
	return model.Schedule{Seasons: []string{"s"}, Slots: slots}
}

func TestFindSchedule_GroupOverride(t *testing.T) {
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)
	trips := 3
	tripDur := 40
	grp := model.StartTimeGroup{Times: []string{"10:00"}, Trips: &trips, TripDuration: &tripDur}
	slot := model.Slot{StartTimes: []model.StartTimeGroup{grp}}
	sched := testSched(slot)
	seasons := testSeasons(from, to)

	appt := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	rr := FindSchedule([]model.Schedule{sched}, appt, "10:00", appt.Weekday(), seasons)
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

func TestFindSchedule_NoMatch(t *testing.T) {
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	sched := testSched(model.Slot{})
	seasons := testSeasons(from, to)
	appt := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	rr := FindSchedule([]model.Schedule{sched}, appt, "10:00", appt.Weekday(), seasons)
	if rr != nil {
		t.Fatalf("expected nil for out-of-range date")
	}
}

func TestFindSchedule_BoundaryFirstDay(t *testing.T) {
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC)
	sched := testSched(model.Slot{})
	seasons := testSeasons(from, to)
	if FindSchedule([]model.Schedule{sched}, from, "", from.Weekday(), seasons) == nil {
		t.Fatalf("expected match on first day")
	}
}

func TestFindSchedule_BoundaryLastDay(t *testing.T) {
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC)
	sched := testSched(model.Slot{})
	seasons := testSeasons(from, to)
	if FindSchedule([]model.Schedule{sched}, to, "", to.Weekday(), seasons) == nil {
		t.Fatalf("expected match on last day")
	}
}

func TestFindSchedule_EmptyList(t *testing.T) {
	if FindSchedule([]model.Schedule{}, time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), "", time.Monday, nil) != nil {
		t.Fatalf("expected nil for empty schedule list")
	}
}

func TestFindSchedule_StartTimeMismatchReturnsEntry(t *testing.T) {
	adv := 30
	groupTrips := 3
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)
	grp := model.StartTimeGroup{Times: []string{"14:40"}, Trips: &groupTrips}
	slot := model.Slot{FirstAdvance: &adv, StartTimes: []model.StartTimeGroup{grp}}
	sched := testSched(slot)
	seasons := testSeasons(from, to)

	rr := FindSchedule([]model.Schedule{sched}, time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC), "10:00", time.Friday, seasons)
	if rr == nil {
		t.Fatalf("expected slot-level result on startTime mismatch")
	}
	if rr.Trips != nil {
		t.Fatalf("expected Trips=nil (no group match) got %v", *rr.Trips)
	}
}

func TestFindSchedule_GroupFieldOverridesSlot(t *testing.T) {
	slotTrips := 2
	groupTrips := 5
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)
	grp := model.StartTimeGroup{Times: []string{"09:00"}, Trips: &groupTrips}
	slot := model.Slot{Trips: &slotTrips, StartTimes: []model.StartTimeGroup{grp}}
	sched := testSched(slot)
	seasons := testSeasons(from, to)

	rr := FindSchedule([]model.Schedule{sched}, time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC), "09:00", time.Friday, seasons)
	if rr == nil || rr.Trips == nil || *rr.Trips != 5 {
		t.Fatalf("expected group trips=5 to override slot trips=2")
	}
}

func TestFindSchedule_WeekdayMatch(t *testing.T) {
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 26, 0, 0, 0, 0, time.UTC)
	slot := model.Slot{Weekdays: []string{"Tue", "Fri"}}
	sched := testSched(slot)
	seasons := testSeasons(from, to)

	// 2026-04-07 is a Tuesday
	tue := time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC)
	if rr := FindSchedule([]model.Schedule{sched}, tue, "", tue.Weekday(), seasons); rr == nil {
		t.Fatalf("expected match on Tuesday")
	}

	// 2026-04-10 is a Friday
	fri := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	if rr := FindSchedule([]model.Schedule{sched}, fri, "", fri.Weekday(), seasons); rr == nil {
		t.Fatalf("expected match on Friday")
	}
}

func TestFindSchedule_WeekdayNoMatch(t *testing.T) {
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 26, 0, 0, 0, 0, time.UTC)
	slot := model.Slot{Weekdays: []string{"Tue", "Fri"}}
	sched := testSched(slot)
	seasons := testSeasons(from, to)

	// 2026-04-08 is a Wednesday
	wed := time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC)
	if rr := FindSchedule([]model.Schedule{sched}, wed, "", wed.Weekday(), seasons); rr != nil {
		t.Fatalf("expected nil for Wednesday when weekdays=[Tue,Fri]")
	}
}

func TestFindSchedule_EmptyWeekdaysMatchesAll(t *testing.T) {
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)
	slot := model.Slot{} // no weekdays filter
	sched := testSched(slot)
	seasons := testSeasons(from, to)

	for _, d := range []time.Time{
		time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC),  // Mon
		time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC),  // Tue
		time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC), // Sat
	} {
		if rr := FindSchedule([]model.Schedule{sched}, d, "", d.Weekday(), seasons); rr == nil {
			t.Fatalf("expected match for %s with empty weekdays list", d.Weekday())
		}
	}
}

func TestFindSchedule_WeekdayFallThroughToSecondSchedule(t *testing.T) {
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 26, 0, 0, 0, 0, time.UTC)
	trips := 9
	// first schedule: Tue/Fri only; second schedule: all days, has trips override
	sched1 := testSched(model.Slot{Weekdays: []string{"Tue", "Fri"}})
	sched2 := testSched(model.Slot{Trips: &trips})
	seasons := testSeasons(from, to)

	// Wednesday: should skip sched1 (no slot matches weekday) and match sched2
	wed := time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC)
	rr := FindSchedule([]model.Schedule{sched1, sched2}, wed, "", wed.Weekday(), seasons)
	if rr == nil {
		t.Fatalf("expected match from second schedule on Wednesday")
	}
	if rr.Trips == nil || *rr.Trips != 9 {
		t.Fatalf("expected trips=9 from second schedule, got %+v", rr.Trips)
	}
}

func TestFindSchedule_MultiWindowSeason(t *testing.T) {
	// Season with two non-overlapping windows: Apr 1–17 and Sep 1–Oct 31
	seasons := map[string]model.Season{
		"s": {
			{From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC)},
			{From: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)},
		},
	}
	sched := model.Schedule{Seasons: []string{"s"}, Slots: []model.Slot{{}}}
	inApril := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	inSep := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	inSummer := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)

	if FindSchedule([]model.Schedule{sched}, inApril, "", inApril.Weekday(), seasons) == nil {
		t.Fatalf("expected match for date in first window")
	}
	if FindSchedule([]model.Schedule{sched}, inSep, "", inSep.Weekday(), seasons) == nil {
		t.Fatalf("expected match for date in second window")
	}
	if FindSchedule([]model.Schedule{sched}, inSummer, "", inSummer.Weekday(), seasons) != nil {
		t.Fatalf("expected nil for date between the two windows")
	}
}

func TestFindSchedule_MultiSeason(t *testing.T) {
	// Schedule referencing two different seasons
	seasons := map[string]model.Season{
		"spring": {{From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 6, 28, 0, 0, 0, 0, time.UTC)}},
		"autumn": {{From: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)}},
	}
	sched := model.Schedule{Seasons: []string{"spring", "autumn"}, Slots: []model.Slot{{}}}

	if FindSchedule([]model.Schedule{sched}, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), "", time.Friday, seasons) == nil {
		t.Fatalf("expected match for spring date")
	}
	if FindSchedule([]model.Schedule{sched}, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), "", time.Thursday, seasons) == nil {
		t.Fatalf("expected match for autumn date")
	}
	if FindSchedule([]model.Schedule{sched}, time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC), "", time.Wednesday, seasons) != nil {
		t.Fatalf("expected nil for summer date not in either season")
	}
}

func TestEffectiveWeekday(t *testing.T) {
	exceptions := map[string]model.Exception{
		"2026-04-06": {Weekday: "Sun"}, // 2026-04-06 is actually a Monday
	}

	tests := []struct {
		name string
		date time.Time
		want time.Weekday
	}{
		{"exception remaps to Sunday", time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC), time.Sunday},
		{"no exception uses natural weekday", time.Date(2026, 4, 7, 0, 0, 0, 0, time.UTC), time.Tuesday},
		{"unknown abbr falls through", time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC), time.Wednesday},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := EffectiveWeekday(tc.date, exceptions)
			if got != tc.want {
				t.Fatalf("EffectiveWeekday(%s) = %v, want %v", tc.date.Format("2006-01-02"), got, tc.want)
			}
		})
	}
}

func TestEffectiveWeekday_UnknownAbbr(t *testing.T) {
	exceptions := map[string]model.Exception{
		"2026-04-06": {Weekday: "Xyz"}, // invalid abbreviation
	}
	date := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC) // Monday
	if got := EffectiveWeekday(date, exceptions); got != time.Monday {
		t.Fatalf("expected natural Monday for unknown abbr, got %v", got)
	}
}

func TestFindSchedule_ExceptionRemapsWeekday(t *testing.T) {
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC)
	// Slot only matches on Sat/Sun
	slot := model.Slot{Weekdays: []string{"Sat", "Sun"}}
	sched := testSched(slot)
	seasons := testSeasons(from, to)

	// 2026-04-06 is a Monday — naturally would not match Sat/Sun slot.
	exceptionDate := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC)

	// Without exception: no match
	if rr := FindSchedule([]model.Schedule{sched}, exceptionDate, "", exceptionDate.Weekday(), seasons); rr != nil {
		t.Fatalf("expected nil without exception remap, got match")
	}
	// With exception remapping to Sunday: should match
	if rr := FindSchedule([]model.Schedule{sched}, exceptionDate, "", time.Sunday, seasons); rr == nil {
		t.Fatalf("expected match when effectiveWeekday=Sunday")
	}
}

func TestFindSchedule_FirstShiftTimeFromSlot(t *testing.T) {
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)
	ft := "09:30"
	slot := model.Slot{FirstShiftTime: &ft}
	sched := testSched(slot)
	seasons := testSeasons(from, to)

	rr := FindSchedule([]model.Schedule{sched}, time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC), "10:00", time.Friday, seasons)
	if rr == nil {
		t.Fatalf("expected resolved range")
	}
	if rr.FirstShiftTime == nil || *rr.FirstShiftTime != "09:30" {
		t.Fatalf("expected FirstShiftTime=09:30, got %v", rr.FirstShiftTime)
	}
}

func TestFindSchedule_FirstShiftTimeOverriddenByGroup(t *testing.T) {
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)
	slotFT := "09:30"
	grpFT := "08:00"
	grp := model.StartTimeGroup{Times: []string{"10:00"}, FirstShiftTime: &grpFT}
	slot := model.Slot{FirstShiftTime: &slotFT, StartTimes: []model.StartTimeGroup{grp}}
	sched := testSched(slot)
	seasons := testSeasons(from, to)

	rr := FindSchedule([]model.Schedule{sched}, time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC), "10:00", time.Friday, seasons)
	if rr == nil {
		t.Fatalf("expected resolved range")
	}
	if rr.FirstShiftTime == nil || *rr.FirstShiftTime != "08:00" {
		t.Fatalf("expected group FirstShiftTime=08:00 to override slot 09:30, got %v", rr.FirstShiftTime)
	}
}

func TestFindSchedule_FirstShiftCountFromSlot(t *testing.T) {
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)
	count := 2
	slot := model.Slot{FirstShiftCount: &count}
	sched := testSched(slot)
	seasons := testSeasons(from, to)

	rr := FindSchedule([]model.Schedule{sched}, time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC), "", time.Friday, seasons)
	if rr == nil {
		t.Fatalf("expected resolved range")
	}
	if rr.FirstShiftCount == nil || *rr.FirstShiftCount != 2 {
		t.Fatalf("expected FirstShiftCount=2, got %v", rr.FirstShiftCount)
	}
}

func TestFindSchedule_FirstShiftCountOverriddenByGroup(t *testing.T) {
	from := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)
	slotCount := 2
	grpCount := 3
	grp := model.StartTimeGroup{Times: []string{"10:00"}, FirstShiftCount: &grpCount}
	slot := model.Slot{FirstShiftCount: &slotCount, StartTimes: []model.StartTimeGroup{grp}}
	sched := testSched(slot)
	seasons := testSeasons(from, to)

	rr := FindSchedule([]model.Schedule{sched}, time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC), "10:00", time.Friday, seasons)
	if rr == nil {
		t.Fatalf("expected resolved range")
	}
	if rr.FirstShiftCount == nil || *rr.FirstShiftCount != 3 {
		t.Fatalf("expected group FirstShiftCount=3 to override slot count=2, got %v", rr.FirstShiftCount)
	}
}
