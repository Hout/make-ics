package pipeline

import (
	"fmt"
	"os"
	"time"

	"github.com/jeroen/make-ics-go/pkg/model"
	dr "github.com/jeroen/make-ics-go/pkg/range"
	"github.com/jeroen/make-ics-go/pkg/schedule"
)

const defaultAppointmentMinutes = 240 // 4 h fallback when no trip data is configured

type resolvedRow struct {
	parsed          parsedRow
	advance         int  // minutes before departure
	durationMinutes int  // total duration including aftercare
	remains         int  // aftercare minutes (also included in durationMinutes; needed by BuildProgram)
	trips           *int // nil when not configured
	tripDurVal      *int // nil when not configured
	breakDurVal     int
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
) ([]resolvedRow, error) {
	lastIdx := make(map[string]int)
	groupOrder := make(map[string][]int)
	for i, p := range parsed {
		key := fmt.Sprintf("%s|%s", p.Code, p.Date.Format("2006-01-02"))
		lastIdx[key] = i
		groupOrder[key] = append(groupOrder[key], i)
	}
	positionOf := make(map[int]int, len(parsed))
	for _, indices := range groupOrder {
		for pos, idx := range indices {
			positionOf[idx] = pos
		}
	}

	resolved := make([]resolvedRow, 0, len(parsed))
	for i, p := range parsed {
		key := fmt.Sprintf("%s|%s", p.Code, p.Date.Format("2006-01-02"))
		isLast := lastIdx[key] == i

		// resolve shift type; unknown codes use a zero-value ShiftType (all pointer
		// fields nil), which causes all helpers to return their safe defaults.
		shift, hasShift := shiftTypes[p.Code]
		startTime := fmt.Sprintf("%02d:%02d", p.Hour, p.Min)
		eff := dr.EffectiveWeekday(p.Date, exceptions)
		rangeEntry := dr.FindSchedule(shift.Schedules, p.Date, startTime, eff, seasons)

		// resolve effective first-shift count (how many leading shifts get the advance)
		effectiveCount := 1
		if rangeEntry != nil && rangeEntry.FirstShiftPreparationCount != nil {
			effectiveCount = *rangeEntry.FirstShiftPreparationCount
		} else if hasShift && shift.FirstShiftPreparationCount != nil {
			effectiveCount = *shift.FirstShiftPreparationCount
		}

		// resolve first_shift_preparation_time and first_shift_preparation_duration independently
		// so cross-level conflicts (one from range, other from shift) can be detected.
		var effectiveFirstPrepTime *string
		var effectiveFirstPrepTimeSrc string
		var effectiveFirstPrepDuration *int
		var effectiveFirstPrepDurationSrc string
		if rangeEntry != nil {
			if rangeEntry.FirstShiftPreparationTime != nil {
				effectiveFirstPrepTime = rangeEntry.FirstShiftPreparationTime
				effectiveFirstPrepTimeSrc = rangeEntry.FirstShiftPreparationTimeSrc
			}
			if rangeEntry.FirstShiftPreparationDuration != nil {
				effectiveFirstPrepDuration = rangeEntry.FirstShiftPreparationDuration
				effectiveFirstPrepDurationSrc = rangeEntry.FirstShiftPreparationDurationSrc
			}
		}
		if effectiveFirstPrepTime == nil && hasShift && shift.FirstShiftPreparationTime != nil {
			effectiveFirstPrepTime = shift.FirstShiftPreparationTime
			effectiveFirstPrepTimeSrc = "" // ShiftType level
		}
		if effectiveFirstPrepDuration == nil && hasShift && shift.FirstShiftPreparationDuration != nil {
			effectiveFirstPrepDuration = shift.FirstShiftPreparationDuration
			effectiveFirstPrepDurationSrc = "" // ShiftType level
		}
		if effectiveFirstPrepTime != nil && effectiveFirstPrepDuration != nil && !warnedCrossLevel[p.Code] {
			warnedCrossLevel[p.Code] = true
			timeInfo := lineForShiftField(p.Code, effectiveFirstPrepTimeSrc, "first_shift_preparation_time", lines)
			advInfo := lineForShiftField(p.Code, effectiveFirstPrepDurationSrc, "first_shift_preparation_duration", lines)
			fmt.Fprintf(os.Stderr, "  [WARN] shift %s: first_shift_preparation_time%s and first_shift_preparation_duration%s set at different levels; first_shift_preparation_time prevails\n",
				p.Code, timeInfo, advInfo)
		}

		// Determine whether this departure is among the first effectiveCount
		// scheduled times for this slot. FirstScheduledTimes looks up the slot from
		// config and returns the chronologically earliest N times; if no start_times
		// are defined we fall back to positional order within the xlsx rows.
		firstTimes := dr.FirstScheduledTimes(shift.Schedules, p.Date, eff, seasons, effectiveCount)
		var isFirstShift bool
		if firstTimes != nil {
			isFirstShift = firstTimes[startTime]
		} else {
			isFirstShift = positionOf[i] < effectiveCount
		}

		var advance int
		if isFirstShift {
			switch {
			case effectiveFirstPrepTime != nil:
				ft, err := time.Parse("15:04", *effectiveFirstPrepTime)
				if err != nil {
					return nil, fmt.Errorf("shift %s: invalid first_shift_preparation_time %q: %v", p.Code, *effectiveFirstPrepTime, err)
				}
				firstTimeMinutes := ft.Hour()*60 + ft.Minute()
				departureMinutes := p.Hour*60 + p.Min
				if firstTimeMinutes >= departureMinutes {
					lineInfo := lineForShiftField(p.Code, effectiveFirstPrepTimeSrc, "first_shift_preparation_time", lines)
					return nil, fmt.Errorf("shift %s on %s: first_shift_preparation_time %q%s is at or after departure %02d:%02d",
						p.Code, p.Date.Format("2006-01-02"), *effectiveFirstPrepTime, lineInfo, p.Hour, p.Min)
				}
				advance = departureMinutes - firstTimeMinutes
			case effectiveFirstPrepDuration != nil:
				advance = *effectiveFirstPrepDuration
			default:
				advance = defaultAdvanceMinutes
			}
		} else {
			advance = defaultAdvanceMinutes
		}

		trips := schedule.GetTrips(shift, rangeEntry)
		durationMinutes := schedule.GetShiftDurationMinutes(shift, rangeEntry, trips, defaultAppointmentMinutes)
		remains := 0
		if isLast {
			remains = schedule.GetLastShiftAftercare(shift, rangeEntry)
		}
		durationMinutes += remains

		var tripDurVal *int
		var breakDurVal int
		if rangeEntry != nil && rangeEntry.TripDuration != nil {
			tripDurVal = rangeEntry.TripDuration
		} else if shift.TripDuration != nil {
			tripDurVal = shift.TripDuration
		}
		if rangeEntry != nil && rangeEntry.BreakDuration != nil {
			breakDurVal = *rangeEntry.BreakDuration
		} else if shift.BreakDuration != nil {
			breakDurVal = *shift.BreakDuration
		}

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
			trips:           trips,
			tripDurVal:      tripDurVal,
			breakDurVal:     breakDurVal,
			summary:         summary,
			description:     descriptionPrefix,
		})
	}
	return resolved, nil
}

// lineForShiftField returns " (line N)" when lines contains the YAML path for
// field within the given shift code and source path, otherwise returns "".
// srcPath is the relative path within the ShiftType (e.g. "schedules[0].slots[1]");
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
