package pipeline

import (
	"bytes"
	"testing"
	"time"

	"github.com/jeroen/make-ics-go/pkg/i18n"
)

func makeResolved(advance, durationMinutes int, trips, tripDurVal *int, breakDurVal, remains int) resolvedRow {
	d := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)
	return resolvedRow{
		parsed:          parsedRow{Code: "A", Date: d, Hour: 10, Min: 0},
		advance:         advance,
		durationMinutes: durationMinutes,
		remains:         remains,
		trips:           trips,
		tripDurVal:      tripDurVal,
		breakDurVal:     breakDurVal,
		summary:         "A",
	}
}

func mustLocalizer(lang string) *i18n.Localizer {
	l, _ := i18n.NewLocalizer(lang)
	return l
}

func TestBuildEvents_DtStartDtEnd(t *testing.T) {
	loc := mustLocalizer("en")
	tz, _ := time.LoadLocation("Europe/Amsterdam")
	r := makeResolved(30, 120, nil, nil, 0, 0)

	events := buildEvents([]resolvedRow{r}, tz, loc)
	if len(events) != 1 {
		t.Fatalf("expected 1 event got %d", len(events))
	}
	e := events[0]
	appt := time.Date(2026, 4, 3, 10, 0, 0, 0, tz)
	wantStart := appt.Add(-30 * time.Minute)
	wantEnd := appt.Add(120 * time.Minute)
	if !e.DtStart.Equal(wantStart) {
		t.Errorf("DtStart want %v got %v", wantStart, e.DtStart)
	}
	if !e.DtEnd.Equal(wantEnd) {
		t.Errorf("DtEnd want %v got %v", wantEnd, e.DtEnd)
	}
}

func TestBuildEvents_SummaryFromResolved(t *testing.T) {
	loc := mustLocalizer("en")
	tz, _ := time.LoadLocation("Europe/Amsterdam")
	r := makeResolved(10, 60, nil, nil, 0, 0)
	r.summary = "My Shift"

	events := buildEvents([]resolvedRow{r}, tz, loc)
	if events[0].Summary != "My Shift" {
		t.Errorf("summary want %q got %q", "My Shift", events[0].Summary)
	}
}

func TestBuildEvents_DescriptionPrefixIncluded(t *testing.T) {
	loc := mustLocalizer("en")
	tz, _ := time.LoadLocation("Europe/Amsterdam")
	r := makeResolved(10, 60, nil, nil, 0, 0)
	r.description = "Route detail\n"

	events := buildEvents([]resolvedRow{r}, tz, loc)
	if !bytes.Contains([]byte(events[0].Description), []byte("Route detail")) {
		t.Errorf("expected description to contain prefix, got %q", events[0].Description)
	}
}

func TestBuildEvents_NilTrips_SimpleFormat(t *testing.T) {
	loc := mustLocalizer("en")
	tz, _ := time.LoadLocation("Europe/Amsterdam")
	r := makeResolved(30, 240, nil, nil, 0, 0)

	events := buildEvents([]resolvedRow{r}, tz, loc)
	desc := events[0].Description
	if !bytes.Contains([]byte(desc), []byte("10:00")) {
		t.Errorf("expected departure time in description, got %q", desc)
	}
}

func TestBuildEvents_WithTripsAndTripDur_BuildProgram(t *testing.T) {
	loc := mustLocalizer("en")
	tz, _ := time.LoadLocation("Europe/Amsterdam")
	trips := 2
	tripDur := 50
	r := makeResolved(30, 130, &trips, &tripDur, 10, 0)

	events := buildEvents([]resolvedRow{r}, tz, loc)
	desc := events[0].Description
	// BuildProgram includes "Trip" lines
	if !bytes.Contains([]byte(desc), []byte("Trip")) {
		t.Errorf("expected BuildProgram output in description, got %q", desc)
	}
}

func TestBuildEvents_TripsNoTripDur_FallbackFormat(t *testing.T) {
	loc := mustLocalizer("en")
	tz, _ := time.LoadLocation("Europe/Amsterdam")
	trips := 3
	r := makeResolved(30, 240, &trips, nil, 0, 0)

	events := buildEvents([]resolvedRow{r}, tz, loc)
	desc := events[0].Description
	// fallback format shows trip count but no BuildProgram program lines
	if !bytes.Contains([]byte(desc), []byte("3")) {
		t.Errorf("expected trip count in fallback description, got %q", desc)
	}
}

func TestBuildEvents_TimezoneApplied(t *testing.T) {
	loc := mustLocalizer("en")
	tz, _ := time.LoadLocation("Europe/Amsterdam")
	r := makeResolved(10, 60, nil, nil, 0, 0)

	events := buildEvents([]resolvedRow{r}, tz, loc)
	if events[0].DtStart.Location() != tz {
		t.Errorf("DtStart timezone want %v got %v", tz, events[0].DtStart.Location())
	}
}
