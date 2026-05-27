package model

import (
	"time"

	"gopkg.in/yaml.v3"
)

// TripTime defines one explicit trip segment for a shift.
type TripTime struct {
	Start    string `yaml:"start"`
	Duration int    `yaml:"duration"`
}

// Shift defines one numbered shift inside a day schedule.
// The first trip start is used as the departure match key.
// Formats accepted by UnmarshalYAML:
//   - Plain sequence: ["10:20", "11:40"] — trip start times
//   - Mapping with trips sequence: {arrive: "9:15", trips: ["10:20", "11:40"], leave: "15:00"}
//   - Mapping with trip_times: {arrive: "9:00", trip_times: [{start: "10:00"}], leave: "12:30"}
//
// Arrive and Leave are optional. When absent the pipeline applies defaults:
// arrive = first_trip − 30 min; leave = last_trip + min_consecutive_interval (single-trip requires explicit leave).
type Shift struct {
	TripTimes []TripTime `yaml:"trip_times,omitempty"`
	Arrive    *string    `yaml:"arrive,omitempty"`
	Leave     *string    `yaml:"leave,omitempty"`
}

// UnmarshalYAML accepts three formats for a Shift value (see type doc).
func (s *Shift) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.SequenceNode {
		times := make([]TripTime, 0, len(value.Content))
		for _, n := range value.Content {
			times = append(times, TripTime{Start: n.Value})
		}
		s.TripTimes = times
		return nil
	}
	// Mapping: check whether there is a `trips` key with a sequence value.
	// If so, treat that sequence as trip start times and decode the rest normally.
	if value.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(value.Content); i += 2 {
			if value.Content[i].Value == "trips" && value.Content[i+1].Kind == yaml.SequenceNode {
				seq := value.Content[i+1]
				times := make([]TripTime, 0, len(seq.Content))
				for _, n := range seq.Content {
					times = append(times, TripTime{Start: n.Value})
				}
				s.TripTimes = times
				// Decode the remaining fields (arrive, leave) via an alias struct
				// that has no `trips` field so the sequence key is ignored.
				type shiftRest struct {
					Arrive *string `yaml:"arrive,omitempty"`
					Leave  *string `yaml:"leave,omitempty"`
				}
				var rest shiftRest
				if err := value.Decode(&rest); err != nil {
					return err
				}
				s.Arrive = rest.Arrive
				s.Leave = rest.Leave
				return nil
			}
		}
	}
	type shiftAlias Shift
	var alias shiftAlias
	if err := value.Decode(&alias); err != nil {
		return err
	}
	*s = Shift(alias)
	return nil
}

// StartTimeGroup is an optional override within a Slot that narrows scheduling
// parameters to specific departure times.
type StartTimeGroup struct {
	Times []string `yaml:"times,omitempty"`
	Trips *int     `yaml:"trips,omitempty"`
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
	Weekdays   []string         `yaml:"weekdays,omitempty"`
	Trips      *int             `yaml:"trips,omitempty"`
	Shifts     map[string]Shift `yaml:"shifts,omitempty"`
	StartTimes []StartTimeGroup `yaml:"start_times,omitempty"`
}

// Schedule associates one or more named seasons with a set of weekday Slots.
type Schedule struct {
	Seasons []string `yaml:"seasons"`
	Slots   []Slot   `yaml:"day_schedules,omitempty"`
}

// ShiftType holds the scheduling parameters for a named shift code.
type ShiftType struct {
	Summary     string     `yaml:"summary,omitempty"`
	Description string     `yaml:"description,omitempty"`
	Aliases     []string   `yaml:"aliases,omitempty"`
	Trips       *int       `yaml:"trips,omitempty"`
	Schedules   []Schedule `yaml:"season_schedules,omitempty"`
}

// Exception remaps a specific calendar date to a different weekday for schedule
// matching. The key in config.yaml is the ISO date string (e.g. "2026-04-06").
type Exception struct {
	Description string `yaml:"description,omitempty"`
	Weekday     string `yaml:"weekday"`
}

// CalDAVConfig holds the CalDAV sync settings.
// Credentials are read from environment variables at sync time; only their
// names are stored here so secrets never appear in config.yaml.
type CalDAVConfig struct {
	URL                 string `yaml:"url"`
	UsernameEnv         string `yaml:"username_env"`
	PasswordEnv         string `yaml:"password_env"`
	CalendarDisplayName string `yaml:"calendar_display_name"`
}

// Config is the top-level structure of config.yaml.
type Config struct {
	Timezone   string               `yaml:"timezone"`
	Locale     string               `yaml:"locale"`
	Exceptions map[string]Exception `yaml:"exceptions,omitempty"`
	Seasons    map[string]Season    `yaml:"seasons,omitempty"`
	ShiftType  map[string]ShiftType `yaml:"shift_type"`
	CalDAV     CalDAVConfig         `yaml:"caldav,omitempty"`
}
