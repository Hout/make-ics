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
	cfg, _, err := LoadConfig(filepath.Join(tmp, "nope.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.ShiftType) != 0 || cfg.Timezone != "" {
		t.Fatalf("expected empty config for missing file")
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

func TestValidateConfig_BothFirstShiftFieldsOnShiftType(t *testing.T) {
	adv := 30
	ft := "09:00"
	cfg := model.Config{
		Timezone: "Europe/Amsterdam",
		Locale:   "nl_NL",
		ShiftType: map[string]model.ShiftType{
			"A": {FirstShiftAdvanceDuration: &adv, FirstShiftAdvanceTime: &ft},
		},
	}
	if err := ValidateConfig(cfg, "cfg.yaml", nil); err == nil {
		t.Fatalf("expected error when both first_shift_advance and first_shift_time set on ShiftType")
	}
}

func TestValidateConfig_BothFirstShiftFieldsOnSlot(t *testing.T) {
	adv := 30
	ft := "09:00"
	from := model.DateRange{From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)}
	cfg := model.Config{
		Timezone: "Europe/Amsterdam",
		Locale:   "nl_NL",
		Seasons:  map[string]model.Season{"s": {from}},
		ShiftType: map[string]model.ShiftType{
			"A": {Schedules: []model.Schedule{{
				Seasons: []string{"s"},
				Slots:   []model.Slot{{FirstShiftAdvanceDuration: &adv, FirstShiftAdvanceTime: &ft}},
			}}},
		},
	}
	if err := ValidateConfig(cfg, "cfg.yaml", nil); err == nil {
		t.Fatalf("expected error when both fields set on Slot")
	}
}

func TestValidateConfig_InvalidFirstShiftTimeFormat(t *testing.T) {
	ft := "9am"
	cfg := model.Config{
		Timezone: "Europe/Amsterdam",
		Locale:   "nl_NL",
		ShiftType: map[string]model.ShiftType{
			"A": {FirstShiftAdvanceTime: &ft},
		},
	}
	if err := ValidateConfig(cfg, "cfg.yaml", nil); err == nil {
		t.Fatalf("expected error for invalid first_shift_time format")
	}
}

func TestValidateConfig_ValidFirstShiftTime(t *testing.T) {
	ft := "09:00"
	from := model.DateRange{From: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), To: time.Date(2026, 4, 30, 0, 0, 0, 0, time.UTC)}
	cfg := model.Config{
		Timezone: "Europe/Amsterdam",
		Locale:   "nl_NL",
		Seasons:  map[string]model.Season{"s": {from}},
		ShiftType: map[string]model.ShiftType{
			"A": {
				FirstShiftAdvanceTime: &ft,
				Schedules: []model.Schedule{{
					Seasons: []string{"s"},
					Slots:   []model.Slot{{}},
				}},
			},
		},
	}
	if err := ValidateConfig(cfg, "cfg.yaml", nil); err != nil {
		t.Fatalf("unexpected error for valid config: %v", err)
	}
}
