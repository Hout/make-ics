package pipeline

import (
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/jeroen/make-ics-go/pkg/i18n"
	"github.com/jeroen/make-ics-go/pkg/model"
	"github.com/jeroen/make-ics-go/pkg/parser"
	dr "github.com/jeroen/make-ics-go/pkg/range"
	"github.com/jeroen/make-ics-go/pkg/schedule"
)

type Event struct {
	Label       string
	Summary     string
	Description string
	DtStart     time.Time
	DtEnd       time.Time
	UID         string
}

// IterEvents reads the first sheet of the workbook and returns generated events.
// It applies scheduling rules from shiftTypes, uses date-range overrides,
// and produces localized descriptions via the provided Localizer.
func IterEvents(f *excelize.File, defaultAdvanceMinutes int, timezone string, shiftTypes map[string]model.ShiftType, loc *i18n.Localizer) ([]Event, error) {
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
		if len(r) > 1 && r[1] != "" {
			code = r[1]
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

	// determine first/last per (code,date)
	firstIdx := make(map[string]int)
	lastIdx := make(map[string]int)
	for i, p := range parsed {
		key := fmt.Sprintf("%s|%s", p.Code, p.Date.Format("2006-01-02"))
		if _, ok := firstIdx[key]; !ok {
			firstIdx[key] = i
		}
		lastIdx[key] = i
	}

	locTZ, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, err
	}
	var events []Event
	// default appointment duration is fixed at 240 minutes (4 hours)
	defaultMinutes := 240
	for i, p := range parsed {
		key := fmt.Sprintf("%s|%s", p.Code, p.Date.Format("2006-01-02"))
		isFirst := firstIdx[key] == i
		isLast := lastIdx[key] == i

		// resolve shift type; unknown codes use a zero-value ShiftType (all pointer
		// fields nil), which causes all helpers to return their safe defaults.
		shift, hasShift := shiftTypes[p.Code]
		startTime := fmt.Sprintf("%02d:%02d", p.Hour, p.Min)
		rangeEntry := dr.FindDateRange(shift.DateRanges, p.Date, startTime)

		// advance precedence
		var advance int
		if isFirst {
			if rangeEntry != nil && rangeEntry.FirstAdvance != nil {
				advance = *rangeEntry.FirstAdvance
			} else if hasShift && shift.FirstShiftAdv != nil {
				advance = *shift.FirstShiftAdv
			} else {
				advance = defaultAdvanceMinutes
			}
		} else {
			advance = defaultAdvanceMinutes
		}

		trips := schedule.GetTrips(shift, rangeEntry)
		durationMinutes := schedule.GetShiftDurationMinutes(shift, rangeEntry, trips, defaultMinutes)
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
				description += "\n" + loc.T("- {n}m in advance", map[string]interface{}{"n": advance})
				description += fmt.Sprintf("\n%d %s", *trips, loc.N("trip", *trips, nil))
			}
		} else {
			description += fmt.Sprintf("%02d:%02d ", p.Hour, p.Min)
			description += loc.T("Start", nil)
			description += "\n" + loc.T("- {n}m in advance", map[string]interface{}{"n": advance})
		}

		dtAppt := time.Date(p.Date.Year(), p.Date.Month(), p.Date.Day(), p.Hour, p.Min, 0, 0, locTZ)
		dtStart := dtAppt.Add(-time.Duration(advance) * time.Minute)
		dtEnd := dtAppt.Add(time.Duration(durationMinutes) * time.Minute)

		ev := Event{
			Label:       fmt.Sprintf("%s %02d:%02d  %s", p.Date.Format("2006-01-02"), p.Hour, p.Min, p.Code),
			Summary:     p.Code,
			Description: description,
			DtStart:     dtStart,
			DtEnd:       dtEnd,
			UID:         uuid.NewString(),
		}
		events = append(events, ev)
	}
	return events, nil
}
