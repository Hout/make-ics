package pipeline

import (
	"fmt"
	"time"

	"github.com/jeroen/make-ics-go/pkg/model"
	dr "github.com/jeroen/make-ics-go/pkg/range"
	"github.com/jeroen/make-ics-go/pkg/schedule"
)

const defaultAppointmentMinutes = 240 // 4 h fallback when no trip data is configured

type resolvedRow struct {
	parsed          parsedRow
	advance         int // minutes before departure
	durationMinutes int // total duration
	remains         int // aftercare minutes after last trip end (used by BuildProgramFromTripTimes)
	tripTimes       []model.TripTime
	trips           *int // nil when not configured
	summary         string
	description     string // shift.Description prefix (may be empty)
}

func resolveRows(
	parsed []parsedRow,
	defaultAdvanceMinutes int,
	shiftTypes map[string]model.ShiftType,
	seasons map[string]model.Season,
	exceptions map[string]model.Exception,
	lines map[string]int,
	warnedCrossLevel map[string]bool,
	warnings *[]string,
) ([]resolvedRow, error) {
	resolved := make([]resolvedRow, 0, len(parsed))
	for _, p := range parsed {
		shift, hasShift := shiftTypes[p.Code]
		startTime := fmt.Sprintf("%02d:%02d", p.Hour, p.Min)
		eff := dr.EffectiveWeekday(p.Date, exceptions)
		rangeEntry := dr.FindSchedule(shift.Schedules, p.Date, startTime, eff, seasons)

		tripTimes := rangeEntryTripTimes(rangeEntry)
		departureMinutes := p.Hour*60 + p.Min

		// Compute arrive time
		var arriveMinutes int
		if rangeEntry != nil && rangeEntry.Arrive != nil {
			at, err := time.Parse("15:04", *rangeEntry.Arrive)
			if err != nil {
				return nil, fmt.Errorf("shift %s on %s: invalid arrive time %q: %v", p.Code, p.Date.Format("2006-01-02"), *rangeEntry.Arrive, err)
			}
			arriveMinutes = at.Hour()*60 + at.Minute()
			if arriveMinutes >= departureMinutes {
				return nil, fmt.Errorf("shift %s on %s: arrive time %q is at or after departure %02d:%02d", p.Code, p.Date.Format("2006-01-02"), *rangeEntry.Arrive, p.Hour, p.Min)
			}
		} else {
			arriveMinutes = departureMinutes - defaultAdvanceMinutes
		}
		advance := departureMinutes - arriveMinutes

		// Compute leave time
		var leaveMinutes int
		if rangeEntry != nil && rangeEntry.Leave != nil {
			lt, err := time.Parse("15:04", *rangeEntry.Leave)
			if err != nil {
				return nil, fmt.Errorf("shift %s on %s: invalid leave time %q: %v", p.Code, p.Date.Format("2006-01-02"), *rangeEntry.Leave, err)
			}
			leaveMinutes = lt.Hour()*60 + lt.Minute()
		} else if len(tripTimes) > 0 {
			if len(tripTimes) == 1 {
				return nil, fmt.Errorf("shift %s on %s: single-trip shift requires explicit leave time", p.Code, p.Date.Format("2006-01-02"))
			}
			// Multi-trip without explicit leave: use last trip end
			lastTrip := tripTimes[len(tripTimes)-1]
			lt, err := time.Parse("15:04", lastTrip.Start)
			if err != nil {
				return nil, fmt.Errorf("shift %s: invalid last trip start %q: %v", p.Code, lastTrip.Start, err)
			}
			if lastTrip.Duration <= 0 {
				return nil, fmt.Errorf("shift %s on %s: last trip has no duration, cannot auto-compute leave", p.Code, p.Date.Format("2006-01-02"))
			}
			leaveMinutes = lt.Hour()*60 + lt.Minute() + lastTrip.Duration
		} else {
			leaveMinutes = departureMinutes + defaultAppointmentMinutes
		}
		durationMinutes := leaveMinutes - departureMinutes

		// Compute remains: aftercare minutes after last trip end (for description)
		var remains int
		if len(tripTimes) > 0 {
			lastTrip := tripTimes[len(tripTimes)-1]
			lt, err := time.Parse("15:04", lastTrip.Start)
			if err == nil {
				lastTripEndMinutes := lt.Hour()*60 + lt.Minute()
				if lastTrip.Duration > 0 {
					lastTripEndMinutes += lastTrip.Duration
				}
				remains = leaveMinutes - lastTripEndMinutes
				if remains < 0 {
					remains = 0
				}
			}
		}

		trips := schedule.GetTrips(shift, rangeEntry)

		summary := p.Code
		if hasShift && shift.Summary != "" {
			summary = shift.Summary
		}
		descriptionPrefix := ""
		if hasShift && shift.Description != "" {
			descriptionPrefix = shift.Description + "\n"
		}

		resolved = append(resolved, resolvedRow{
			parsed:          p,
			advance:         advance,
			durationMinutes: durationMinutes,
			remains:         remains,
			tripTimes:       tripTimes,
			trips:           trips,
			summary:         summary,
			description:     descriptionPrefix,
		})
	}
	return resolved, nil
}

func rangeEntryTripTimes(rangeEntry *dr.ResolvedRange) []model.TripTime {
	if rangeEntry == nil || len(rangeEntry.TripTimes) == 0 {
		return nil
	}
	out := make([]model.TripTime, len(rangeEntry.TripTimes))
	copy(out, rangeEntry.TripTimes)
	return out
}

// lineForShiftField returns " (line N)" when lines contains the YAML path for
// field within the given shift code and source path, otherwise returns "".
// srcPath is the relative path within the ShiftType (e.g. "season_schedules[0].day_schedules[1]");
// an empty srcPath means the field is at ShiftType level.
func lineForShiftField(code, srcPath, field string, lines map[string]int) string {
	if lines == nil {
		return ""
	}
	var fullPath string
	if srcPath == "" {
		fullPath = "shift_type." + code + "." + field
	} else {
		fullPath = "shift_type." + code + "." + srcPath + "." + field
	}
	if n, ok := lines[fullPath]; ok {
		return fmt.Sprintf(" (line %d)", n)
	}
	return ""
}
