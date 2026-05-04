package pipeline

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jeroen/make-ics-go/pkg/i18n"
	"github.com/jeroen/make-ics-go/pkg/schedule"
)

func buildEvents(resolved []resolvedRow, locTZ *time.Location, loc *i18n.Localizer) []Event {
	events := make([]Event, 0, len(resolved))
	for _, r := range resolved {
		p := r.parsed
		description := r.description
		if len(r.tripTimes) > 0 {
			prog := schedule.BuildProgramFromTripTimes(r.tripTimes, r.advance, r.remains, loc)
			description += prog
		} else if r.trips != nil {
			description += fmt.Sprintf("%02d:%02d ", p.Hour, p.Min)
			description += loc.T("Start", nil)
			description += "\n" + loc.T("- {n}m in advance", map[string]any{"n": r.advance})
			description += fmt.Sprintf("\n%d %s", *r.trips, loc.N("trip", *r.trips, nil))
		} else {
			description += fmt.Sprintf("%02d:%02d ", p.Hour, p.Min)
			description += loc.T("Start", nil)
			description += "\n" + loc.T("- {n}m in advance", map[string]any{"n": r.advance})
		}

		dtAppt := time.Date(p.Date.Year(), p.Date.Month(), p.Date.Day(), p.Hour, p.Min, 0, 0, locTZ)
		dtStart := dtAppt.Add(-time.Duration(r.advance) * time.Minute)
		dtEnd := dtAppt.Add(time.Duration(r.durationMinutes) * time.Minute)

		events = append(events, Event{
			Summary:     r.summary,
			Description: description,
			DtStart:     dtStart,
			DtEnd:       dtEnd,
			UID:         uuid.NewString(),
		})
	}
	return events
}
