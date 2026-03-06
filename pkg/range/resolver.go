package drange

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jeroen/make-ics-go/pkg/model"
)

// ResolvedRange represents the merged result of a Schedule Slot and an optional
// StartTimeGroup override. Fields are pointers to distinguish missing values.
// The Src fields record the relative YAML path (within the ShiftType) of the
// struct that contributed that field, for line-number annotations in warnings
// and errors. An empty string means ShiftType level.
type ResolvedRange struct {
	Trips                        *int
	TripDuration                 *int
	BreakDuration                *int
	FirstShiftAdvanceDuration    *int
	FirstShiftAdvanceTime        *string
	FirstShiftCount              *int
	LastRemains                  *int
	FirstShiftAdvanceDurationSrc string // relative path of struct that set FirstShiftAdvanceDuration
	FirstShiftAdvanceTimeSrc     string // relative path of struct that set FirstShiftAdvanceTime
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
// resolvedFromSlot builds a ResolvedRange populated from the slot-level fields.
// slotPath is the relative YAML path (e.g. "schedules[0].slots[1]") used for
// line-number annotation of source fields.
func resolvedFromSlot(slot model.Slot, slotPath string) ResolvedRange {
	rr := ResolvedRange{
		Trips:                     slot.Trips,
		TripDuration:              slot.TripDuration,
		BreakDuration:             slot.BreakDuration,
		FirstShiftAdvanceDuration: slot.FirstShiftAdvanceDuration,
		FirstShiftAdvanceTime:     slot.FirstShiftAdvanceTime,
		FirstShiftCount:           slot.FirstShiftCount,
		LastRemains:               slot.LastRemains,
	}
	if slot.FirstShiftAdvanceDuration != nil {
		rr.FirstShiftAdvanceDurationSrc = slotPath
	}
	if slot.FirstShiftAdvanceTime != nil {
		rr.FirstShiftAdvanceTimeSrc = slotPath
	}
	return rr
}

// FirstScheduledTimes returns the set of the chronologically first count
// departure times (as "HH:MM" strings) from the slot that matches apptDate
// and effectiveWeekday, by collecting and sorting all times across the slot's
// start_times groups. Returns nil when no slot matches or the matched slot
// defines no start_times (caller should fall back to positional ordering).
func FirstScheduledTimes(schedules []model.Schedule, apptDate time.Time, effectiveWeekday time.Weekday, seasons map[string]model.Season, count int) map[string]bool {
	for _, sched := range schedules {
		if !dateInSchedule(apptDate, sched, seasons) {
			continue
		}
		for _, slot := range sched.Slots {
			if len(slot.Weekdays) > 0 && !containsWeekday(slot.Weekdays, effectiveWeekday) {
				continue
			}
			if len(slot.StartTimes) == 0 {
				return nil
			}
			var mins []int
			for _, g := range slot.StartTimes {
				for _, tm := range g.Times {
					t, err := time.Parse("15:04", strings.TrimSpace(tm))
					if err != nil {
						continue
					}
					mins = append(mins, t.Hour()*60+t.Minute())
				}
			}
			sort.Ints(mins)
			if count > len(mins) {
				count = len(mins)
			}
			result := make(map[string]bool, count)
			for _, m := range mins[:count] {
				result[fmt.Sprintf("%02d:%02d", m/60, m%60)] = true
			}
			return result
		}
	}
	return nil
}

// FindSchedule finds the first schedule whose seasons cover apptDate, then the
// first slot within that schedule whose weekday set contains effectiveWeekday.
// If a start_time matches a group's Times, the group's fields override the
// slot-level fields. An empty weekdays list on a slot means all days are allowed.
// effectiveWeekday is used for weekday matching instead of apptDate.Weekday(),
// allowing exception dates to be treated as a different day of the week.
// Returns nil when no schedule/slot matches.
func FindSchedule(schedules []model.Schedule, apptDate time.Time, startTime string, effectiveWeekday time.Weekday, seasons map[string]model.Season) *ResolvedRange {
	for si, sched := range schedules {
		if !dateInSchedule(apptDate, sched, seasons) {
			continue
		}
		for sli, slot := range sched.Slots {
			if len(slot.Weekdays) > 0 && !containsWeekday(slot.Weekdays, effectiveWeekday) {
				continue
			}
			slotPath := fmt.Sprintf("schedules[%d].slots[%d]", si, sli)
			if startTime != "" {
				for _, g := range slot.StartTimes {
					for _, tm := range g.Times {
						if strings.TrimSpace(tm) == strings.TrimSpace(startTime) {
							rr := resolvedFromSlot(slot, slotPath)
							if g.Trips != nil {
								rr.Trips = g.Trips
							}
							if g.TripDuration != nil {
								rr.TripDuration = g.TripDuration
							}
							if g.BreakDuration != nil {
								rr.BreakDuration = g.BreakDuration
							}

							if g.LastRemains != nil {
								rr.LastRemains = g.LastRemains
							}
							return &rr
						}
					}
				}
			}
			rr := resolvedFromSlot(slot, slotPath)
			return &rr
		}
	}
	return nil
}
