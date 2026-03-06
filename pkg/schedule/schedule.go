package schedule

import (
	"fmt"
	"strings"
	"time"

	"github.com/jeroen/make-ics-go/pkg/model"
	dr "github.com/jeroen/make-ics-go/pkg/range"
)

// Translator is a minimal interface for localised strings used by schedule helpers.
type Translator interface {
	T(id string, data map[string]any) string
	N(id string, count int, data map[string]any) string
}

// GetTrips returns the effective trip count, with rangeEntry taking precedence
// over the shift-level setting. Returns nil when no count is configured.
func GetTrips(shift model.ShiftType, rangeEntry *dr.ResolvedRange) *int {
	if rangeEntry != nil && rangeEntry.Trips != nil {
		return rangeEntry.Trips
	}
	if shift.Trips != nil {
		return shift.Trips
	}
	return nil
}

// GetShiftDurationMinutes returns the total shift duration in minutes using the
// formula trips×tripDuration + max(0,trips−1)×breakDuration. Falls back to
// defaultMinutes when trips or tripDuration are not configured.
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

// GetLastShiftAftercare returns the extra minutes appended to the last shift of
// a (code, date) group, with rangeEntry taking precedence over the shift setting.
func GetLastShiftAftercare(shift model.ShiftType, rangeEntry *dr.ResolvedRange) int {
	if rangeEntry != nil && rangeEntry.LastAftercare != nil {
		return *rangeEntry.LastAftercare
	}
	if shift.LastShiftAftercare != nil {
		return *shift.LastShiftAftercare
	}
	return 0
}

// GetShiftPreparationDuration returns the per-shift preparation minutes, with
// rangeEntry taking precedence over the shift setting. Returns nil when not
// configured (caller uses its own default).
func GetShiftPreparationDuration(shift model.ShiftType, rangeEntry *dr.ResolvedRange) *int {
	if rangeEntry != nil && rangeEntry.ShiftPreparationDuration != nil {
		return rangeEntry.ShiftPreparationDuration
	}
	if shift.ShiftPreparationDuration != nil {
		return shift.ShiftPreparationDuration
	}
	return nil
}

// GetDurationRationale returns a human-readable breakdown of how the shift
// duration is computed, e.g. "3x40+2x10=140min+15min".
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

// BuildTripTimes returns a slice of (Start, End) time strings for each trip,
// computed using time.Time arithmetic to avoid modular int math.
func BuildTripTimes(hour, minute, trips, tripDuration, breakDuration int) []struct{ Start, End string } {
	out := make([]struct{ Start, End string }, 0, trips)
	cur := time.Date(0, 1, 1, hour, minute, 0, 0, time.UTC)
	for range trips {
		end := cur.Add(time.Duration(tripDuration) * time.Minute)
		out = append(out, struct{ Start, End string }{
			Start: fmt.Sprintf("%02d:%02d", cur.Hour(), cur.Minute()),
			End:   fmt.Sprintf("%02d:%02d", end.Hour(), end.Minute()),
		})
		cur = end.Add(time.Duration(breakDuration) * time.Minute)
	}
	return out
}

// FormatTripSchedule formats a list of (Start, End) trip segments into a
// single human-readable line, e.g. "3 trips: 10:00-11:00, 11:10-12:10 and 12:20-13:20".
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

// formatTimeLine assembles a single schedule line as "{time} - {label}" using
// the "{time} {text}" i18n key. Falls back to a plain space separator when no
// Translator is provided (e.g. in nil-translator unit tests).
func formatTimeLine(timeStr, label string, t Translator) string {
	if t == nil {
		return timeStr + " " + label
	}
	return t.T("{time} {text}", map[string]any{"time": timeStr, "text": label})
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
		lines = append(lines, formatTimeLine(fmt.Sprintf("%02d:%02d", prep.Hour(), prep.Minute()), prepLabel, t))
	}
	cur := base
	for i := 1; i <= trips; i++ {
		tripLabel := fmt.Sprintf("Trip %d", i)
		if t != nil {
			tripLabel = t.T("Trip {n}", map[string]any{"n": i})
		}
		lines = append(lines, formatTimeLine(fmt.Sprintf("%02d:%02d", cur.Hour(), cur.Minute()), tripLabel, t))
		end := cur.Add(time.Duration(tripDuration) * time.Minute)
		if i < trips {
			breakLabel := fmt.Sprintf("Break %d", i)
			if t != nil {
				breakLabel = t.T("Break {n}", map[string]any{"n": i})
			}
			lines = append(lines, formatTimeLine(fmt.Sprintf("%02d:%02d", end.Hour(), end.Minute()), breakLabel, t))
			cur = end.Add(time.Duration(breakDuration) * time.Minute)
		} else {
			cur = end
		}
	}
	if remains > 0 {
		afterEnd := cur.Add(time.Duration(remains) * time.Minute)
		endStr := fmt.Sprintf("%02d:%02d", afterEnd.Hour(), afterEnd.Minute())
		afterLabel := fmt.Sprintf("aftercare \u2192 %s", endStr)
		if t != nil {
			afterLabel = t.T("aftercare \u2192 {time}", map[string]any{"time": endStr})
		}
		lines = append(lines, formatTimeLine(fmt.Sprintf("%02d:%02d", cur.Hour(), cur.Minute()), afterLabel, t))
	}
	return strings.Join(lines, "\n")
}
