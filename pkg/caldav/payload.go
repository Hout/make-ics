package caldav

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	ical "github.com/emersion/go-ical"

	"github.com/jeroen/make-ics-go/pkg/pipeline"
)

const managedProp = "X-MAKE-ICS-MANAGED"

// buildCalObject builds a single-VEVENT VCALENDAR for upload via CalDAV PUT.
// X-MAKE-ICS-MANAGED:1 marks it as owned by this tool so Sync can distinguish
// our events from user-created ones.
func buildCalObject(e pipeline.Event) *ical.Calendar {
	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropProductID, "-//make-ics//go//NL")
	cal.Props.SetText(ical.PropVersion, "2.0")

	ev := ical.NewEvent()
	ev.Props.SetText(ical.PropUID, e.UID)
	ev.Props.SetText(ical.PropSummary, e.Summary)
	ev.Props.SetText(ical.PropDescription, e.Description)
	ev.Props.SetDateTime(ical.PropDateTimeStart, e.DtStart)
	ev.Props.SetDateTime(ical.PropDateTimeEnd, e.DtEnd)
	ev.Props.SetDateTime(ical.PropDateTimeStamp, time.Now().UTC())

	managed := ical.NewProp(managedProp)
	managed.Value = "1"
	ev.Props.Set(managed)

	cal.Children = append(cal.Children, ev.Component)
	return cal
}

// eventHash returns a stable content hash for e. DTSTAMP is intentionally
// excluded so re-uploading an unchanged event does not look like an update.
func eventHash(e pipeline.Event) string {
	return hashFields(e.UID, e.Summary, e.Description, e.DtStart, e.DtEnd)
}

// objectHash computes the same hash for a remote calendar object so it can
// be compared against eventHash for change detection.
func objectHash(cal *ical.Calendar) string {
	events := cal.Events()
	if len(events) == 0 {
		return ""
	}
	ev := events[0]
	uid, _ := ev.Props.Text(ical.PropUID)
	summary, _ := ev.Props.Text(ical.PropSummary)
	desc, _ := ev.Props.Text(ical.PropDescription)
	dtstart, _ := ev.Props.DateTime(ical.PropDateTimeStart, time.UTC)
	dtend, _ := ev.Props.DateTime(ical.PropDateTimeEnd, time.UTC)
	return hashFields(uid, summary, desc, dtstart, dtend)
}

func hashFields(uid, summary, desc string, dtstart, dtend time.Time) string {
	key := strings.Join([]string{
		uid,
		summary,
		desc,
		dtstart.UTC().Format(time.RFC3339),
		dtend.UTC().Format(time.RFC3339),
	}, "\x00")
	sum := sha256.Sum256([]byte(key))
	return fmt.Sprintf("%x", sum)
}

// isManaged reports whether cal contains a VEVENT with X-MAKE-ICS-MANAGED:1.
func isManaged(cal *ical.Calendar) bool {
	for _, ev := range cal.Events() {
		if p := ev.Props.Get(managedProp); p != nil && p.Value == "1" {
			return true
		}
	}
	return false
}

// extractUID returns the UID of the first VEVENT in cal, or "" when absent.
func extractUID(cal *ical.Calendar) string {
	events := cal.Events()
	if len(events) == 0 {
		return ""
	}
	uid, _ := events[0].Props.Text(ical.PropUID)
	return uid
}
