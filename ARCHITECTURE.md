# Architecture

`make-ics` converts a Dutch xlsx schedule (`report.xlsx`) into an ICS calendar,
driven by a `config.yaml`. It ships as both a CLI and a stateless HTTP server.

- **Module**: `github.com/jeroen/make-ics-go`
- **Go version**: 1.24+
- **Deployment**: single static binary (CLI) or container (web)

## Data flow

```
xlsx file ──► config.LoadConfig ──► model.Config
                  │
                  ▼
        pipeline.IterEvents ──► []pipeline.Event ──► ics.WriteCalendar ──► .ics
                  ▲
                  └── parser, range, schedule, i18n
```

1. `pkg/config` loads and validates `config.yaml`, returning a typed `model.Config`
   plus a `LineMap` of YAML line numbers used in error messages.
2. `pkg/pipeline.IterEvents` reads the first sheet of the workbook, iterates rows,
   resolves the active schedule for each `(date, start-time)` and assembles events.
3. `pkg/ics.WriteCalendar` (or `WriteCalendarWriter`) serialises the events to ICS.

Event UIDs are deterministic: a UUID v5 derived from a fixed namespace so the
same shift always produces the same UID across runs.

## Entry points

| Binary            | Purpose                                                                                       |
| ----------------- | --------------------------------------------------------------------------------------------- |
| `cmd/make-ics`    | CLI. `Run(args, stdout, stderr)` is testable; `main` is a thin shim.                          |
| `cmd/web`         | HTTP server. Multipart upload of xlsx → ICS download. Fully in-memory; no disk I/O, no state. |
| `cmd/list-shifts` | Debug tool that prints parsed shifts.                                                         |

The default `config.yaml` is embedded into both binaries via `internal/defaultcfg`,
so they run without an external config file.

## Package layout

| Package               | Responsibility                                                                                                                       |
| --------------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| `cmd/make-ics`        | CLI entry point.                                                                                                                     |
| `cmd/web`             | Stateless HTTP server (multipart upload → ICS response).                                                                             |
| `cmd/list-shifts`     | Diagnostic listing of parsed shifts.                                                                                                 |
| `internal/defaultcfg` | Embeds `config.yaml` into the binary.                                                                                                |
| `pkg/config`          | Loads, parses, and validates `config.yaml`; produces a `LineMap`.                                                                    |
| `pkg/model`           | Shared types: `Config`, `ShiftType`, `Schedule`, `Slot`, `Season`, `Exception`.                                                      |
| `pkg/parser`          | Dutch date strings and `HH:MM` time parsing from xlsx cells.                                                                         |
| `pkg/range`           | `FindSchedule` resolves the active `Schedule`/`Slot` for a date+time; `FirstScheduledTimes` returns the N earliest scheduled starts. |
| `pkg/schedule`        | Duration helpers and `BuildProgram` (localized description builder).                                                                 |
| `pkg/pipeline`        | `IterEvents` — orchestrates parsing, scheduling and event assembly.                                                                  |
| `pkg/ics`             | `WriteCalendar` / `WriteCalendarWriter`.                                                                                             |
| `pkg/i18n`            | Thin wrapper around `go-i18n` v2.                                                                                                    |

`internal/` is reserved for non-reusable helpers; everything intended to be
imported from outside lives under `pkg/`.

## Configuration model

Configuration is hierarchical, with each level overriding values from its parent:

```
ShiftType
  └─ Schedule       (matched by season)
      └─ Slot       (matched by weekday)
          └─ StartTimeGroup  (matched by start time)
```

`range.FindSchedule` walks this tree and returns a `*ResolvedRange`
(`nil` if no rule matches the row).

### Duration formula

```
duration = trips × trip_duration + max(0, trips − 1) × break_duration
```

### Seasons and exceptions

- `seasons` are named lists of `DateRange` windows; `Schedule.seasons` references
  them by name to bind a schedule to specific calendar windows.
- `exceptions` remap individual calendar dates onto a different weekday for
  schedule matching (e.g. a public holiday treated as Sunday).

### First / last shift logic

`IterEvents` pre-collects all rows so it can detect the first and last shift
per `(code, date)` pair. The first-shift advance is resolved in this priority order:

1. `first_shift_preparation_time` — absolute clock time
2. `first_shift_preparation_duration` — minutes
3. `defaultAdvanceMinutes` — CLI/server default when neither is set

Non-first shifts always use `defaultAdvanceMinutes`. If both
`first_shift_preparation_time` and `first_shift_preparation_duration` are set
at different config levels, a warning is emitted to `os.Stderr` and
`first_shift_preparation_time` takes precedence.

Unknown shift codes resolve to a zero-value `ShiftType{}`; all helpers return
safe defaults so a single bad code never aborts a run.

## Web server

`cmd/web` exposes:

- `GET /` — upload form
- `POST /convert` — accepts multipart `file=<xlsx>`, returns `text/calendar`

It reuses the embedded default config and shares the same pipeline as the CLI.
No request data is persisted; the handler operates on `io.Reader`/`io.Writer`
buffers. A Caddy reverse-proxy config is checked into the repo root.

## Internationalisation

`pkg/i18n` wraps `go-i18n` v2. Locale message bundles drive the localized
event descriptions produced by `schedule.BuildProgram`. The CLI accepts a
locale flag; the web server reads `Accept-Language`.

## Error handling and diagnostics

- Diagnostic and skip messages → `os.Stderr`.
- Successful program output → `os.Stdout`.
- Config errors carry YAML line numbers via `LineMap` to help users locate problems.
- The pipeline prefers logging-and-skipping over hard failure for per-row issues,
  so one malformed row never aborts an entire workbook.

## Code style conventions

- Exported identifiers carry doc comments; unexported constants use `camelCase`.
- Early returns; low cyclomatic complexity.
- Compiled regexes are hoisted to package-level `var` blocks.
- Date and time math uses `time.Time` arithmetic; never manual int modular math.

## Testing

- TDD: failing test first.
- Table-driven tests with `t.Run`; `*_test.go` lives next to the package.
- Edge cases that must be covered: missing config keys, date-range boundaries,
  first/last shift logic, unknown shift codes, exception remapping.

## Build and verification

```bash
go build ./...
go vet ./...
go test ./...
go run ./cmd/make-ics report.xlsx     # CLI smoke test
go run ./cmd/web                       # web smoke test on :8080
```

Cross-compilation targets:

```bash
go build -o make-ics-macos ./cmd/make-ics
GOOS=windows GOARCH=amd64 go build -o make-ics.exe ./cmd/make-ics
```

Pre-commit hooks re-run `go build`, `go vet`, and `go test`.
