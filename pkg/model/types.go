package model


import "time"

type StartTimeGroup struct {
    Times         []string               `yaml:"times,omitempty"`
    Trips         *int                   `yaml:"trips,omitempty"`
    TripDuration  *int                   `yaml:"trip_duration,omitempty"`
    BreakDuration *int                   `yaml:"break_duration,omitempty"`
    FirstAdvance  *int                   `yaml:"first_shift_advance,omitempty"`
    LastRemains   *int                   `yaml:"last_shift_remains,omitempty"`
}

type DateRange struct {
    From          time.Time            `yaml:"from"`
    To            time.Time            `yaml:"to"`
    FirstAdvance  *int                 `yaml:"first_shift_advance,omitempty"`
    StartTimes    []StartTimeGroup     `yaml:"start_times,omitempty"`
    Trips         *int                 `yaml:"trips,omitempty"`
    TripDuration  *int                 `yaml:"trip_duration,omitempty"`
    BreakDuration *int                 `yaml:"break_duration,omitempty"`
    LastRemains   *int                 `yaml:"last_shift_remains,omitempty"`
}

type ShiftType struct {
    Summary         string       `yaml:"summary,omitempty"`
    Description     string       `yaml:"description,omitempty"`
    Trips           *int         `yaml:"trips,omitempty"`
    TripDuration    *int         `yaml:"trip_duration,omitempty"`
    BreakDuration   *int         `yaml:"break_duration,omitempty"`
    FirstShiftAdv   *int         `yaml:"first_shift_advance,omitempty"`
    LastShiftRem    *int         `yaml:"last_shift_remains,omitempty"`
    DateRanges      []DateRange  `yaml:"date_ranges,omitempty"`
}

type Config struct {
    Timezone  string                 `yaml:"timezone"`
    Locale    string                 `yaml:"locale"`
    ShiftType map[string]ShiftType   `yaml:"shift_type"`
}
