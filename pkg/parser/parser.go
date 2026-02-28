package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	dutchDateRE = regexp.MustCompile(`(?i)^(\d{2})-([A-Za-z]{3})-(\d{2})$`)
	timeRE      = regexp.MustCompile(`^(\d{1,2}):(\d{2})`)
	dataRowRE   = regexp.MustCompile(`^\d{2}-[A-Za-z]{3}-\d{2}$`)

	monthMap = map[string]time.Month{
		"jan": time.January,
		"feb": time.February,
		"mrt": time.March,
		"mar": time.March,
		"apr": time.April,
		"mei": time.May,
		"jun": time.June,
		"jul": time.July,
		"aug": time.August,
		"sep": time.September,
		"okt": time.October,
		"oct": time.October,
		"nov": time.November,
		"dec": time.December,
	}
)

// ParseDutchDate parses a strict Dutch date in format DD-MMM-YY (e.g. 03-apr-26)
// and returns a time.Time with date fields set (hour/min/sec/nsec zero) in UTC.
// Two-digit year is interpreted as 2000 + yy.
func ParseDutchDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	m := dutchDateRE.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, fmt.Errorf("could not parse date: %q", s)
	}
	dayStr, monStr, yearStr := m[1], strings.ToLower(m[2]), m[3]
	day, err := strconv.Atoi(dayStr)
	if err != nil {
		return time.Time{}, err
	}
	mon, ok := monthMap[monStr]
	if !ok {
		return time.Time{}, fmt.Errorf("unknown month abbreviation: %q", monStr)
	}
	yy, err := strconv.Atoi(yearStr)
	if err != nil {
		return time.Time{}, err
	}
	year := 2000 + yy
	// Validate day/month by attempting to construct time
	t := time.Date(year, mon, day, 0, 0, 0, 0, time.UTC)
	if t.Day() != day || t.Month() != mon || t.Year() != year {
		return time.Time{}, fmt.Errorf("invalid date components: %q", s)
	}
	return t, nil
}

// ParseTime parses a time string like "14:40 uur" and returns hour, minute.
func ParseTime(s string) (int, int, error) {
	s = strings.TrimSpace(s)
	m := timeRE.FindStringSubmatch(s)
	if m == nil {
		return 0, 0, fmt.Errorf("unexpected time format: %q", s)
	}
	h, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, 0, err
	}
	min, err := strconv.Atoi(m[2])
	if err != nil {
		return 0, 0, err
	}
	if h < 0 || h > 23 || min < 0 || min > 59 {
		return 0, 0, fmt.Errorf("invalid time: %02d:%02d", h, min)
	}
	return h, min, nil
}

// IsDataRow returns true if the provided cell strings look like a data row
// with a date in DD-MMM-YY in first column and a time in third column.
func IsDataRow(cells []string) bool {
	if len(cells) < 3 {
		return false
	}
	dateVal := strings.TrimSpace(cells[0])
	timeVal := strings.TrimSpace(cells[2])
	if dateVal == "" || timeVal == "" {
		return false
	}
	// must match dd-mon-yy
	return dataRowRE.MatchString(dateVal)
}
