package config

import (
	"fmt"
	"io"
	"os"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/jeroen/make-ics-go/pkg/model"
)

// IsEmpty reports whether cfg is a zero-value Config (no timezone set).
// It is used to detect when no config file was found so the caller can
// fall back to a compiled-in default.
func IsEmpty(cfg model.Config) bool {
	return cfg.Timezone == "" && cfg.Locale == "" && len(cfg.ShiftType) == 0
}

// LoadConfig reads and unmarshals the YAML file at path into a Config.
// A missing file is not an error — it returns a zero-value Config.
func LoadConfig(path string) (model.Config, error) {
	var cfg model.Config
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return cfg, err
	}
	return LoadConfigFromBytes(data)
}

// LoadConfigFromBytes unmarshals YAML config from the provided bytes.
func LoadConfigFromBytes(data []byte) (model.Config, error) {
	var cfg model.Config
	if len(data) == 0 {
		return cfg, nil
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// ValidateConfig checks that the required fields are present and that the
// timezone is a valid IANA location name.
func ValidateConfig(cfg model.Config, path string) error {
	if cfg.Timezone == "" || cfg.Locale == "" || len(cfg.ShiftType) == 0 {
		return fmt.Errorf("config file %q is missing required keys: timezone, locale, or shift_type", path)
	}
	if _, err := time.LoadLocation(cfg.Timezone); err != nil {
		return fmt.Errorf("config file %q has invalid timezone: %q", path, cfg.Timezone)
	}
	return nil
}
