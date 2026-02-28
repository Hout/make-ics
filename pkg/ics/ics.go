package ics

import (
	"os"

	ical "github.com/arran4/golang-ical"
	"github.com/jeroen/make-ics-go/pkg/pipeline"
)

func WriteCalendar(name string, events []pipeline.Event, path string) error {
	cal := ical.NewCalendar()
	cal.SetMethod(ical.MethodPublish)
	cal.SetProductId("-//make-ics//go//NL")
	cal.SetXWRCalName(name)
	for _, e := range events {
		ev := cal.AddEvent(e.UID)
		ev.SetSummary(e.Summary)
		ev.SetDescription(e.Description)
		ev.SetStartAt(e.DtStart)
		ev.SetEndAt(e.DtEnd)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(cal.Serialize())
	return err
}
