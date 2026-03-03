// list-shifts prints a Markdown table of all scheduled start times per day of
// the week, grouped by date-range window, as defined in the config file.
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jeroen/make-ics-go/pkg/config"
	"github.com/jeroen/make-ics-go/pkg/model"
)

// weekdayOrder defines the display order Mon → Sun.
var weekdayOrder = []time.Weekday{
	time.Monday,
	time.Tuesday,
	time.Wednesday,
	time.Thursday,
	time.Friday,
	time.Saturday,
	time.Sunday,
}

func main() {
	if err := Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

// windowEntry holds one shift type's DateRange.
type windowEntry struct {
	code    string
	summary string
	dr      model.DateRange
}

// Run is the testable entry point.
func Run(args []string) error {
	fs := flag.NewFlagSet("list-shifts", flag.ContinueOnError)
	cfgPath := fs.String("config", "config.yaml", "Path to YAML config file")
	fs.StringVar(cfgPath, "c", "config.yaml", "Path to YAML config file (alias)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.LoadConfig(*cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if err := config.ValidateConfig(cfg, *cfgPath); err != nil {
		return err
	}

	fmt.Print(renderShiftTable(cfg))
	return nil
}

// renderShiftTable builds the full Markdown output as disjunct period sections.
// A sweep-line over all boundary dates produces non-overlapping intervals;
// every shift type active in each interval is included in that section's table.
func renderShiftTable(cfg model.Config) string {
	// Collect all entries and boundary dates.
	var all []windowEntry
	boundarySet := map[time.Time]struct{}{}

	for code, st := range cfg.ShiftType {
		for _, dr := range st.DateRanges {
			all = append(all, windowEntry{code: code, summary: st.Summary, dr: dr})
			boundarySet[dr.From] = struct{}{}
			boundarySet[dr.To.AddDate(0, 0, 1)] = struct{}{}
		}
	}

	// Sort unique boundary dates.
	boundaries := make([]time.Time, 0, len(boundarySet))
	for t := range boundarySet {
		boundaries = append(boundaries, t)
	}
	sort.Slice(boundaries, func(i, j int) bool { return boundaries[i].Before(boundaries[j]) })

	var sb strings.Builder
	first := true
	for i := 0; i+1 < len(boundaries); i++ {
		start := boundaries[i]
		end := boundaries[i+1].AddDate(0, 0, -1)

		// Collect entries whose date range overlaps [start, end].
		var entries []windowEntry
		for _, e := range all {
			if !e.dr.From.After(end) && !e.dr.To.Before(start) {
				entries = append(entries, e)
			}
		}
		if len(entries) == 0 {
			continue
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].code < entries[j].code })

		if !first {
			sb.WriteString("\n")
		}
		first = false

		fmt.Fprintf(&sb, "## %s – %s\n\n", start.Format("2006-01-02"), end.Format("2006-01-02"))

		// header row: "code (summary)" combined in one cell
		sb.WriteString("| Day |")
		for _, e := range entries {
			shortSummary := strings.TrimPrefix(e.summary, "Binnendieze ")
			fmt.Fprintf(&sb, " %s (%s) |", e.code, shortSummary)
		}
		sb.WriteString("\n")

		// separator
		sb.WriteString("| --- |")
		for range entries {
			sb.WriteString(" --- |")
		}
		sb.WriteString("\n")

		// one row per weekday
		for _, wd := range weekdayOrder {
			fmt.Fprintf(&sb, "| %s |", wd.String()[:3])
			for _, e := range entries {
				fmt.Fprintf(&sb, " %s |", timesForWeekday(e.dr, wd))
			}
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

// timesForWeekday returns the sorted, comma-joined start times for wd in dr,
// or "–" when the weekday is excluded by the weekdays filter or no times are defined.
func timesForWeekday(dr model.DateRange, wd time.Weekday) string {
	if len(dr.Weekdays) > 0 {
		abbr := wd.String()[:3]
		found := false
		for _, w := range dr.Weekdays {
			if w == abbr {
				found = true
				break
			}
		}
		if !found {
			return "–"
		}
	}

	var times []string
	for _, g := range dr.StartTimes {
		times = append(times, g.Times...)
	}
	if len(times) == 0 {
		return "–"
	}
	sort.Strings(times)
	return strings.Join(times, ", ")
}
