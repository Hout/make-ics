package ics_test

import (
	"strings"
	"testing"
	"time"

	pkgics "github.com/jeroen/make-ics-go/pkg/ics"
	"github.com/jeroen/make-ics-go/pkg/pipeline"
)

func TestWriteCalendar_TZID(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}

	events := []pipeline.Event{
		{
			Summary:     "Test HRM",
			Description: "Test description",
			DtStart:     time.Date(2026, 4, 3, 14, 10, 0, 0, loc),
			DtEnd:       time.Date(2026, 4, 3, 16, 0, 0, 0, loc),
			UID:         "test-uid-1234",
		},
	}

	var buf strings.Builder
	if err := pkgics.WriteCalendarWriter(&buf, "Test Cal", events); err != nil {
		t.Fatalf("WriteCalendarWriter: %v", err)
	}

	got := buf.String()

	// DTSTART and DTEND must carry TZID=Europe/Amsterdam
	if !strings.Contains(got, "DTSTART;TZID=Europe/Amsterdam:20260403T141000") {
		t.Errorf("missing DTSTART with TZID=Europe/Amsterdam\ngot:\n%s", got)
	}
	if !strings.Contains(got, "DTEND;TZID=Europe/Amsterdam:20260403T160000") {
		t.Errorf("missing DTEND with TZID=Europe/Amsterdam\ngot:\n%s", got)
	}
	// Must NOT have UTC Z-suffix for DTSTART/DTEND
	if strings.Contains(got, "DTSTART:") {
		t.Errorf("DTSTART must not be bare UTC\ngot:\n%s", got)
	}
	if strings.Contains(got, "DTEND:") {
		t.Errorf("DTEND must not be bare UTC\ngot:\n%s", got)
	}
}
