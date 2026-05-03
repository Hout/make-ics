package pipeline

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/jeroen/make-ics-go/pkg/parser"
)

type parsedRow struct {
	Code string
	Date time.Time
	Hour int
	Min  int
}

func parseRows(f *excelize.File) ([]parsedRow, error) {
	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, err
	}
	var parsed []parsedRow
	for _, r := range rows {
		if !parser.IsDataRow(r) {
			continue
		}
		dateStr := r[0]
		code := "Afspraak"
		if len(r) > 1 && strings.TrimSpace(r[1]) != "" {
			code = strings.TrimSpace(r[1])
		}
		tdate, err := parser.ParseDutchDate(dateStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [SKIP] Could not parse date %q: %v\n", dateStr, err)
			continue
		}
		h, m, err := parser.ParseTime(r[2])
		if err != nil {
			fmt.Fprintf(os.Stderr, "  [SKIP] Could not parse time %q: %v\n", r[2], err)
			continue
		}
		parsed = append(parsed, parsedRow{Code: code, Date: tdate, Hour: h, Min: m})
	}
	return parsed, nil
}
