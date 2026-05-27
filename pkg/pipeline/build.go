package pipeline

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jeroen/make-ics-go/pkg/i18n"
	"github.com/jeroen/make-ics-go/pkg/schedule"
)

// eventUID is a deterministic UUIDv5 derived from (date, shift code, start
// time). The same shift on the same date always yields the same UID, so
// downstream CalDAV sync can recognise and update an existing event instead
// of creating a duplicate.
func eventUID(date time.Time, code string, hour, min int) string {
	key := fmt.Sprintf("%s|%s|%02d:%02d", date.Format("2006-01-02"), code, hour, min)
	return uuid.NewSHA1(makeICSNamespace, []byte(key)).String()
}

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
			UID:         eventUID(p.Date, p.Code, p.Hour, p.Min),
		})
	}
	return events
}
