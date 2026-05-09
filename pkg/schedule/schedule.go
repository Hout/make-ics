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
	if rangeEntry != nil && len(rangeEntry.TripTimes) > 0 {
		n := len(rangeEntry.TripTimes)
		return &n
	}
	if rangeEntry != nil && rangeEntry.Trips != nil {
		return rangeEntry.Trips
	}
	if shift.Trips != nil {
		return shift.Trips
	}
	return nil
}

// BuildProgramFromTripTimes returns a multi-line program using explicit trip windows.
func BuildProgramFromTripTimes(tripTimes []model.TripTime, advance int, remains int, t Translator) string {
	if len(tripTimes) == 0 {
		return ""
	}
	firstStart, err := time.Parse("15:04", tripTimes[0].Start)
	if err != nil {
		return ""
	}
	base := time.Date(0, 1, 1, firstStart.Hour(), firstStart.Minute(), 0, 0, time.UTC)

	var lines []string
	if advance > 0 {
		prep := base.Add(-time.Duration(advance) * time.Minute)
		prepLabel := "Preparation"
		if t != nil {
			prepLabel = t.T("Preparation", nil)
		}
		lines = append(lines, formatTimeLine(fmt.Sprintf("%02d:%02d", prep.Hour(), prep.Minute()), prepLabel, t))
	}

	for i, trip := range tripTimes {
		start, errStart := time.Parse("15:04", trip.Start)
		if errStart != nil {
			return ""
		}
		hasDuration := trip.Duration > 0
		end := time.Date(0, 1, 1, start.Hour(), start.Minute(), 0, 0, time.UTC)
		if hasDuration {
			end = end.Add(time.Duration(trip.Duration) * time.Minute)
		}
		tripLabel := fmt.Sprintf("Trip %d", i+1)
		if t != nil {
			tripLabel = t.T("Trip {n}", map[string]any{"n": i + 1})
		}
		lines = append(lines, formatTimeLine(fmt.Sprintf("%02d:%02d", start.Hour(), start.Minute()), tripLabel, t))

		if i < len(tripTimes)-1 && hasDuration {
			nextStart, err := time.Parse("15:04", tripTimes[i+1].Start)
			if err != nil {
				return ""
			}
			if nextStart.After(end) {
				breakLabel := fmt.Sprintf("Break %d", i+1)
				if t != nil {
					breakLabel = t.T("Break {n}", map[string]any{"n": i + 1})
				}
				lines = append(lines, formatTimeLine(fmt.Sprintf("%02d:%02d", end.Hour(), end.Minute()), breakLabel, t))
			}
		}
	}

	if remains > 0 {
		lastTrip := tripTimes[len(tripTimes)-1]
		lastStart, err := time.Parse("15:04", lastTrip.Start)
		if err != nil {
			return ""
		}
		afterBase := time.Date(0, 1, 1, lastStart.Hour(), lastStart.Minute(), 0, 0, time.UTC)
		if lastTrip.Duration > 0 {
			afterBase = afterBase.Add(time.Duration(lastTrip.Duration) * time.Minute)
		}
		afterEnd := afterBase.Add(time.Duration(remains) * time.Minute)
		endStr := fmt.Sprintf("%02d:%02d", afterEnd.Hour(), afterEnd.Minute())
		afterLabel := fmt.Sprintf("aftercare → %s", endStr)
		if t != nil {
			afterLabel = t.T("aftercare → {time}", map[string]any{"time": endStr})
		}
		lines = append(lines, afterLabel)
	}

	return strings.Join(lines, "\n")
}

// FormatExplicitTripSchedule formats explicit trip starts+durations into one line.
func FormatExplicitTripSchedule(tripTimes []model.TripTime, t Translator) string {
	n := len(tripTimes)
	segments := make([]string, n)
	for i, trip := range tripTimes {
		start, err := time.Parse("15:04", trip.Start)
		if err != nil || trip.Duration <= 0 {
			continue
		}
		end := time.Date(0, 1, 1, start.Hour(), start.Minute(), 0, 0, time.UTC).Add(time.Duration(trip.Duration) * time.Minute)
		segments[i] = fmt.Sprintf("%s-%02d:%02d", trip.Start, end.Hour(), end.Minute())
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
