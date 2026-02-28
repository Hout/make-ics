package config

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/jeroen/make-ics-go/pkg/model"
)

func TestLoadConfig_MissingFile(t *testing.T) {
    tmp := t.TempDir()
    cfg, err := LoadConfig(filepath.Join(tmp, "nope.yaml"))
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
    cfg, err := LoadConfig(p)
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
    if err := ValidateConfig(cfg, path); err == nil {
        t.Fatalf("expected error for missing keys")
    }
}

func TestValidateConfig_InvalidTimezone(t *testing.T) {
    cfg := model.Config{Timezone: "Not/A/Zone", Locale: "nl_NL", ShiftType: map[string]model.ShiftType{"A": {}}}
    if err := ValidateConfig(cfg, "cfg.yaml"); err == nil {
        t.Fatalf("expected error for invalid timezone")
    }
}
