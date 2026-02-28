package schedule

import (
    "fmt"
    "strings"
    "time"

    "github.com/jeroen/make-ics-go/pkg/model"
    dr "github.com/jeroen/make-ics-go/pkg/range"
)

// Translator is a minimal interface for localized strings used by schedule helpers.
type Translator interface {
    T(id string, data map[string]interface{}) string
    N(id string, count int, data map[string]interface{}) string
}

func GetTrips(shift model.ShiftType, rangeEntry *dr.ResolvedRange) *int {
    if rangeEntry != nil && rangeEntry.Trips != nil {
        return rangeEntry.Trips
    }
    if shift.Trips != nil {
        return shift.Trips
    }
    return nil
}

func GetShiftDurationMinutes(shift model.ShiftType, rangeEntry *dr.ResolvedRange, trips *int, defaultMinutes int) int {
    if trips == nil || *trips == 0 {
        return defaultMinutes
    }
    // merged lookup: range overrides shift
    var tripDuration *int
    var breakDuration *int
    if rangeEntry != nil && rangeEntry.TripDuration != nil {
        tripDuration = rangeEntry.TripDuration
    } else {
        tripDuration = shift.TripDuration
    }
    if tripDuration == nil {
        return defaultMinutes
    }
    if rangeEntry != nil && rangeEntry.BreakDuration != nil {
        breakDuration = rangeEntry.BreakDuration
    } else {
        breakDuration = shift.BreakDuration
    }
    bd := 0
    if breakDuration != nil {
        bd = *breakDuration
    }
    td := *tripDuration
    n := *trips
    return n*td + max(0, n-1)*bd
}

func GetLastShiftRemains(shift model.ShiftType, rangeEntry *dr.ResolvedRange) int {
    if rangeEntry != nil && rangeEntry.LastRemains != nil {
        return *rangeEntry.LastRemains
    }
    if shift.LastShiftRem != nil {
        return *shift.LastShiftRem
    }
    return 0
}

func GetDurationRationale(shift model.ShiftType, rangeEntry *dr.ResolvedRange, trips *int, defaultMinutes int, lastShiftRemains int) string {
    if trips != nil && *trips > 0 {
        var tripDuration *int
        var breakDuration *int
        if rangeEntry != nil && rangeEntry.TripDuration != nil {
            tripDuration = rangeEntry.TripDuration
        } else {
            tripDuration = shift.TripDuration
        }
        if tripDuration != nil {
            if rangeEntry != nil && rangeEntry.BreakDuration != nil {
                breakDuration = rangeEntry.BreakDuration
            } else {
                breakDuration = shift.BreakDuration
            }
            td := *tripDuration
            bd := 0
            if breakDuration != nil {
                bd = *breakDuration
            }
            n := *trips
            parts := fmt.Sprintf("%dx%d", n, td)
            nbreaks := max(0, n-1)
            if nbreaks > 0 && bd > 0 {
                parts = fmt.Sprintf("%s+%dx%d", parts, nbreaks, bd)
            }
            base := n*td + nbreaks*bd
            rationale := fmt.Sprintf("%s=%dmin", parts, base)
            if lastShiftRemains > 0 {
                rationale = fmt.Sprintf("%s+%dmin", rationale, lastShiftRemains)
            }
            return rationale
        }
    }
    return fmt.Sprintf("%dmin (default)", defaultMinutes)
}

func BuildTripTimes(hour int, minute int, trips int, tripDuration int, breakDuration int) []struct{ Start, End string } {
    var out []struct{ Start, End string }
    offset := 0
    for i := 0; i < trips; i++ {
        startH := hour
        startM := minute + offset
        // normalize minutes into hour/min
        sh := startH + (startM / 60)
        sm := startM % 60
        endMTotal := startM + tripDuration
        eh := hour + (endMTotal / 60)
        em := endMTotal % 60
        out = append(out, struct{ Start, End string }{fmt.Sprintf("%02d:%02d", sh, sm), fmt.Sprintf("%02d:%02d", eh, em)})
        offset += tripDuration + breakDuration
    }
    return out
}

func FormatTripSchedule(tripTimes []struct{ Start, End string }, t Translator) string {
    n := len(tripTimes)
    segments := make([]string, n)
    for i, seg := range tripTimes {
        segments[i] = fmt.Sprintf("%s-%s", seg.Start, seg.End)
    }
    tripWord := "trip"
    andWord := "and"
    if t != nil {
        tripWord = t.N("trip", n, nil)
        andWord = t.T("and", nil)
    }
    if n <= 1 {
        return fmt.Sprintf("%d %s: %s", n, tripWord, segments[0])
    }
    return fmt.Sprintf("%d %s: %s %s %s", n, tripWord, strings.Join(segments[:n-1], ", "), andWord, segments[n-1])
}

// BuildProgram returns a multi-line program like the Python original.
func BuildProgram(hour int, minute int, advance int, trips int, tripDuration int, breakDuration int, remains int, t Translator) string {
    base := time.Date(0, 1, 1, hour, minute, 0, 0, time.UTC)
    var lines []string
    if advance > 0 {
        prep := base.Add(-time.Duration(advance) * time.Minute)
        prepLabel := "Preparation"
        if t != nil {
            prepLabel = t.T("Preparation", nil)
        }
        lines = append(lines, fmt.Sprintf("%02d:%02d %s", prep.Hour(), prep.Minute(), prepLabel))
    }
    cur := base
    for i := 1; i <= trips; i++ {
        tripLabel := fmt.Sprintf("Trip %d", i)
        if t != nil {
            tripLabel = t.T("Trip", map[string]interface{}{"n": i})
        }
        lines = append(lines, fmt.Sprintf("%02d:%02d %s", cur.Hour(), cur.Minute(), tripLabel))
        end := cur.Add(time.Duration(tripDuration) * time.Minute)
        if i < trips {
            breakLabel := fmt.Sprintf("Break %d", i)
            if t != nil {
                breakLabel = t.T("Break", map[string]interface{}{"n": i})
            }
            lines = append(lines, fmt.Sprintf("%02d:%02d %s", end.Hour(), end.Minute(), breakLabel))
            cur = end.Add(time.Duration(breakDuration) * time.Minute)
        } else {
            cur = end
        }
    }
    if remains > 0 {
        afterEnd := cur.Add(time.Duration(remains) * time.Minute)
        afterMsg := fmt.Sprintf("aftercare \u2192 %02d:%02d", afterEnd.Hour(), afterEnd.Minute())
        if t != nil {
            afterMsg = t.T("aftercare", map[string]interface{}{"time": fmt.Sprintf("%02d:%02d", afterEnd.Hour(), afterEnd.Minute())})
        }
        lines = append(lines, fmt.Sprintf("%02d:%02d %s", cur.Hour(), cur.Minute(), afterMsg))
    }
    return strings.Join(lines, "\n")
}
