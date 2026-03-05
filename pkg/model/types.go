package model

import "time"

// StartTimeGroup is an optional override within a DateRange that narrows
// scheduling parameters to specific departure times.
type StartTimeGroup struct {
	Times         []string `yaml:"times,omitempty"`
	Trips         *int     `yaml:"trips,omitempty"`
	TripDuration  *int     `yaml:"trip_duration,omitempty"`
	BreakDuration *int     `yaml:"break_duration,omitempty"`
	FirstAdvance  *int     `yaml:"first_shift_advance,omitempty"`
	LastRemains   *int     `yaml:"last_shift_remains,omitempty"`
}

// DateRange defines a calendar window [From, To] with optional per-window
// overrides for trips, durations, advance, and last-shift extension.
type DateRange struct {
	From          time.Time        `yaml:"from"`
	To            time.Time        `yaml:"to"`
	Weekdays      []string         `yaml:"weekdays,omitempty"`
	FirstAdvance  *int             `yaml:"first_shift_advance,omitempty"`
	StartTimes    []StartTimeGroup `yaml:"start_times,omitempty"`
	Trips         *int             `yaml:"trips,omitempty"`
	TripDuration  *int             `yaml:"trip_duration,omitempty"`
	BreakDuration *int             `yaml:"break_duration,omitempty"`
	LastRemains   *int             `yaml:"last_shift_remains,omitempty"`
}

// ShiftType holds the scheduling parameters for a named shift code.
type ShiftType struct {
	Summary       string      `yaml:"summary,omitempty"`
	Description   string      `yaml:"description,omitempty"`
	Trips         *int        `yaml:"trips,omitempty"`
	TripDuration  *int        `yaml:"trip_duration,omitempty"`
	BreakDuration *int        `yaml:"break_duration,omitempty"`
	FirstShiftAdv *int        `yaml:"first_shift_advance,omitempty"`
	LastShiftRem  *int        `yaml:"last_shift_remains,omitempty"`
	DateRanges    []DateRange `yaml:"date_ranges,omitempty"`
}

// Config is the top-level structure of config.yaml.
type Config struct {
	Timezone  string               `yaml:"timezone"`
	Locale    string               `yaml:"locale"`
	ShiftType map[string]ShiftType `yaml:"shift_type"`
}
