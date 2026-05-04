package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeroen/make-ics-go/pkg/model"
)

func TestLoadConfig_MissingFile(t *testing.T) {
	tmp := t.TempDir()
	_, _, err := LoadConfig(filepath.Join(tmp, "nope.yaml"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoadConfig_EmptyFile(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "cfg.yaml")
	if err := os.WriteFile(p, []byte(""), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	cfg, _, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Timezone != "" {
		t.Fatalf("expected empty timezone for empty file")
	}
}

func TestValidateConfig_MissingKeys(t *testing.T) {
	var cfg model.Config
	tmp := t.TempDir()
	path := filepath.Join(tmp, "cfg.yaml")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if err := ValidateConfig(cfg, path, nil); err == nil {
		t.Fatalf("expected error for missing keys")
	}
}

func TestValidateConfig_InvalidTimezone(t *testing.T) {
	cfg := model.Config{Timezone: "Not/A/Zone", Locale: "nl_NL", ShiftType: map[string]model.ShiftType{"A": {}}}
	if err := ValidateConfig(cfg, "cfg.yaml", nil); err == nil {
		t.Fatalf("expected error for invalid timezone")
	}
}

func TestValidateConfig_InvalidStartTime(t *testing.T) {
	from := model.DateRange{From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)}
	cfg := model.Config{
		Timezone: "Europe/Amsterdam",
		Locale:   "nl_NL",
		Seasons:  map[string]model.Season{"s": {from}},
		ShiftType: map[string]model.ShiftType{
			"A": {Schedules: []model.Schedule{{
				Seasons: []string{"s"},
				Slots: []model.Slot{{
					StartTimes: []model.StartTimeGroup{{Times: []string{"9am"}}},
				}},
			}}},
		},
	}
	if err := ValidateConfig(cfg, "cfg.yaml", nil); err == nil {
		t.Fatalf("expected error for invalid start_times time format")
	}
}

func TestValidateConfig_ValidStartTime(t *testing.T) {
	from := model.DateRange{From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)}
	cfg := model.Config{
		Timezone: "Europe/Amsterdam",
		Locale:   "nl_NL",
		Seasons:  map[string]model.Season{"s": {from}},
		ShiftType: map[string]model.ShiftType{
			"A": {Schedules: []model.Schedule{{
				Seasons: []string{"s"},
				Slots: []model.Slot{{
					StartTimes: []model.StartTimeGroup{{Times: []string{"08:00", "13:30"}}},
				}},
			}}},
		},
	}
	if err := ValidateConfig(cfg, "cfg.yaml", nil); err != nil {
		t.Fatalf("unexpected error for valid start_times: %v", err)
	}
}

func TestValidateConfig_InvalidShiftKey(t *testing.T) {
	from := model.DateRange{From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)}
	cfg := model.Config{
		Timezone: "Europe/Amsterdam",
		Locale:   "nl_NL",
		Seasons:  map[string]model.Season{"s": {from}},
		ShiftType: map[string]model.ShiftType{
			"A": {Schedules: []model.Schedule{{
				Seasons: []string{"s"},
				Slots: []model.Slot{{
					Shifts: map[string]model.Shift{
						"morning": {TripTimes: []model.TripTime{{Start: "10:00", Duration: 50}}},
					},
				}},
			}}},
		},
	}
	if err := ValidateConfig(cfg, "cfg.yaml", nil); err == nil {
		t.Fatalf("expected error for non-numeric shift key")
	}
}

func TestValidateConfig_EmptyShiftTripTimes(t *testing.T) {
	from := model.DateRange{From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)}
	cfg := model.Config{
		Timezone: "Europe/Amsterdam",
		Locale:   "nl_NL",
		Seasons:  map[string]model.Season{"s": {from}},
		ShiftType: map[string]model.ShiftType{
			"A": {Schedules: []model.Schedule{{
				Seasons: []string{"s"},
				Slots: []model.Slot{{
					Shifts: map[string]model.Shift{"1": {}},
				}},
			}}},
		},
	}
	if err := ValidateConfig(cfg, "cfg.yaml", nil); err == nil {
		t.Fatalf("expected error for empty shift trip_times")
	}
}

func TestValidateConfig_InvalidShiftTripTimeFormat(t *testing.T) {
	from := model.DateRange{From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)}
	cfg := model.Config{
		Timezone: "Europe/Amsterdam",
		Locale:   "nl_NL",
		Seasons:  map[string]model.Season{"s": {from}},
		ShiftType: map[string]model.ShiftType{
			"A": {Schedules: []model.Schedule{{
				Seasons: []string{"s"},
				Slots: []model.Slot{{
					Shifts: map[string]model.Shift{
						"1": {TripTimes: []model.TripTime{{Start: "9am", Duration: 50}}},
					},
				}},
			}}},
		},
	}
	if err := ValidateConfig(cfg, "cfg.yaml", nil); err == nil {
		t.Fatalf("expected error for invalid shift trip_times format")
	}
}

func TestValidateConfig_NonIncreasingShiftTripTimes(t *testing.T) {
	from := model.DateRange{From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)}
	cfg := model.Config{
		Timezone: "Europe/Amsterdam",
		Locale:   "nl_NL",
		Seasons:  map[string]model.Season{"s": {from}},
		ShiftType: map[string]model.ShiftType{
			"A": {Schedules: []model.Schedule{{
				Seasons: []string{"s"},
				Slots: []model.Slot{{
					Shifts: map[string]model.Shift{
						"1": {TripTimes: []model.TripTime{{Start: "10:00", Duration: 50}, {Start: "10:00", Duration: 50}}},
					},
				}},
			}}},
		},
	}
	if err := ValidateConfig(cfg, "cfg.yaml", nil); err == nil {
		t.Fatalf("expected error for overlapping or non-increasing shift trip_times")
	}
}

func TestValidateConfig_ShiftsAndStartTimesConflict(t *testing.T) {
	from := model.DateRange{From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)}
	cfg := model.Config{
		Timezone: "Europe/Amsterdam",
		Locale:   "nl_NL",
		Seasons:  map[string]model.Season{"s": {from}},
		ShiftType: map[string]model.ShiftType{
			"A": {Schedules: []model.Schedule{{
				Seasons: []string{"s"},
				Slots: []model.Slot{{
					Shifts:     map[string]model.Shift{"1": {TripTimes: []model.TripTime{{Start: "10:00", Duration: 50}}}},
					StartTimes: []model.StartTimeGroup{{Times: []string{"10:00"}}},
				}},
			}}},
		},
	}
	if err := ValidateConfig(cfg, "cfg.yaml", nil); err == nil {
		t.Fatalf("expected error when both shifts and start_times are set on same day schedule")
	}
}

func TestValidateConfig_ValidShiftTripTimes(t *testing.T) {
	from := model.DateRange{From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)}
	cfg := model.Config{
		Timezone: "Europe/Amsterdam",
		Locale:   "nl_NL",
		Seasons:  map[string]model.Season{"s": {from}},
		ShiftType: map[string]model.ShiftType{
			"A": {Schedules: []model.Schedule{{
				Seasons: []string{"s"},
				Slots: []model.Slot{{
					Shifts: map[string]model.Shift{
						"1": {TripTimes: []model.TripTime{{Start: "10:00", Duration: 50}, {Start: "11:20", Duration: 50}}},
					},
				}},
			}}},
		},
	}
	if err := ValidateConfig(cfg, "cfg.yaml", nil); err != nil {
		t.Fatalf("unexpected error for valid shifts trip_times: %v", err)
	}
}
