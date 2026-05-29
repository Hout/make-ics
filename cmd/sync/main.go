package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"github.com/jeroen/make-ics-go/pkg/caldav"
	"github.com/jeroen/make-ics-go/pkg/config"
	"github.com/jeroen/make-ics-go/pkg/i18n"
	"github.com/jeroen/make-ics-go/pkg/model"
	"github.com/jeroen/make-ics-go/pkg/pipeline"
)

const defaultAdvanceMinutes = 30

const defaultEnvPath = ".env"

type envLoadStats struct {
	Found   bool
	Loaded  int
	Skipped int
}

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
	from := fs.String("from", "", "Start date for the sync window (YYYY-MM-DD)")
	to := fs.String("to", "", "End date for the sync window (YYYY-MM-DD)")
	includePast := fs.Bool("include-past", false, "Sync past events too (default: future only)")
	dryRun := fs.Bool("dry-run", false, "Report changes without writing to the server")
	verbose := fs.Bool("verbose", false, "Enable verbose logging")
	fs.BoolVar(verbose, "v", false, "Enable verbose logging (alias)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		*input = fs.Arg(0)
	}

	start, err := parseDateBound(*from, false)
	if err != nil {
		return err
	}
	end, err := parseDateBound(*to, true)
	if err != nil {
		return err
	}
	if !start.IsZero() && !end.IsZero() && start.After(end) {
		return fmt.Errorf("invalid sync window: from %s is after to %s", *from, *to)
	}

	logf := func(format string, a ...any) {
		if *verbose {
			fmt.Fprintf(os.Stderr, "[verbose] "+format+"\n", a...)
		}
	}

	logf("starting sync: input=%q config=%q include_past=%t dry_run=%t from=%q to=%q", *input, *cfgPath, *includePast, *dryRun, *from, *to)

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

	dotEnvStats, err := loadDotEnv(defaultEnvPath)
	if err != nil {
		return fmt.Errorf("failed to load %s: %w", defaultEnvPath, err)
	}
	if dotEnvStats.Found {
		logf("loaded %s: set=%d skipped_existing=%d", defaultEnvPath, dotEnvStats.Loaded, dotEnvStats.Skipped)
	} else {
		logf("%s not found; using existing environment only", defaultEnvPath)
	}

	logf("credential env presence: %s=%t %s=%t ID=%t PASSWORD=%t HOST=%t",
		cfg.CalDAV.UsernameEnv,
		strings.TrimSpace(os.Getenv(cfg.CalDAV.UsernameEnv)) != "",
		cfg.CalDAV.PasswordEnv,
		strings.TrimSpace(os.Getenv(cfg.CalDAV.PasswordEnv)) != "",
		strings.TrimSpace(os.Getenv("ID")) != "",
		strings.TrimSpace(os.Getenv("PASSWORD")) != "",
		strings.TrimSpace(os.Getenv("HOST")) != "",
	)

	calDAVCfg, username, password, err := resolveCalDAVSettings(cfg.CalDAV)
	if err != nil {
		return err
	}
	logf("resolved CalDAV URL=%q username_len=%d password_len=%d", calDAVCfg.URL, len(username), len(password))

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
	connectStarted := time.Now()
	logf("connecting to CalDAV server: begin")
	client, err := caldav.New(ctx, calDAVCfg, username, password)
	if err != nil {
		return fmt.Errorf("caldav connect: %w", err)
	}
	logf("connecting to CalDAV server: done in %s", time.Since(connectStarted))

	opts := caldav.SyncOptions{
		IncludePast: *includePast,
		Start:       start,
		End:         end,
		DryRun:      *dryRun,
		Logf:        logf,
	}
	logf("syncing events: count=%d", len(events))
	result, err := client.Sync(ctx, events, opts)
	if err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}
	logf("sync completed: added=%d updated=%d deleted=%d unchanged=%d skipped=%d errors=%d",
		result.Added,
		result.Updated,
		result.Deleted,
		result.Unchanged,
		result.Skipped,
		len(result.Errors),
	)

	if *dryRun {
		fmt.Print("[dry-run] ")
	}
	fmt.Printf("added=%d updated=%d deleted=%d unchanged=%d skipped=%d\n",
		result.Added, result.Updated, result.Deleted, result.Unchanged, result.Skipped)

	if result.ExcludedFromDeletionScope > 0 {
		fmt.Fprintf(os.Stderr,
			"note: %d managed remote events were excluded from deletion scope (outside sync window/past policy)\n",
			result.ExcludedFromDeletionScope,
		)
	}

	if len(result.Errors) > 0 {
		fmt.Fprintf(os.Stderr, "%d error(s) during sync:\n", len(result.Errors))
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "  %v\n", e)
		}
		return fmt.Errorf("%d sync error(s)", len(result.Errors))
	}
	return nil
}

func parseDateBound(value string, endOfDay bool) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q, expected YYYY-MM-DD: %w", value, err)
	}
	if endOfDay {
		return parsed.Add(24*time.Hour - time.Nanosecond), nil
	}
	return parsed, nil
}

func loadDotEnv(path string) (envLoadStats, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return envLoadStats{}, nil
		}
		return envLoadStats{}, err
	}
	defer file.Close()

	stats := envLoadStats{Found: true}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, ok := parseEnvLine(scanner.Text())
		if !ok {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			stats.Skipped++
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return envLoadStats{}, fmt.Errorf("set %s: %w", key, err)
		}
		stats.Loaded++
	}

	if err := scanner.Err(); err != nil {
		return envLoadStats{}, err
	}

	return stats, nil
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
