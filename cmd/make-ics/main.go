package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/xuri/excelize/v2"

	"github.com/jeroen/make-ics-go/pkg/config"
	"github.com/jeroen/make-ics-go/pkg/i18n"
	"github.com/jeroen/make-ics-go/pkg/ics"
	"github.com/jeroen/make-ics-go/pkg/pipeline"
)

const defaultAdvanceMinutes = 30

func main() {
	// delegate to Run for testability
	if err := Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

// Run executes the command using the provided args (excluding program name).
// localesDir is the directory containing *.json i18n bundles; pass "" to use
// the default "locales" path relative to cwd.
// It returns an error instead of calling os.Exit so tests can call it safely.
func Run(args []string, localesDir ...string) error {
	dir := "locales"
	if len(localesDir) > 0 && localesDir[0] != "" {
		dir = localesDir[0]
	}
	fs := flag.NewFlagSet("make-ics", flag.ContinueOnError)
	input := fs.String("input", "report.xlsx", "Path to the input xlsx file")
	cfgPath := fs.String("config", "config.yaml", "Path to YAML config file")
	// -c is an alias for -config; both write to the same pointer so the last one wins.
	fs.StringVar(cfgPath, "c", *cfgPath, "Path to YAML config file (alias)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	// If a positional input is provided, override
	if fs.NArg() > 0 {
		*input = fs.Arg(0)
	}

	// load config
	cfg, err := config.LoadConfig(*cfgPath)
	if err != nil {
		return fmt.Errorf("Failed to load config: %w", err)
	}
	if err := config.ValidateConfig(cfg, *cfgPath); err != nil {
		return err
	}

	// i18n
	loc, err := i18n.NewLocalizer(dir, cfg.Locale)
	if err != nil {
		return fmt.Errorf("Failed to initialize i18n: %w", err)
	}

	fmt.Println(loc.T("Reading {path} …", map[string]interface{}{"path": *input}))

	wb, err := excelize.OpenFile(*input)
	if err != nil {
		return fmt.Errorf("Failed to open workbook: %w", err)
	}
	defer wb.Close()

	events, err := pipeline.IterEvents(wb, defaultAdvanceMinutes, cfg.Timezone, cfg.ShiftType, loc)
	if err != nil {
		return fmt.Errorf("Failed to build events: %w", err)
	}

	icsPath := (*input)[:len(*input)-len(filepath.Ext(*input))] + ".ics"
	if err := ics.WriteCalendar(filepath.Base(*input), events, icsPath); err != nil {
		return fmt.Errorf("Failed to write ICS: %w", err)
	}

	fmt.Println(loc.T("Total events written: {count}", map[string]interface{}{"count": len(events)}))
	fmt.Println(loc.T("Written to {path}", map[string]interface{}{"path": icsPath}))
	return nil
}
