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

// dateFmt is the date layout used in section headings when dates span multiple months.
const dateFmt = "02-Jan-06"

// formatDateRange formats a date range for a section heading.
// When start and end fall in the same month and year the compact form
// "dd > dd Mmm-yy" is used; otherwise both dates are rendered in full
// as "dd-Mmm-yy – dd-Mmm-yy".
func formatDateRange(start, end time.Time) string {
	if start.Month() == end.Month() && start.Year() == end.Year() {
		return fmt.Sprintf("%02d > %02d %s", start.Day(), end.Day(), end.Format("Jan-06"))
	}
	return start.Format(dateFmt) + " > " + end.Format(dateFmt)
}

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
		fmt.Print(renderShiftTable(cfg))
	}
	return nil
}

// renderShiftTable builds the full Markdown output as disjunct period sections.
// A sweep-line over all boundary dates produces non-overlapping intervals;
// every shift type active in each interval is included in that section's table.
// Weekdays with identical times are collapsed into a single row (e.g. Tue–Sun).
func renderShiftTable(cfg model.Config) string {
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

		fmt.Fprintf(&sb, "## %s\n", formatDateRange(start, end))

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

			sb.WriteString("| Day | Times |\n")
			sb.WriteString("| --- | --- |\n")

			var content [7]string
			for i, wd := range weekdayOrder {
				content[i] = mergedTimesForWeekday(group, wd)
			}
			for _, grp := range groupWeekdaysByContent(content) {
				if grp.val == "\u2013" {
					continue
				}
				fmt.Fprintf(&sb, "| %s | %s |\n", formatDayRange(grp.days), grp.val)
			}
		}
	}
	return sb.String()
}

// renderMermaidCharts produces a Markdown document where each date-range window
// is rendered as a Mermaid Gantt chart. Each departure is shown as a single bar
// spanning from the first trip's start to the last trip's end (breaks included).
// The chart uses the representative weekday (the one with the most active shifts).
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

		fmt.Fprintf(&sb, "## %s (%s)\n\n",
			formatDateRange(wStart, wEnd), repDay.String()[:3])
		sb.WriteString("```mermaid\n")
		sb.WriteString("gantt\n")
		fmt.Fprintf(&sb, "    title Shifts \u2013 %s (%s to %s)\n",
			repDay.String(), wStart.Format(dateFmt), wEnd.Format(dateFmt))
		sb.WriteString("    dateFormat HH:mm\n")
		sb.WriteString("    axisFormat %H:%M\n")

		for _, code := range orderedCodes {
			group := byCode[code]
			deps := resolveDeparturesForWeekday(group, *repDay)
			if len(deps) == 0 {
				continue
			}
			fmt.Fprintf(&sb, "\n    section %s\n", group[0].summary)
			for _, dep := range deps {
				// Gantt task labels must not contain ':' (it is the separator).
				// Replace ':' in the time with 'h': "10:20" -> "10h20".
				label := strings.Replace(dep.start, ":", "h", 1)
				fmt.Fprintf(&sb, "        %s : %s, %dm\n", label, dep.start, dep.totalDuration)
			}
		}
		sb.WriteString("```\n")
	}
	return sb.String()
}

// pickRepresentativeDay returns the first weekday (Mon→Sun) that has the most
// shift types with resolvable departure bars active in the given window.
// Returns nil when no weekday has any active shift.
func pickRepresentativeDay(byCode map[string][]windowEntry, orderedCodes []string) *time.Weekday {
	bestCount := 0
	var best *time.Weekday
	for _, wd := range weekdayOrder {
		count := 0
		for _, code := range orderedCodes {
			if len(resolveDeparturesForWeekday(byCode[code], wd)) > 0 {
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
// depBar holds one departure's start time and its total span in minutes
// (trips × tripDuration + max(0, trips−1) × breakDuration).
type depBar struct {
	start         string
	totalDuration int
}

// resolveDeparturesForWeekday returns one depBar per unique departure start time
// for wd across all entries in group, using the three-level override chain
// (StartTimeGroup → DateRange → ShiftType). The bar spans the full working time
// from the first trip start to the last trip end (breaks included).
// Entries without a resolved tripDuration are silently skipped.
func resolveDeparturesForWeekday(group []windowEntry, wd time.Weekday) []depBar {
	seen := map[string]depBar{}
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
			n := *trips
			td := *tripDuration
			total := n*td + max(0, n-1)*bd
			for _, ts := range g.Times {
				if _, exists := seen[ts]; !exists {
					seen[ts] = depBar{start: ts, totalDuration: total}
				}
			}
		}
	}
	bars := make([]depBar, 0, len(seen))
	for _, b := range seen {
		bars = append(bars, b)
	}
	sort.Slice(bars, func(i, j int) bool { return bars[i].start < bars[j].start })
	return bars
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

// weekdayGroup is a set of weekdays that share identical cell content.
type weekdayGroup struct {
	days []time.Weekday
	val  string
}

// formatDayRange formats a slice of weekdays (in Mon→Sun order) as a compact
// range string, collapsing consecutive runs with an en-dash, e.g. [Tue Wed Thu
// Fri Sat Sun] → "Tue–Sun", [Mon Sat Sun] → "Mon, Sat–Sun".
func formatDayRange(days []time.Weekday) string {
	// Build an index so we can detect consecutiveness.
	wdIndex := map[time.Weekday]int{}
	for i, wd := range weekdayOrder {
		wdIndex[wd] = i
	}

	var parts []string
	runStart := days[0]
	runEnd := days[0]
	for _, wd := range days[1:] {
		if wdIndex[wd] == wdIndex[runEnd]+1 {
			runEnd = wd
		} else {
			parts = append(parts, fmtRun(runStart, runEnd))
			runStart = wd
			runEnd = wd
		}
	}
	parts = append(parts, fmtRun(runStart, runEnd))
	return strings.Join(parts, ", ")
}

// fmtRun formats a single consecutive run of weekdays.
func fmtRun(start, end time.Weekday) string {
	if start == end {
		return start.String()[:3]
	}
	return start.String()[:3] + "\u2013" + end.String()[:3]
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
