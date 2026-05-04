package drange

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jeroen/make-ics-go/pkg/model"
)

// ResolvedRange represents the merged result of a Schedule Slot (and optional
// per-shift override). Fields are pointers to distinguish missing values.
type ResolvedRange struct {
	TripTimes []model.TripTime
	Trips     *int
	Arrive    *string // explicit arrive time from the matched Shift, if set
	Leave     *string // explicit leave time from the matched Shift, if set
}

func sortedShiftKeys(shifts map[string]model.Shift) []string {
	keys := make([]string, 0, len(shifts))
	for k := range shifts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		ii, errI := strconv.Atoi(keys[i])
		jj, errJ := strconv.Atoi(keys[j])
		if errI != nil || errJ != nil {
			return keys[i] < keys[j]
		}
		if ii == jj {
			return keys[i] < keys[j]
		}
		return ii < jj
	})
	return keys
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

// resolvedFromSlot builds a ResolvedRange from the slot-level fields.
func resolvedFromSlot(slot model.Slot) ResolvedRange {
	return ResolvedRange{Trips: slot.Trips}
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
			if len(slot.Shifts) == 0 && len(slot.StartTimes) == 0 {
				return nil
			}
			var mins []int
			if len(slot.Shifts) > 0 {
				for _, shiftKey := range sortedShiftKeys(slot.Shifts) {
					shift := slot.Shifts[shiftKey]
					if len(shift.TripTimes) == 0 {
						continue
					}
					t, _ := time.Parse("15:04", strings.TrimSpace(shift.TripTimes[0].Start))
					mins = append(mins, t.Hour()*60+t.Minute())
				}
			} else {
				for _, g := range slot.StartTimes {
					for _, tm := range g.Times {
						t, _ := time.Parse("15:04", strings.TrimSpace(tm))
						mins = append(mins, t.Hour()*60+t.Minute())
					}
				}
			}
			if len(mins) == 0 {
				return nil
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
// first slot within that schedule whose weekday set contains effectiveWeekday
// and whose start_times (if any) contain startTime.
// A slot with no start_times matches any departure time (wildcard).
// A slot that declares start_times only matches departures listed in those groups.
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
				if len(slot.Shifts) > 0 {
					for _, shiftKey := range sortedShiftKeys(slot.Shifts) {
						shift := slot.Shifts[shiftKey]
						if len(shift.TripTimes) == 0 {
							continue
						}
						if strings.TrimSpace(shift.TripTimes[0].Start) != strings.TrimSpace(startTime) {
							continue
						}
						rr := resolvedFromSlot(slot)
						rr.TripTimes = shift.TripTimes
						tripCount := len(shift.TripTimes)
						rr.Trips = &tripCount
						rr.Arrive = shift.Arrive
						rr.Leave = shift.Leave
						return &rr
					}
				} else {
					for _, g := range slot.StartTimes {
						for _, tm := range g.Times {
							if strings.TrimSpace(tm) == strings.TrimSpace(startTime) {
								rr := resolvedFromSlot(slot)
								if g.Trips != nil {
									rr.Trips = g.Trips
								}
								return &rr
							}
						}
					}
				}
				// startTime was not found in any group. A slot that declares
				// shifts/start_times only covers listed departures; skip to the next slot.
				if len(slot.Shifts) > 0 || len(slot.StartTimes) > 0 {
					continue
				}
			}
			rr := resolvedFromSlot(slot)
			return &rr
		}
	}
	return nil
}
