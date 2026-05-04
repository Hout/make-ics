package config

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/jeroen/make-ics-go/pkg/model"
)

// LoadConfig reads and unmarshals the YAML file at path into a Config.
// It also returns a LineMap for use in error messages.
func LoadConfig(path string) (model.Config, LineMap, error) {
	var cfg model.Config
	f, err := os.Open(path)
	if err != nil {
		return cfg, nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return cfg, nil, err
	}
	return LoadConfigFromBytes(data)
}

// LineMap maps a YAML dotted path (e.g. "shift_type.HRm_.first_shift_advance")
// to the 1-based line number of that field's value in the source YAML.
type LineMap = map[string]int

// BuildLineMap parses data as YAML and returns a map from every dotted path
// to the 1-based line number of its value node. Returns nil when data is empty
// or unparseable — callers must handle nil gracefully.
func BuildLineMap(data []byte) LineMap {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil || len(root.Content) == 0 {
		return nil
	}
	lines := make(LineMap)
	walkYAMLNode(root.Content[0], "", lines)
	return lines
}

// walkYAMLNode recursively visits node and records line numbers at their
// full dotted paths (sequences use [N] notation).
func walkYAMLNode(node *yaml.Node, path string, lines LineMap) {
	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i].Value
			val := node.Content[i+1]
			var childPath string
			if path == "" {
				childPath = key
			} else {
				childPath = path + "." + key
			}
			lines[childPath] = val.Line
			walkYAMLNode(val, childPath, lines)
		}
	case yaml.SequenceNode:
		for idx, child := range node.Content {
			childPath := fmt.Sprintf("%s[%d]", path, idx)
			lines[childPath] = child.Line
			walkYAMLNode(child, childPath, lines)
		}
	case yaml.AliasNode:
		walkYAMLNode(node.Alias, path, lines)
	}
}

// lineAnnotation returns " (line N)" when yamlPath has a known line in lines,
// otherwise returns "".
func lineAnnotation(yamlPath string, lines LineMap) string {
	if lines == nil {
		return ""
	}
	if n, ok := lines[yamlPath]; ok {
		return fmt.Sprintf(" (line %d)", n)
	}
	return ""
}

// LoadConfigFromBytes unmarshals YAML config from the provided bytes.
// It also returns a LineMap for use in error messages; the map may be nil
// when data is empty or a node tree cannot be built.
func LoadConfigFromBytes(data []byte) (model.Config, LineMap, error) {
	var cfg model.Config
	if len(data) == 0 {
		return cfg, nil, nil
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, nil, err
	}
	return cfg, BuildLineMap(data), nil
}

// ValidateConfig checks that the required fields are present, that the timezone
// is a valid IANA location name, and that every Schedule.Seasons name references
// a key defined in cfg.Seasons. lines is the LineMap returned by LoadConfig or
// LoadConfigFromBytes and is used to include source line numbers in errors; it
// may be nil.
func ValidateConfig(cfg model.Config, path string, lines LineMap) error {
	if cfg.Timezone == "" || cfg.Locale == "" || len(cfg.ShiftType) == 0 {
		return fmt.Errorf("config file %q is missing required keys: timezone, locale, or shift_type", path)
	}
	if _, err := time.LoadLocation(cfg.Timezone); err != nil {
		return fmt.Errorf("config file %q has invalid timezone: %q", path, cfg.Timezone)
	}
	for code, st := range cfg.ShiftType {
		for si, sched := range st.Schedules {
			if len(sched.Seasons) == 0 {
				return fmt.Errorf("config file %q: shift_type.%s.season_schedules[%d] has no seasons", path, code, si)
			}
			if len(sched.Slots) == 0 {
				return fmt.Errorf("config file %q: shift_type.%s.season_schedules[%d] has no day_schedules", path, code, si)
			}
			for _, name := range sched.Seasons {
				if _, ok := cfg.Seasons[name]; !ok {
					return fmt.Errorf("config file %q: shift_type.%s.season_schedules[%d] references unknown season %q", path, code, si, name)
				}
			}
			for sli, slot := range sched.Slots {
				slotLoc := fmt.Sprintf("shift_type.%s.season_schedules[%d].day_schedules[%d]", code, si, sli)
				if len(slot.Shifts) > 0 && len(slot.StartTimes) > 0 {
					anno := lineAnnotation(slotLoc+".shifts", lines)
					return fmt.Errorf("config file %q: %s%s: may not set both shifts and start_times", path, slotLoc, anno)
				}
				if err := validateSlotShifts(slot.Shifts, slotLoc, lines); err != nil {
					return fmt.Errorf("config file %q: %s", path, err)
				}
				for gi, g := range slot.StartTimes {
					for ti, tm := range g.Times {
						if _, err := time.Parse("15:04", tm); err != nil {
							timePath := fmt.Sprintf("%s.start_times[%d].times[%d]", slotLoc, gi, ti)
							anno := lineAnnotation(timePath, lines)
							return fmt.Errorf("config file %q: %s%s: invalid time %q (expected HH:MM)", path, timePath, anno, tm)
						}
					}
				}
			}
		}
	}
	return nil
}

func validateSlotShifts(shifts map[string]model.Shift, slotLoc string, lines LineMap) error {
	if len(shifts) == 0 {
		return nil
	}
	keys := make([]string, 0, len(shifts))
	for key := range shifts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		ii, _ := strconv.Atoi(keys[i])
		jj, _ := strconv.Atoi(keys[j])
		if ii == jj {
			return keys[i] < keys[j]
		}
		return ii < jj
	})

	for _, key := range keys {
		idx, err := strconv.Atoi(key)
		shiftPath := fmt.Sprintf("%s.shifts.%s", slotLoc, key)
		if err != nil || idx <= 0 {
			anno := lineAnnotation(shiftPath, lines)
			return fmt.Errorf("%s%s: invalid shift key %q (expected positive number string)", shiftPath, anno, key)
		}
		shift := shifts[key]
		if len(shift.TripTimes) == 0 {
			anno := lineAnnotation(shiftPath+".trip_times", lines)
			return fmt.Errorf("%s.trip_times%s: must contain at least one trip", shiftPath, anno)
		}
		prevStart := -1
		for i, tt := range shift.TripTimes {
			startPath := fmt.Sprintf("%s.trip_times[%d].start", shiftPath, i)
			st, err := time.Parse("15:04", tt.Start)
			if err != nil {
				anno := lineAnnotation(startPath, lines)
				return fmt.Errorf("%s%s: invalid time %q (expected HH:MM)", startPath, anno, tt.Start)
			}
			startMin := st.Hour()*60 + st.Minute()
			if prevStart >= 0 && startMin <= prevStart {
				anno := lineAnnotation(startPath, lines)
				return fmt.Errorf("%s%s: trip times must be strictly increasing", startPath, anno)
			}
			prevStart = startMin
		}
	}
	return nil
}
