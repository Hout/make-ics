package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/xuri/excelize/v2"

	"github.com/jeroen/make-ics-go/pkg/caldav"
	"github.com/jeroen/make-ics-go/pkg/config"
	"github.com/jeroen/make-ics-go/pkg/i18n"
	"github.com/jeroen/make-ics-go/pkg/model"
	"github.com/jeroen/make-ics-go/pkg/pipeline"
)

const defaultAdvanceMinutes = 30

const defaultEnvPath = ".env"

func main() {
	if err := Run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

// Run executes the sync using the provided args.
func Run(args []string) error {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	input := fs.String("input", "report.xlsx", "Path to the input xlsx file")
	cfgPath := fs.String("config", "config.yaml", "Path to YAML config file")
	fs.StringVar(cfgPath, "c", *cfgPath, "Path to YAML config file (alias)")
	includePast := fs.Bool("include-past", false, "Sync past events too (default: future only)")
	dryRun := fs.Bool("dry-run", false, "Report changes without writing to the server")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		*input = fs.Arg(0)
	}

	cfg, lines, err := config.LoadConfig(*cfgPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if err := config.ValidateConfig(cfg, *cfgPath, lines); err != nil {
		return err
	}
	if cfg.CalDAV.URL == "" {
		return fmt.Errorf("no caldav block in config — add caldav.url, caldav.username_env, caldav.password_env, and caldav.calendar_display_name")
	}

	if err := loadDotEnv(defaultEnvPath); err != nil {
		return fmt.Errorf("failed to load %s: %w", defaultEnvPath, err)
	}

	calDAVCfg, username, password, err := resolveCalDAVSettings(cfg.CalDAV)
	if err != nil {
		return err
	}

	loc, err := i18n.NewLocalizer(cfg.Locale)
	if err != nil {
		return fmt.Errorf("failed to initialize i18n: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Reading %s …\n", *input)
	wb, err := excelize.OpenFile(*input)
	if err != nil {
		return fmt.Errorf("failed to open workbook: %w", err)
	}
	defer wb.Close()

	events, warnings, err := pipeline.IterEvents(wb, defaultAdvanceMinutes, cfg.Timezone, cfg.ShiftType, cfg.Seasons, cfg.Exceptions, lines, loc)
	if err != nil {
		return fmt.Errorf("failed to build events: %w", err)
	}
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "  %s\n", w)
	}
	fmt.Fprintf(os.Stderr, "%d events parsed\n", len(events))

	ctx := context.Background()
	client, err := caldav.New(ctx, calDAVCfg, username, password)
	if err != nil {
		return fmt.Errorf("caldav connect: %w", err)
	}

	opts := caldav.SyncOptions{
		IncludePast: *includePast,
		DryRun:      *dryRun,
	}
	result, err := client.Sync(ctx, events, opts)
	if err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	if *dryRun {
		fmt.Print("[dry-run] ")
	}
	fmt.Printf("added=%d updated=%d deleted=%d unchanged=%d skipped=%d\n",
		result.Added, result.Updated, result.Deleted, result.Unchanged, result.Skipped)

	if len(result.Errors) > 0 {
		fmt.Fprintf(os.Stderr, "%d error(s) during sync:\n", len(result.Errors))
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "  %v\n", e)
		}
		return fmt.Errorf("%d sync error(s)", len(result.Errors))
	}
	return nil
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, ok := parseEnvLine(scanner.Text())
		if !ok {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set %s: %w", key, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func parseEnvLine(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", "", false
	}
	if strings.HasPrefix(trimmed, "export ") {
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "export "))
	}

	key, value, found := strings.Cut(trimmed, "=")
	if !found {
		return "", "", false
	}

	key = strings.TrimSpace(key)
	if key == "" {
		return "", "", false
	}

	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) || (strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
			value = value[1 : len(value)-1]
		}
	}

	return key, value, true
}

func resolveCalDAVSettings(cfg model.CalDAVConfig) (model.CalDAVConfig, string, string, error) {
	resolved := cfg

	if host := strings.TrimSpace(os.Getenv("HOST")); host != "" {
		resolved.URL = host
	}

	if _, err := url.ParseRequestURI(resolved.URL); err != nil {
		return resolved, "", "", fmt.Errorf("caldav host/URL %q is invalid: %w", resolved.URL, err)
	}

	username := strings.TrimSpace(os.Getenv(cfg.UsernameEnv))
	if username == "" {
		username = strings.TrimSpace(os.Getenv("ID"))
	}

	password := strings.TrimSpace(os.Getenv(cfg.PasswordEnv))
	if password == "" {
		password = strings.TrimSpace(os.Getenv("PASSWORD"))
	}

	if username == "" {
		return resolved, "", "", fmt.Errorf("environment variable %s (caldav.username_env) is not set and fallback ID is empty", cfg.UsernameEnv)
	}
	if password == "" {
		return resolved, "", "", fmt.Errorf("environment variable %s (caldav.password_env) is not set and fallback PASSWORD is empty", cfg.PasswordEnv)
	}

	return resolved, username, password, nil
}
