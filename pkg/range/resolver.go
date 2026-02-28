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

// FindDateRange finds the first date range covering apptDate. If a start_time
// matches a group's Times, the group's fields override the entry-level fields.
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
        // if startTime provided, check groups
        if startTime != "" {
            for _, g := range entry.StartTimes {
                for _, tm := range g.Times {
                    if strings.TrimSpace(tm) == strings.TrimSpace(startTime) {
                        // build merged result: group overrides entry
                        rr := ResolvedRange{
                            Trips:         entry.Trips,
                            TripDuration:  entry.TripDuration,
                            BreakDuration: entry.BreakDuration,
                            FirstAdvance:  entry.FirstAdvance,
                            LastRemains:   entry.LastRemains,
                        }
                        if g.Trips != nil { rr.Trips = g.Trips }
                        if g.TripDuration != nil { rr.TripDuration = g.TripDuration }
                        if g.BreakDuration != nil { rr.BreakDuration = g.BreakDuration }
                        if g.FirstAdvance != nil { rr.FirstAdvance = g.FirstAdvance }
                        if g.LastRemains != nil { rr.LastRemains = g.LastRemains }
                        return &rr
                    }
                }
            }
        }
        // no matching group — return entry-level fields
        rr := ResolvedRange{
            Trips:         entry.Trips,
            TripDuration:  entry.TripDuration,
            BreakDuration: entry.BreakDuration,
            FirstAdvance:  entry.FirstAdvance,
            LastRemains:   entry.LastRemains,
        }
        return &rr
    }
    return nil
}
