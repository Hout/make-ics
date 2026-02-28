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
// Returns nil when no range matches.
func FindDateRange(dateRanges []model.DateRange, apptDate time.Time, startTime string) *ResolvedRange {
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
		if len(entry.Weekdays) > 0 && !containsWeekday(entry.Weekdays, apptDate.Weekday()) {
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
