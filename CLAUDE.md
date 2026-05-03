# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

CLI tool and HTTP server that convert a Dutch xlsx schedule (`report.xlsx`) to an ICS calendar file, driven by `config.yaml`. The Python implementation has been archived to `archive/python/` for reference only.

- **Module**: `github.com/jeroen/make-ics-go`
- **Go version**: 1.24+

## Commands

```bash
go test ./...                            # run all tests
go test ./pkg/pipeline/...              # run a single package's tests
go vet ./...                             # static analysis (must be clean)
go build ./...                           # build all packages
go run ./cmd/make-ics report.xlsx        # smoke-test CLI
go run ./cmd/web                         # start web server on :8080
go build -o make-ics-macos ./cmd/make-ics
GOOS=windows GOARCH=amd64 go build -o make-ics.exe ./cmd/make-ics
```

Pre-commit hooks run `go build`, `go vet`, and `go test` automatically. Install once after cloning:

```bash
pre-commit install
```

## Architecture

Data flow: `xlsx` → `pipeline.IterEvents` → `[]pipeline.Event` → `ics.WriteCalendar`

### Package layout

| Package               | Responsibility                                                                                                                                                                         |
| --------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `cmd/make-ics`        | CLI entry point; `Run` is testable, `main` delegates                                                                                                                                   |
| `cmd/web`             | HTTP server; multipart upload → ICS download, all in memory                                                                                                                            |
| `cmd/list-shifts`     | Debug tool that lists parsed shifts                                                                                                                                                    |
| `internal/defaultcfg` | Embeds `config.yaml` into the binary                                                                                                                                                   |
| `pkg/config`          | Load, parse, and validate `config.yaml`; returns a `LineMap` for YAML line numbers used in error messages                                                                              |
| `pkg/model`           | Shared types: `Config`, `ShiftType`, `Schedule`, `Slot`, `Season`, `Exception`                                                                                                         |
| `pkg/parser`          | Parse Dutch date strings and `HH:MM` time strings from xlsx cells                                                                                                                      |
| `pkg/range`           | `FindSchedule` — resolves the active `Schedule`/`Slot` for a given date and start-time; `FirstScheduledTimes` — returns the N earliest scheduled start times for first-shift detection |
| `pkg/schedule`        | Duration helpers (`GetShiftDurationMinutes`, `GetLastShiftAftercare`, …) and `BuildProgram` (localized description builder)                                                            |
| `pkg/pipeline`        | `IterEvents` — orchestrates parsing, scheduling, and event assembly                                                                                                                    |
| `pkg/ics`             | `WriteCalendar` / `WriteCalendarWriter` — serialises events to ICS                                                                                                                     |
| `pkg/i18n`            | Thin wrapper around go-i18n v2                                                                                                                                                         |

### Config hierarchy

`ShiftType` → `Schedule` (matched by season) → `Slot` (matched by weekday) → `StartTimeGroup` (matched by start time). Fields at a more specific level override the parent. `FindSchedule` returns a `*ResolvedRange` (nil = no match).

**Duration formula**: `trips × trip_duration + max(0, trips−1) × break_duration`

### First/last shift logic

`IterEvents` pre-collects all rows before building events so it can detect the first and last shift for each `(code, date)` pair. The first-shift advance can be set via `first_shift_preparation_time` (absolute clock time) or `first_shift_preparation_duration` (minutes); non-first shifts use `defaultAdvanceMinutes`. If both `first_shift_preparation_time` and `first_shift_preparation_duration` are set at different config levels a warning is emitted and `first_shift_preparation_time` takes precedence.

### Exceptions and seasons

`exceptions` remaps specific calendar dates to a different weekday for schedule matching (e.g. public holidays treated as Sunday). `seasons` are named collections of `DateRange` windows referenced by `Schedule.seasons`.

## Code style

- Exported names have doc comments; unexported constants use `camelCase`
- Diagnostic/skip messages go to `os.Stderr`; output to `os.Stdout`
- Prefer early returns; keep cyclomatic complexity low
- Hoist compiled regexes to package-level `var` blocks
- Use `time.Time` arithmetic; never manual int-based modular math
- Unknown shift codes use a zero-value `ShiftType{}` — all helpers return safe defaults

## Testing

- TDD: write the failing test first
- Table-driven tests with `t.Run`; test files live next to the package (`*_test.go`)
- Cover edge cases: missing config keys, date-range boundaries, first/last shift logic

## Shell notes

- **Never use heredocs** in terminal commands — they corrupt the shell session
- **Never use `/tmp`** — use `.scratch/` in the workspace root instead (gitignored)

## Feature completion checklist

1. `go build ./...` — must succeed
2. `go vet ./...` — must be clean
3. `go test ./...` — all must pass
4. `go run ./cmd/make-ics report.xlsx` — verify output
5. Commit (pre-commit hooks re-run steps 1–3)
