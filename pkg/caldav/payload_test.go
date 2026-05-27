package caldav

import (
	"bytes"
	"testing"
	"time"

	ical "github.com/emersion/go-ical"

	"github.com/jeroen/make-ics-go/pkg/pipeline"
)

var tz, _ = time.LoadLocation("Europe/Amsterdam")

func testEvent(uid, summary, desc string, dtstart, dtend time.Time) pipeline.Event {
	return pipeline.Event{
		UID:         uid,
		Summary:     summary,
		Description: desc,
		DtStart:     dtstart,
		DtEnd:       dtend,
	}
}

func TestEventHash_Stable(t *testing.T) {
	e := testEvent("uid-1", "Shift A", "desc", time.Date(2026, 6, 1, 9, 0, 0, 0, tz), time.Date(2026, 6, 1, 13, 0, 0, 0, tz))
	if eventHash(e) == "" {
		t.Fatal("hash is empty")
	}
	if eventHash(e) != eventHash(e) {
		t.Fatal("hash is not stable across calls")
	}
}

func TestEventHash_DifferentUID(t *testing.T) {
	base := testEvent("uid-1", "Shift A", "desc", time.Date(2026, 6, 1, 9, 0, 0, 0, tz), time.Date(2026, 6, 1, 13, 0, 0, 0, tz))
	other := base
	other.UID = "uid-2"
	if eventHash(base) == eventHash(other) {
		t.Error("different UIDs produced the same hash")
	}
}

func TestEventHash_DifferentTime(t *testing.T) {
	base := testEvent("uid-1", "Shift A", "desc", time.Date(2026, 6, 1, 9, 0, 0, 0, tz), time.Date(2026, 6, 1, 13, 0, 0, 0, tz))
	other := base
	other.DtStart = base.DtStart.Add(30 * time.Minute)
	if eventHash(base) == eventHash(other) {
		t.Error("different DtStart produced the same hash")
	}
}

func TestEventHash_DTSTAMP_Excluded(t *testing.T) {
	// Hashing is independent of when buildCalObject is called (DTSTAMP varies).
	e := testEvent("uid-1", "Shift A", "desc", time.Date(2026, 6, 1, 9, 0, 0, 0, tz), time.Date(2026, 6, 1, 13, 0, 0, 0, tz))
	h1 := eventHash(e)
	h2 := eventHash(e)
	if h1 != h2 {
		t.Error("hash changed between calls (DTSTAMP must not be included)")
	}
}

func TestObjectHash_RoundTrip(t *testing.T) {
	e := testEvent("uid-1", "Shift A", "desc", time.Date(2026, 6, 1, 9, 0, 0, 0, tz), time.Date(2026, 6, 1, 13, 0, 0, 0, tz))
	cal := buildCalObject(e)

	// Round-trip through ICS encoding and decoding to simulate server storage.
	var buf bytes.Buffer
	if err := ical.NewEncoder(&buf).Encode(cal); err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := ical.NewDecoder(&buf).Decode()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if objectHash(decoded) != eventHash(e) {
		t.Errorf("objectHash after round-trip %q != eventHash %q", objectHash(decoded), eventHash(e))
	}
}

func TestIsManaged_True(t *testing.T) {
	e := testEvent("uid-1", "S", "d", time.Now(), time.Now().Add(time.Hour))
	cal := buildCalObject(e)
	if !isManaged(cal) {
		t.Error("expected buildCalObject result to be managed")
	}
}

func TestIsManaged_False(t *testing.T) {
	cal := ical.NewCalendar()
	ev := ical.NewEvent()
	ev.Props.SetText(ical.PropUID, "other-uid")
	cal.Children = append(cal.Children, ev.Component)
	if isManaged(cal) {
		t.Error("user-created event should not be managed")
	}
}

func TestExtractUID(t *testing.T) {
	e := testEvent("my-uid", "S", "d", time.Now(), time.Now().Add(time.Hour))
	cal := buildCalObject(e)
	if uid := extractUID(cal); uid != "my-uid" {
		t.Errorf("extractUID = %q, want %q", uid, "my-uid")
	}
}

func TestExtractUID_Empty(t *testing.T) {
	cal := ical.NewCalendar()
	if uid := extractUID(cal); uid != "" {
		t.Errorf("extractUID on empty calendar = %q, want empty", uid)
	}
}
