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
// It also returns a LineMap for use in error messages.
func LoadConfig(path string) (model.Config, LineMap, error) {
	var cfg model.Config
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil, nil
		}
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
		loc := fmt.Sprintf("shift_type.%s", code)
		if err := checkFirstShiftFields(st.FirstShiftAdvanceDuration, st.FirstShiftAdvanceTime, loc, lines); err != nil {
			return fmt.Errorf("config file %q: %s", path, err)
		}
		for si, sched := range st.Schedules {
			if len(sched.Seasons) == 0 {
				return fmt.Errorf("config file %q: shift_type.%s.schedules[%d] has no seasons", path, code, si)
			}
			if len(sched.Slots) == 0 {
				return fmt.Errorf("config file %q: shift_type.%s.schedules[%d] has no slots", path, code, si)
			}
			for _, name := range sched.Seasons {
				if _, ok := cfg.Seasons[name]; !ok {
					return fmt.Errorf("config file %q: shift_type.%s.schedules[%d] references unknown season %q", path, code, si, name)
				}
			}
			for sli, slot := range sched.Slots {
				slotLoc := fmt.Sprintf("shift_type.%s.schedules[%d].slots[%d]", code, si, sli)
				if err := checkFirstShiftFields(slot.FirstShiftAdvanceDuration, slot.FirstShiftAdvanceTime, slotLoc, lines); err != nil {
					return fmt.Errorf("config file %q: %s", path, err)
				}
				_ = slot.StartTimes // first_shift_* fields are not supported at start_times level
			}
		}
	}
	return nil
}

// checkFirstShiftFields returns an error when both first_shift_advance_duration and
// first_shift_advance_time are set on the same struct, or when first_shift_advance_time
// is not a valid HH:MM string. When lines is non-nil the offending field's line number
// is appended to the error message.
func checkFirstShiftFields(firstAdv *int, firstTime *string, location string, lines LineMap) error {
	if firstAdv != nil && firstTime != nil {
		anno := lineAnnotation(location+".first_shift_advance_time", lines)
		return fmt.Errorf("%s%s: may not set both first_shift_advance_duration and first_shift_advance_time", location, anno)
	}
	if firstTime != nil {
		if _, err := time.Parse("15:04", *firstTime); err != nil {
			anno := lineAnnotation(location+".first_shift_advance_time", lines)
			return fmt.Errorf("%s%s: invalid first_shift_advance_time %q (expected HH:MM)", location, anno, *firstTime)
		}
	}
	return nil
}
