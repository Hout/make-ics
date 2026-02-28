package ics

import (
	"io"
	"os"

	ical "github.com/arran4/golang-ical"
	"github.com/jeroen/make-ics-go/pkg/pipeline"
)

// WriteCalendarWriter serialises events to w, setting name as the calendar
// display name (X-WR-CALNAME). DTSTART and DTEND carry a TZID parameter
// derived from each event's location, e.g. DTSTART;TZID=Europe/Amsterdam:...
func WriteCalendarWriter(w io.Writer, name string, events []pipeline.Event) error {
	cal := ical.NewCalendar()
	cal.SetMethod(ical.MethodPublish)
	cal.SetProductId("-//make-ics//go//NL")
	cal.SetXWRCalName(name)
	for _, e := range events {
		ev := cal.AddEvent(e.UID)
		ev.SetSummary(e.Summary)
		ev.SetDescription(e.Description)
		tzid := e.DtStart.Location().String()
		ev.AddProperty(
			ical.ComponentPropertyDtStart,
			e.DtStart.Format("20060102T150405"),
			ical.WithTZID(tzid),
		)
		ev.AddProperty(
			ical.ComponentPropertyDtEnd,
			e.DtEnd.Format("20060102T150405"),
			ical.WithTZID(tzid),
		)
	}
	_, err := io.WriteString(w, cal.Serialize())
	return err
}

// WriteCalendar serialises events to an ICS file at path. It delegates to
// WriteCalendarWriter so DTSTART/DTEND always include a TZID parameter.
func WriteCalendar(name string, events []pipeline.Event, path string) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()
	return WriteCalendarWriter(f, name, events)
}
