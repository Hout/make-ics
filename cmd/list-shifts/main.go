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
	"github.com/jeroen/make-ics-go/pkg/schedule"
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

// windowEntry holds one shift type's DateRange together with its parent ShiftType
// so that helpers can resolve the three-level override chain (StartTimeGroup →
// DateRange → ShiftType) without an additional map lookup.
type windowEntry struct {
	code      string
	summary   string
	dr        model.DateRange
	shiftType model.ShiftType
}

// Run is the testable entry point.
func Run(args []string) error {
	fs := flag.NewFlagSet("list-shifts", flag.ContinueOnError)
	cfgPath := fs.String("config", "config.yaml", "Path to YAML config file")
	fs.StringVar(cfgPath, "c", "config.yaml", "Path to YAML config file (alias)")
	showTrips := fs.Bool("trips", false, "Show individual trip start times with sequence numbers")
	showMermaid := fs.Bool("mermaid", false, "Output Mermaid Gantt charts (one per date-range window)")
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

	if *showMermaid {
		fmt.Print(renderMermaidCharts(cfg))
	} else {
		fmt.Print(renderShiftTable(cfg, *showTrips))
	}
	return nil
}

// renderShiftTable builds the full Markdown output as disjunct period sections.
// A sweep-line over all boundary dates produces non-overlapping intervals;
// every shift type active in each interval is included in that section's table.
// When showTrips is true, cells list individual trip start times annotated with
// their 1-based sequence number (e.g. "10:20(1), 11:40(2)"), and weekdays that
// share identical content are collapsed into a single row.
func renderShiftTable(cfg model.Config, showTrips bool) string {
	// Collect all entries and boundary dates.
	var all []windowEntry
	boundarySet := map[time.Time]struct{}{}

	for code, st := range cfg.ShiftType {
		for _, dr := range st.DateRanges {
			all = append(all, windowEntry{code: code, summary: st.Summary, dr: dr, shiftType: st})
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

		fmt.Fprintf(&sb, "## %s \u2013 %s\n", start.Format("2006-01-02"), end.Format("2006-01-02"))

		// Group entries by code so that multiple DateRanges for the same shift
		// type (e.g. split by weekday) are merged into one table.
		var orderedCodes []string
		byCode := map[string][]windowEntry{}
		for _, e := range entries {
			if _, exists := byCode[e.code]; !exists {
				orderedCodes = append(orderedCodes, e.code)
			}
			byCode[e.code] = append(byCode[e.code], e)
		}

		for _, code := range orderedCodes {
			group := byCode[code]
			fmt.Fprintf(&sb, "\n### %s\n\n", group[0].summary)

			if showTrips {
				sb.WriteString("| Day | Trips |\n")
				sb.WriteString("| --- | --- |\n")

				var content [7]string
				for i, wd := range weekdayOrder {
					content[i] = tripStartsForWeekday(group, wd)
				}
				for _, grp := range groupWeekdaysByContent(content) {
					dayLabels := make([]string, len(grp.days))
					for i, d := range grp.days {
						dayLabels[i] = d.String()[:3]
					}
					fmt.Fprintf(&sb, "| %s | %s |\n", strings.Join(dayLabels, ", "), grp.val)
				}
			} else {
				sb.WriteString("| Day | Times |\n")
				sb.WriteString("| --- | --- |\n")

				for _, wd := range weekdayOrder {
					times := mergedTimesForWeekday(group, wd)
					fmt.Fprintf(&sb, "| %s | %s |\n", wd.String()[:3], times)
				}
			}
		}
	}
	return sb.String()
}

// renderMermaidCharts produces a Markdown document where each date-range window
// is rendered as a Mermaid Gantt chart. The chart shows individual trip bars
// for the representative weekday (the one with the most active shift types).
// The output goes to stdout so it can be redirected to a Markdown file.
func renderMermaidCharts(cfg model.Config) string {
	var all []windowEntry
	boundarySet := map[time.Time]struct{}{}
	for code, st := range cfg.ShiftType {
		for _, dr := range st.DateRanges {
			all = append(all, windowEntry{code: code, summary: st.Summary, dr: dr, shiftType: st})
			boundarySet[dr.From] = struct{}{}
			boundarySet[dr.To.AddDate(0, 0, 1)] = struct{}{}
		}
	}
	boundaries := make([]time.Time, 0, len(boundarySet))
	for t := range boundarySet {
		boundaries = append(boundaries, t)
	}
	sort.Slice(boundaries, func(i, j int) bool { return boundaries[i].Before(boundaries[j]) })

	var sb strings.Builder
	first := true
	for i := 0; i+1 < len(boundaries); i++ {
		wStart := boundaries[i]
		wEnd := boundaries[i+1].AddDate(0, 0, -1)

		var entries []windowEntry
		for _, e := range all {
			if !e.dr.From.After(wEnd) && !e.dr.To.Before(wStart) {
				entries = append(entries, e)
			}
		}
		if len(entries) == 0 {
			continue
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].code < entries[j].code })

		var orderedCodes []string
		byCode := map[string][]windowEntry{}
		for _, e := range entries {
			if _, exists := byCode[e.code]; !exists {
				orderedCodes = append(orderedCodes, e.code)
			}
			byCode[e.code] = append(byCode[e.code], e)
		}

		repDay := pickRepresentativeDay(byCode, orderedCodes)
		if repDay == nil {
			continue
		}

		if !first {
			sb.WriteString("\n")
		}
		first = false

		fmt.Fprintf(&sb, "## %s \u2013 %s (%s)\n\n",
			wStart.Format("2006-01-02"), wEnd.Format("2006-01-02"), repDay.String()[:3])
		sb.WriteString("```mermaid\n")
		sb.WriteString("gantt\n")
		fmt.Fprintf(&sb, "    title Shifts \u2013 %s (%s to %s)\n",
			repDay.String(), wStart.Format("2006-01-02"), wEnd.Format("2006-01-02"))
		sb.WriteString("    dateFormat HH:mm\n")
		sb.WriteString("    axisFormat %H:%M\n")

		for _, code := range orderedCodes {
			group := byCode[code]
			segs := resolveTripsForWeekday(group, *repDay)
			if len(segs) == 0 {
				continue
			}
			fmt.Fprintf(&sb, "\n    section %s\n", group[0].summary)
			for _, seg := range segs {
				// Gantt task labels must not contain ':' (it is the separator).
				// Replace ':' in the time with 'h': "10:20" -> "10h20".
				label := strings.Replace(seg.start, ":", "h", 1) +
					fmt.Sprintf(" (%d)", seg.seq)
				fmt.Fprintf(&sb, "        %s : %s, %dm\n", label, seg.start, seg.duration)
			}
		}
		sb.WriteString("```\n")
	}
	return sb.String()
}

// pickRepresentativeDay returns the first weekday (Mon→Sun) that has the most
// shift types with resolvable trip segments active in the given window.
// Returns nil when no weekday has any active shift.
func pickRepresentativeDay(byCode map[string][]windowEntry, orderedCodes []string) *time.Weekday {
	bestCount := 0
	var best *time.Weekday
	for _, wd := range weekdayOrder {
		count := 0
		for _, code := range orderedCodes {
			if len(resolveTripsForWeekday(byCode[code], wd)) > 0 {
				count++
			}
		}
		if count > bestCount {
			bestCount = count
			d := wd
			best = &d
		}
	}
	return best
}

// tripSeg holds a resolved trip segment: start time, trip duration in minutes,
// and 1-based sequence number within its departure run.
type tripSeg struct {
	start    string
	duration int
	seq      int
}

// resolveTripsForWeekday returns the sorted, deduplicated trip segments for wd
// across all entries in group, using the three-level override chain
// (StartTimeGroup → DateRange → ShiftType) for trips, tripDuration, and
// breakDuration. Entries without a resolved tripDuration are silently skipped.
func resolveTripsForWeekday(group []windowEntry, wd time.Weekday) []tripSeg {
	seen := map[string]tripSeg{}
	for _, e := range group {
		if !weekdayAllowed(e.dr, wd) {
			continue
		}
		for _, g := range e.dr.StartTimes {
			var trips *int
			if g.Trips != nil {
				trips = g.Trips
			} else if e.dr.Trips != nil {
				trips = e.dr.Trips
			} else {
				trips = e.shiftType.Trips
			}
			var tripDuration *int
			if g.TripDuration != nil {
				tripDuration = g.TripDuration
			} else if e.dr.TripDuration != nil {
				tripDuration = e.dr.TripDuration
			} else {
				tripDuration = e.shiftType.TripDuration
			}
			if trips == nil || tripDuration == nil {
				continue
			}
			var breakDuration *int
			if g.BreakDuration != nil {
				breakDuration = g.BreakDuration
			} else if e.dr.BreakDuration != nil {
				breakDuration = e.dr.BreakDuration
			} else {
				breakDuration = e.shiftType.BreakDuration
			}
			bd := 0
			if breakDuration != nil {
				bd = *breakDuration
			}
			for _, ts := range g.Times {
				h, m := parseHHMM(ts)
				for i, seg := range schedule.BuildTripTimes(h, m, *trips, *tripDuration, bd) {
					if _, exists := seen[seg.Start]; !exists {
						seen[seg.Start] = tripSeg{start: seg.Start, duration: *tripDuration, seq: i + 1}
					}
				}
			}
		}
	}
	segs := make([]tripSeg, 0, len(seen))
	for _, ts := range seen {
		segs = append(segs, ts)
	}
	sort.Slice(segs, func(i, j int) bool { return segs[i].start < segs[j].start })
	return segs
}

// mergedTimesForWeekday unions the start times across all entries in a group for
// the given weekday, returning a sorted, deduplicated, comma-joined string, or
// "\u2013" if no entry contributes any times.
func mergedTimesForWeekday(group []windowEntry, wd time.Weekday) string {
	seen := map[string]struct{}{}
	var times []string
	for _, e := range group {
		t := timesForWeekday(e.dr, wd)
		if t == "\u2013" {
			continue
		}
		for _, s := range strings.Split(t, ", ") {
			if _, exists := seen[s]; !exists {
				seen[s] = struct{}{}
				times = append(times, s)
			}
		}
	}
	if len(times) == 0 {
		return "\u2013"
	}
	sort.Strings(times)
	return strings.Join(times, ", ")
}

// timesForWeekday returns the sorted, comma-joined start times for wd in dr,
// or "\u2013" when the weekday is excluded by the weekdays filter or no times are defined.
func timesForWeekday(dr model.DateRange, wd time.Weekday) string {
	if !weekdayAllowed(dr, wd) {
		return "\u2013"
	}
	var times []string
	for _, g := range dr.StartTimes {
		times = append(times, g.Times...)
	}
	if len(times) == 0 {
		return "\u2013"
	}
	sort.Strings(times)
	return strings.Join(times, ", ")
}

// weekdayAllowed reports whether wd passes the weekday filter of dr.
// An empty Weekdays slice means all days are allowed.
func weekdayAllowed(dr model.DateRange, wd time.Weekday) bool {
	if len(dr.Weekdays) == 0 {
		return true
	}
	abbr := wd.String()[:3]
	for _, w := range dr.Weekdays {
		if w == abbr {
			return true
		}
	}
	return false
}

// parseHHMM parses a "HH:MM" string into hour and minute integers.
func parseHHMM(s string) (int, int) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, 0
	}
	return t.Hour(), t.Minute()
}

// tripEntry holds one trip start time and its 1-based sequence number within
// its departure run, used when building the trips table column.
type tripEntry struct {
	time string
	seq  int
}

// tripStartsForWeekday computes the individual trip start times for wd across
// all entries in a group. Each time is annotated with its 1-based trip sequence
// number within its departure, e.g. "10:20(1), 11:40(2)". When trip_duration is
// not configured for a StartTimeGroup, the raw departure time is emitted as (1).
// Returns "\u2013" when the weekday is excluded or no times are configured.
func tripStartsForWeekday(group []windowEntry, wd time.Weekday) string {
	seen := map[string]tripEntry{}

	for _, e := range group {
		if !weekdayAllowed(e.dr, wd) {
			continue
		}
		for _, g := range e.dr.StartTimes {
			var trips *int
			if g.Trips != nil {
				trips = g.Trips
			} else if e.dr.Trips != nil {
				trips = e.dr.Trips
			} else {
				trips = e.shiftType.Trips
			}
			var tripDuration *int
			if g.TripDuration != nil {
				tripDuration = g.TripDuration
			} else if e.dr.TripDuration != nil {
				tripDuration = e.dr.TripDuration
			} else {
				tripDuration = e.shiftType.TripDuration
			}
			var breakDuration *int
			if g.BreakDuration != nil {
				breakDuration = g.BreakDuration
			} else if e.dr.BreakDuration != nil {
				breakDuration = e.dr.BreakDuration
			} else {
				breakDuration = e.shiftType.BreakDuration
			}
			bd := 0
			if breakDuration != nil {
				bd = *breakDuration
			}
			for _, ts := range g.Times {
				if trips != nil && tripDuration != nil {
					h, m := parseHHMM(ts)
					for i, seg := range schedule.BuildTripTimes(h, m, *trips, *tripDuration, bd) {
						if _, exists := seen[seg.Start]; !exists {
							seen[seg.Start] = tripEntry{time: seg.Start, seq: i + 1}
						}
					}
				} else {
					// Fallback: no full trip config — emit raw departure time as trip 1.
					if _, exists := seen[ts]; !exists {
						seen[ts] = tripEntry{time: ts, seq: 1}
					}
				}
			}
		}
	}

	if len(seen) == 0 {
		return "\u2013"
	}
	entries := make([]tripEntry, 0, len(seen))
	for _, te := range seen {
		entries = append(entries, te)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].time < entries[j].time })
	parts := make([]string, len(entries))
	for i, te := range entries {
		parts[i] = fmt.Sprintf("%s(%d)", te.time, te.seq)
	}
	return strings.Join(parts, ", ")
}

// weekdayGroup is a set of weekdays that share identical cell content.
type weekdayGroup struct {
	days []time.Weekday
	val  string
}

// groupWeekdaysByContent groups the 7 Mon–Sun content strings so that weekdays
// with identical values share a single row. Groups are returned in Mon→Sun order
// of their first-occurring weekday.
func groupWeekdaysByContent(content [7]string) []weekdayGroup {
	var ordered []string
	groups := map[string]*weekdayGroup{}

	for i, wd := range weekdayOrder {
		val := content[i]
		if _, exists := groups[val]; !exists {
			groups[val] = &weekdayGroup{val: val}
			ordered = append(ordered, val)
		}
		groups[val].days = append(groups[val].days, wd)
	}

	result := make([]weekdayGroup, len(ordered))
	for i, val := range ordered {
		result[i] = *groups[val]
	}
	return result
}
