package config

import (
    "fmt"
    "io"
    "os"
    "time"

    "gopkg.in/yaml.v3"

    "github.com/jeroen/make-ics-go/pkg/model"
)

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
    if len(data) == 0 {
        return cfg, nil
    }
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return cfg, err
    }
    return cfg, nil
}

func ValidateConfig(cfg model.Config, path string) error {
    if cfg.Timezone == "" || cfg.Locale == "" || len(cfg.ShiftType) == 0 {
        return fmt.Errorf("Config file '%s' is missing required keys: timezone, locale, or shift_type", path)
    }
    if _, err := time.LoadLocation(cfg.Timezone); err != nil {
        return fmt.Errorf("Config file '%s' has invalid timezone: %q", path, cfg.Timezone)
    }
    return nil
}
