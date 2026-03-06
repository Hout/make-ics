package drange

import (
	"strings"
	"time"

	"github.com/jeroen/make-ics-go/pkg/model"
)

// ResolvedRange represents the merged result of a Schedule Slot and an optional
// StartTimeGroup override. Fields are pointers to distinguish missing values.
type ResolvedRange struct {
	Trips         *int
	TripDuration  *int
	BreakDuration *int
	FirstAdvance  *int
	LastRemains   *int
}

// containsWeekday reports whether the abbreviation of wd (e.g. "Tue") is present
// in the list. Comparison is case-sensitive and uses the first three letters of
// time.Weekday.String() which matches Go's standard "Mon", "Tue", … strings.
func containsWeekday(weekdays []string, wd time.Weekday) bool {
	abbr := wd.String()[:3]
	for _, w := range weekdays {
		if w == abbr {
			return true
		}
	}
	return false
}

// weekdayFromAbbr converts a three-letter weekday abbreviation ("Mon"…"Sun") to
// the corresponding time.Weekday. Returns false when abbr is not recognised.
func weekdayFromAbbr(abbr string) (time.Weekday, bool) {
	switch abbr {
	case "Sun":
		return time.Sunday, true
	case "Mon":
		return time.Monday, true
	case "Tue":
		return time.Tuesday, true
	case "Wed":
		return time.Wednesday, true
	case "Thu":
		return time.Thursday, true
	case "Fri":
		return time.Friday, true
	case "Sat":
		return time.Saturday, true
	}
	return time.Sunday, false
}

// EffectiveWeekday returns the weekday to use for schedule matching on date.
// If date (formatted as "2006-01-02") is present in exceptions and its Weekday
// field is a valid abbreviation, the override weekday is returned; otherwise
// the calendar weekday of date is returned.
func EffectiveWeekday(date time.Time, exceptions map[string]model.Exception) time.Weekday {
	if exc, ok := exceptions[date.Format("2006-01-02")]; ok {
		if wd, ok := weekdayFromAbbr(exc.Weekday); ok {
			return wd
		}
	}
	return date.Weekday()
}

// dateInSchedule reports whether date falls within any DateRange window of
// any season referenced by sched.
func dateInSchedule(date time.Time, sched model.Schedule, seasons map[string]model.Season) bool {
	d := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	for _, name := range sched.Seasons {
		for _, sr := range seasons[name] {
			f := time.Date(sr.From.Year(), sr.From.Month(), sr.From.Day(), 0, 0, 0, 0, time.UTC)
			t := time.Date(sr.To.Year(), sr.To.Month(), sr.To.Day(), 0, 0, 0, 0, time.UTC)
			if !d.Before(f) && !d.After(t) {
				return true
			}
		}
	}
	return false
}

// resolvedFromSlot builds a ResolvedRange populated from the slot-level fields.
func resolvedFromSlot(slot model.Slot) ResolvedRange {
	return ResolvedRange{
		Trips:         slot.Trips,
		TripDuration:  slot.TripDuration,
		BreakDuration: slot.BreakDuration,
		FirstAdvance:  slot.FirstAdvance,
		LastRemains:   slot.LastRemains,
	}
}

// FindSchedule finds the first schedule whose seasons cover apptDate, then the
// first slot within that schedule whose weekday set contains effectiveWeekday.
// If a start_time matches a group's Times, the group's fields override the
// slot-level fields. An empty weekdays list on a slot means all days are allowed.
// effectiveWeekday is used for weekday matching instead of apptDate.Weekday(),
// allowing exception dates to be treated as a different day of the week.
// Returns nil when no schedule/slot matches.
func FindSchedule(schedules []model.Schedule, apptDate time.Time, startTime string, effectiveWeekday time.Weekday, seasons map[string]model.Season) *ResolvedRange {
	for _, sched := range schedules {
		if !dateInSchedule(apptDate, sched, seasons) {
			continue
		}
		for _, slot := range sched.Slots {
			if len(slot.Weekdays) > 0 && !containsWeekday(slot.Weekdays, effectiveWeekday) {
				continue
			}
			if startTime != "" {
				for _, g := range slot.StartTimes {
					for _, tm := range g.Times {
						if strings.TrimSpace(tm) == strings.TrimSpace(startTime) {
							rr := resolvedFromSlot(slot)
							if g.Trips != nil {
								rr.Trips = g.Trips
							}
							if g.TripDuration != nil {
								rr.TripDuration = g.TripDuration
							}
							if g.BreakDuration != nil {
								rr.BreakDuration = g.BreakDuration
							}
							if g.FirstAdvance != nil {
								rr.FirstAdvance = g.FirstAdvance
							}
							if g.LastRemains != nil {
								rr.LastRemains = g.LastRemains
							}
							return &rr
						}
					}
				}
			}
			rr := resolvedFromSlot(slot)
			return &rr
		}
	}
	return nil
}
