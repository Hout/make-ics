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

	cases := []struct {
		name          string
		start         time.Time
		end           time.Time
		wantDtStart   string // expected literal value in ICS (local time, no UTC conversion)
		wantDtEnd     string
		wantUTCOffset string // DST offset for documentation only
	}{
		{
			// Summer: Amsterdam is UTC+2; 14:10 local stays 14:10, NOT 12:10Z
			name:          "summer DST UTC+2",
			start:         time.Date(2026, 4, 3, 14, 10, 0, 0, loc),
			end:           time.Date(2026, 4, 3, 16, 0, 0, 0, loc),
			wantDtStart:   "DTSTART;TZID=Europe/Amsterdam:20260403T141000",
			wantDtEnd:     "DTEND;TZID=Europe/Amsterdam:20260403T160000",
			wantUTCOffset: "+02:00",
		},
		{
			// Winter: Amsterdam is UTC+1; 14:10 local stays 14:10, NOT 13:10Z
			name:          "winter DST UTC+1",
			start:         time.Date(2026, 12, 1, 14, 10, 0, 0, loc),
			end:           time.Date(2026, 12, 1, 16, 0, 0, 0, loc),
			wantDtStart:   "DTSTART;TZID=Europe/Amsterdam:20261201T141000",
			wantDtEnd:     "DTEND;TZID=Europe/Amsterdam:20261201T160000",
			wantUTCOffset: "+01:00",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			events := []pipeline.Event{
				{
					Summary:     "Test",
					Description: "Test",
					DtStart:     tc.start,
					DtEnd:       tc.end,
					UID:         "test-uid",
				},
			}
			var buf strings.Builder
			if err := pkgics.WriteCalendarWriter(&buf, "Test Cal", events); err != nil {
				t.Fatalf("WriteCalendarWriter: %v", err)
			}
			got := buf.String()

			// Local time must be preserved as-is — not converted to UTC
			if !strings.Contains(got, tc.wantDtStart) {
				t.Errorf("missing %s\ngot:\n%s", tc.wantDtStart, got)
			}
			if !strings.Contains(got, tc.wantDtEnd) {
				t.Errorf("missing %s\ngot:\n%s", tc.wantDtEnd, got)
			}
			// Must NOT have bare UTC timestamps
			if strings.Contains(got, "DTSTART:") {
				t.Errorf("DTSTART must not be bare UTC\ngot:\n%s", got)
			}
			if strings.Contains(got, "DTEND:") {
				t.Errorf("DTEND must not be bare UTC\ngot:\n%s", got)
			}
		})
	}
}
