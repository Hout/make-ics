package drange

import (
	"strings"
	"time"

	"github.com/jeroen/make-ics-go/pkg/model"
)

// ResolvedRange represents the merged result of a DateRange and an optional
// StartTimeGroup overrides. Fields are pointers to distinguish missing values.
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

// resolvedFromEntry builds a ResolvedRange populated from the entry-level fields.
func resolvedFromEntry(entry model.DateRange) ResolvedRange {
	return ResolvedRange{
		Trips:         entry.Trips,
		TripDuration:  entry.TripDuration,
		BreakDuration: entry.BreakDuration,
		FirstAdvance:  entry.FirstAdvance,
		LastRemains:   entry.LastRemains,
	}
}

// FindDateRange finds the first date range covering apptDate. If a start_time
// matches a group's Times, the group's fields override the entry-level fields.
// A non-empty weekdays list restricts the range to those days of the week.
// effectiveWeekday is used for weekday matching instead of apptDate.Weekday(),
// allowing exception dates to be treated as a different day of the week.
// Returns nil when no range matches.
func FindDateRange(dateRanges []model.DateRange, apptDate time.Time, startTime string, effectiveWeekday time.Weekday) *ResolvedRange {
	for _, entry := range dateRanges {
		from := entry.From
		to := entry.To
		// normalize date-only comparison
		d := time.Date(apptDate.Year(), apptDate.Month(), apptDate.Day(), 0, 0, 0, 0, time.UTC)
		f := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
		t := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC)
		if d.Before(f) || d.After(t) {
			continue
		}
		// if weekdays are specified, skip entries that don't match
		if len(entry.Weekdays) > 0 && !containsWeekday(entry.Weekdays, effectiveWeekday) {
			continue
		}
		// if startTime provided, check groups
		if startTime != "" {
			for _, g := range entry.StartTimes {
				for _, tm := range g.Times {
					if strings.TrimSpace(tm) == strings.TrimSpace(startTime) {
						rr := resolvedFromEntry(entry)
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
		rr := resolvedFromEntry(entry)
		return &rr
	}
	return nil
}
