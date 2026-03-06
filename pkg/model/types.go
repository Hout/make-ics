package model

import "time"

// StartTimeGroup is an optional override within a Slot that narrows scheduling
// parameters to specific departure times.
type StartTimeGroup struct {
	Times         []string `yaml:"times,omitempty"`
	Trips         *int     `yaml:"trips,omitempty"`
	TripDuration  *int     `yaml:"trip_duration,omitempty"`
	BreakDuration *int     `yaml:"break_duration,omitempty"`
	LastAftercare *int     `yaml:"last_shift_aftercare,omitempty"`
}

// DateRange is a date window [From, To] inclusive.
type DateRange struct {
	From time.Time `yaml:"from"`
	To   time.Time `yaml:"to"`
}

// Season is a named collection of one or more DateRange windows.
type Season []DateRange

// Slot defines the scheduling parameters for a specific weekday set within a Schedule.
type Slot struct {
	Weekdays                  []string         `yaml:"weekdays,omitempty"`
	Trips                     *int             `yaml:"trips,omitempty"`
	TripDuration              *int             `yaml:"trip_duration,omitempty"`
	BreakDuration             *int             `yaml:"break_duration,omitempty"`
	FirstShiftAdvanceDuration *int             `yaml:"first_shift_advance_duration,omitempty"`
	FirstShiftAdvanceTime     *string          `yaml:"first_shift_advance_time,omitempty"`
	FirstShiftCount           *int             `yaml:"first_shift_count,omitempty"`
	LastAftercare             *int             `yaml:"last_shift_aftercare,omitempty"`
	ShiftPreparation          *int             `yaml:"shift_preparation,omitempty"`
	StartTimes                []StartTimeGroup `yaml:"start_times,omitempty"`
}

// Schedule associates one or more named seasons with a set of weekday Slots.
type Schedule struct {
	Seasons []string `yaml:"seasons"`
	Slots   []Slot   `yaml:"slots,omitempty"`
}

// ShiftType holds the scheduling parameters for a named shift code.
type ShiftType struct {
	Summary                   string     `yaml:"summary,omitempty"`
	Description               string     `yaml:"description,omitempty"`
	Trips                     *int       `yaml:"trips,omitempty"`
	TripDuration              *int       `yaml:"trip_duration,omitempty"`
	BreakDuration             *int       `yaml:"break_duration,omitempty"`
	FirstShiftAdvanceDuration *int       `yaml:"first_shift_advance_duration,omitempty"`
	FirstShiftAdvanceTime     *string    `yaml:"first_shift_advance_time,omitempty"`
	FirstShiftCount           *int       `yaml:"first_shift_count,omitempty"`
	LastShiftAftercare        *int       `yaml:"last_shift_aftercare,omitempty"`
	ShiftPreparation          *int       `yaml:"shift_preparation,omitempty"`
	Schedules                 []Schedule `yaml:"schedules,omitempty"`
}

// Exception remaps a specific calendar date to a different weekday for schedule
// matching. The key in config.yaml is the ISO date string (e.g. "2026-04-06").
type Exception struct {
	Description string `yaml:"description,omitempty"`
	Weekday     string `yaml:"weekday"`
}

// Config is the top-level structure of config.yaml.
type Config struct {
	Timezone   string               `yaml:"timezone"`
	Locale     string               `yaml:"locale"`
	Exceptions map[string]Exception `yaml:"exceptions,omitempty"`
	Seasons    map[string]Season    `yaml:"seasons,omitempty"`
	ShiftType  map[string]ShiftType `yaml:"shift_type"`
}
