package pipeline

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/jeroen/make-ics-go/pkg/i18n"
	"github.com/jeroen/make-ics-go/pkg/model"
	"github.com/jeroen/make-ics-go/pkg/parser"
	dr "github.com/jeroen/make-ics-go/pkg/range"
	"github.com/jeroen/make-ics-go/pkg/schedule"
)

// Event holds all fields needed to write a single VEVENT to an ICS calendar.
type Event struct {
	Summary     string
	Description string
	DtStart     time.Time
	DtEnd       time.Time
	UID         string
}

// IterEvents reads the first sheet of the workbook and returns generated events.
// It applies scheduling rules from shiftTypes, uses schedule/slot overrides,
// and produces localized descriptions via the provided Localizer.
// lines is the LineMap returned by config.LoadConfig and is used to include
// source line numbers in warnings and errors; it may be nil.
func IterEvents(f *excelize.File, defaultAdvanceMinutes int, timezone string, shiftTypes map[string]model.ShiftType, seasons map[string]model.Season, exceptions map[string]model.Exception, lines map[string]int, loc *i18n.Localizer) ([]Event, error) {
	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, err
	}
	// collect candidate rows
	var parsed []struct {
		Code string
		Date time.Time
		Hour int
		Min  int
	}
	for _, r := range rows {
		if !parser.IsDataRow(r) {
			continue
		}
		dateStr := r[0]
		code := "Afspraak"
		if len(r) > 1 && strings.TrimSpace(r[1]) != "" {
			code = strings.TrimSpace(r[1])
		}
		tdate, err := parser.ParseDutchDate(dateStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [SKIP] Could not parse date %q: %v\n", dateStr, err)
			continue
		}
		h, m, err := parser.ParseTime(r[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [SKIP] Could not parse time %q: %v\n", r[2], err)
			continue
		}
		parsed = append(parsed, struct {
			Code string
			Date time.Time
			Hour int
			Min  int
		}{Code: code, Date: tdate, Hour: h, Min: m})
	}

	// determine last/position per (code,date)
	lastIdx := make(map[string]int)
	groupOrder := make(map[string][]int)
	for i, p := range parsed {
		key := fmt.Sprintf("%s|%s", p.Code, p.Date.Format("2006-01-02"))
		lastIdx[key] = i
		groupOrder[key] = append(groupOrder[key], i)
	}
	// positionOf maps each row index to its 0-based position within its (code, date) group.
	positionOf := make(map[int]int, len(parsed))
	for _, indices := range groupOrder {
		for pos, idx := range indices {
			positionOf[idx] = pos
		}
	}

	warnedCrossLevel := make(map[string]bool)

	locTZ, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, err
	}
	const defaultAppointmentMinutes = 240 // 4 h fallback when no trip data is configured
	var events []Event
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
		if rangeEntry != nil && rangeEntry.FirstShiftCount != nil {
			effectiveCount = *rangeEntry.FirstShiftCount
		} else if hasShift && shift.FirstShiftCount != nil {
			effectiveCount = *shift.FirstShiftCount
		}

		// resolve first_shift_time and first_shift_advance independently so
		// cross-level conflicts (one from range, other from shift) can be detected.
		var effectiveFirstTime *string
		var effectiveFirstTimeSrc string
		var effectiveFirstAdvance *int
		var effectiveFirstAdvanceSrc string
		if rangeEntry != nil {
			if rangeEntry.FirstShiftTime != nil {
				effectiveFirstTime = rangeEntry.FirstShiftTime
				effectiveFirstTimeSrc = rangeEntry.FirstShiftTimeSrc
			}
			if rangeEntry.FirstAdvance != nil {
				effectiveFirstAdvance = rangeEntry.FirstAdvance
				effectiveFirstAdvanceSrc = rangeEntry.FirstAdvanceSrc
			}
		}
		if effectiveFirstTime == nil && hasShift && shift.FirstShiftTime != nil {
			effectiveFirstTime = shift.FirstShiftTime
			effectiveFirstTimeSrc = "" // ShiftType level
		}
		if effectiveFirstAdvance == nil && hasShift && shift.FirstShiftAdv != nil {
			effectiveFirstAdvance = shift.FirstShiftAdv
			effectiveFirstAdvanceSrc = "" // ShiftType level
		}
		if effectiveFirstTime != nil && effectiveFirstAdvance != nil && !warnedCrossLevel[p.Code] {
			warnedCrossLevel[p.Code] = true
			timeInfo := lineForShiftField(p.Code, effectiveFirstTimeSrc, "first_shift_time", lines)
			advInfo := lineForShiftField(p.Code, effectiveFirstAdvanceSrc, "first_shift_advance", lines)
			fmt.Fprintf(os.Stderr, "  [WARN] shift %s: first_shift_time%s and first_shift_advance%s set at different levels; first_shift_time prevails\n",
				p.Code, timeInfo, advInfo)
		}

		// advance precedence
		var advance int
		if positionOf[i] < effectiveCount {
			switch {
			case effectiveFirstTime != nil:
				ft, err := time.Parse("15:04", *effectiveFirstTime)
				if err != nil {
					return nil, fmt.Errorf("shift %s: invalid first_shift_time %q: %v", p.Code, *effectiveFirstTime, err)
				}
				firstTimeMinutes := ft.Hour()*60 + ft.Minute()
				departureMinutes := p.Hour*60 + p.Min
				if firstTimeMinutes >= departureMinutes {
					lineInfo := lineForShiftField(p.Code, effectiveFirstTimeSrc, "first_shift_time", lines)
					return nil, fmt.Errorf("shift %s on %s: first_shift_time %q%s is at or after departure %02d:%02d",
						p.Code, p.Date.Format("2006-01-02"), *effectiveFirstTime, lineInfo, p.Hour, p.Min)
				}
				advance = departureMinutes - firstTimeMinutes
			case effectiveFirstAdvance != nil:
				advance = *effectiveFirstAdvance
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
			remains = schedule.GetLastShiftRemains(shift, rangeEntry)
		}
		durationMinutes += remains

		description := ""
		if hasShift && shift.Description != "" {
			description = shift.Description + "\n"
		}
		if trips != nil {
			// determine trip_duration and break_duration (merged)
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
			if tripDurVal != nil {
				prog := schedule.BuildProgram(p.Hour, p.Min, advance, *trips, *tripDurVal, breakDurVal, remains, loc)
				description += prog
			} else {
				description += fmt.Sprintf("%02d:%02d ", p.Hour, p.Min)
				description += loc.T("Start", nil)
				description += "\n" + loc.T("- {n}m in advance", map[string]any{"n": advance})
				description += fmt.Sprintf("\n%d %s", *trips, loc.N("trip", *trips, nil))
			}
		} else {
			description += fmt.Sprintf("%02d:%02d ", p.Hour, p.Min)
			description += loc.T("Start", nil)
			description += "\n" + loc.T("- {n}m in advance", map[string]any{"n": advance})
		}

		dtAppt := time.Date(p.Date.Year(), p.Date.Month(), p.Date.Day(), p.Hour, p.Min, 0, 0, locTZ)
		dtStart := dtAppt.Add(-time.Duration(advance) * time.Minute)
		dtEnd := dtAppt.Add(time.Duration(durationMinutes) * time.Minute)

		summary := p.Code
		if hasShift && shift.Summary != "" {
			summary = shift.Summary
		}
		ev := Event{
			Summary:     summary,
			Description: description,
			DtStart:     dtStart,
			DtEnd:       dtEnd,
			UID:         uuid.NewString(),
		}
		events = append(events, ev)
	}
	return events, nil
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
