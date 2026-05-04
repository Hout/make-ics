package model

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func unmarshalShift(t *testing.T, src string) Shift {
	t.Helper()
	var s Shift
	if err := yaml.Unmarshal([]byte(src), &s); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	return s
}

func TestShift_SequenceFormat(t *testing.T) {
	s := unmarshalShift(t, `["10:20", "11:40", "13:00"]`)
	if len(s.TripTimes) != 3 {
		t.Fatalf("expected 3 trip times, got %d", len(s.TripTimes))
	}
	if s.TripTimes[0].Start != "10:20" || s.TripTimes[1].Start != "11:40" || s.TripTimes[2].Start != "13:00" {
		t.Fatalf("unexpected trip times: %+v", s.TripTimes)
	}
	if s.Arrive != nil || s.Leave != nil {
		t.Fatalf("expected nil Arrive/Leave for sequence format")
	}
}

func TestShift_MappingWithTripsSequence(t *testing.T) {
	src := `{arrive: "9:15", trips: ["10:20", "11:40"], leave: "15:00"}`
	s := unmarshalShift(t, src)
	if len(s.TripTimes) != 2 {
		t.Fatalf("expected 2 trip times, got %d", len(s.TripTimes))
	}
	if s.TripTimes[0].Start != "10:20" || s.TripTimes[1].Start != "11:40" {
		t.Fatalf("unexpected trip times: %+v", s.TripTimes)
	}
	if s.Arrive == nil || *s.Arrive != "9:15" {
		t.Fatalf("expected Arrive=9:15, got %v", s.Arrive)
	}
	if s.Leave == nil || *s.Leave != "15:00" {
		t.Fatalf("expected Leave=15:00, got %v", s.Leave)
	}
}

func TestShift_MappingWithTripsSequence_ArrivLeaveOptional(t *testing.T) {
	src := `{trips: ["10:20", "11:40"]}`
	s := unmarshalShift(t, src)
	if len(s.TripTimes) != 2 {
		t.Fatalf("expected 2 trip times, got %d", len(s.TripTimes))
	}
	if s.Arrive != nil {
		t.Fatalf("expected nil Arrive, got %v", s.Arrive)
	}
	if s.Leave != nil {
		t.Fatalf("expected nil Leave, got %v", s.Leave)
	}
}

func TestShift_MappingWithTripTimes(t *testing.T) {
	src := `{trip_times: [{start: "10:20"}, {start: "11:40"}]}`
	s := unmarshalShift(t, src)
	if len(s.TripTimes) != 2 {
		t.Fatalf("expected 2 trip times, got %d", len(s.TripTimes))
	}
	if s.TripTimes[0].Start != "10:20" {
		t.Fatalf("unexpected first trip time: %v", s.TripTimes[0].Start)
	}
	if s.Arrive != nil || s.Leave != nil {
		t.Fatalf("expected nil Arrive/Leave for trip_times format")
	}
}

func TestShift_MappingWithTripTimes_ArrivLeave(t *testing.T) {
	src := `{arrive: "9:00", trip_times: [{start: "10:00"}], leave: "12:30"}`
	s := unmarshalShift(t, src)
	if s.Arrive == nil || *s.Arrive != "9:00" {
		t.Fatalf("expected Arrive=9:00, got %v", s.Arrive)
	}
	if s.Leave == nil || *s.Leave != "12:30" {
		t.Fatalf("expected Leave=12:30, got %v", s.Leave)
	}
}
