package pipeline

import (
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	"github.com/jeroen/make-ics-go/pkg/i18n"
	"github.com/jeroen/make-ics-go/pkg/model"
)

// makeICSNamespace is the UUID v5 namespace for deterministic event UIDs.
// All events produced by this program share this namespace so the same shift
// always yields the same UID across runs.
var makeICSNamespace = uuid.MustParse("4d616b65-4943-5300-0000-000000000001")

// Event holds all fields needed to write a single VEVENT to an ICS calendar.
type Event struct {
	Summary     string
	Description string
	DtStart     time.Time
	DtEnd       time.Time
	UID         string
}

// IterEvents reads the first sheet of the workbook and returns generated events.
// It applies scheduling rules from shiftTypes, uses schedule/slot overrides,
// and produces localized descriptions via the provided Localizer.
// lines is the LineMap returned by config.LoadConfig and is used to include
// source line numbers in warnings and errors; it may be nil.
func IterEvents(f *excelize.File, defaultAdvanceMinutes int, timezone string, shiftTypes map[string]model.ShiftType, seasons map[string]model.Season, exceptions map[string]model.Exception, lines map[string]int, loc *i18n.Localizer) ([]Event, error) {
	parsed, err := parseRows(f)
	if err != nil {
		return nil, err
	}

	locTZ, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, err
	}

	resolved, err := resolveRows(parsed, defaultAdvanceMinutes, shiftTypes, seasons, exceptions, lines, make(map[string]bool))
	if err != nil {
		return nil, err
	}

	return buildEvents(resolved, locTZ, loc), nil
}
