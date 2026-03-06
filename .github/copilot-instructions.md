# Copilot Instructions

## Project Overview
CLI tool (`cmd/make-ics/main.go`) that converts a Dutch xlsx schedule (`report.xlsx`) to an ICS calendar file, driven by `config.yaml`.
The Python implementation has been archived to `archive/python/` for reference only.

## Go Standards

- **Version**: Go 1.24 — use standard idioms; no generics needed at current scope
- **Module**: `github.com/jeroen/make-ics-go`
- **Formatter**: `gofmt` / `goimports` (run via editor or `go fmt ./...`)
- **Vet**: `go vet ./...` — must be clean
- **Tests**: `go test ./...` — all must pass before committing

## Code Style

- Exported names have doc comments
- Unexported constants use `camelCase` (not `ALL_CAPS`)
- Keep cyclomatic complexity low — extract helpers rather than nesting
- Use `time.Time` arithmetic for all date/time calculations; never manual int-based modular math
- Hoist compiled regexes to package-level `var` blocks
- Diagnostic/skip messages go to `os.Stderr`, not `os.Stdout`
- Prefer early returns over deeply nested `if` blocks

## Package Layout

| Package | Responsibility |
|---|---|
| `cmd/make-ics` | CLI entry point (`Run` is testable, `main` just delegates) |
| `pkg/config` | Load + validate `config.yaml` |
| `pkg/model` | Shared types (`Config`, `ShiftType`, `DateRange`, …) |
| `pkg/parser` | Parse Dutch date strings and time strings from xlsx cells |
| `pkg/range` | `FindDateRange` — date-range resolution with `StartTimeGroup` merging |
| `pkg/schedule` | Duration / trip helpers and `BuildProgram` description builder |
| `pkg/pipeline` | `IterEvents` — orchestrates parsing, scheduling, and event assembly |
| `pkg/ics` | `WriteCalendar` — serialises events to an ICS file |
| `pkg/i18n` | Thin wrapper around go-i18n v2 |

## Project Conventions

- Config structure: `shift_type.<code>` with optional `date_ranges` list
- `FindDateRange` returns a `*ResolvedRange` (nil = no match); callers treat it as authoritative for per-shift overrides
- Duration formula: `trips × trip_duration + max(0, trips−1) × break_duration`
- `IterEvents` pre-collects all rows before building events, to support first/last-shift detection per `(code, date)` pair
- Unknown shift codes use a zero-value `ShiftType{}` — all helpers return safe defaults

## Testing

- **TDD**: write the failing test before implementing the feature
- Test files live next to the package they test (`*_test.go`)
- Prefer pure function tests; avoid mocks unless I/O cannot be avoided
- Cover edge cases: missing config keys, date-range boundaries, first/last shift logic
- Use `t.Run` and table-driven tests for data-driven cases

## Dependencies (Go)

- `github.com/xuri/excelize/v2` — read xlsx
- `github.com/arran4/golang-ical` — build ICS
- `gopkg.in/yaml.v3` — load config
- `github.com/nicksnyder/go-i18n/v2` — i18n
- `github.com/google/uuid` — event UIDs

## Shell / Terminal

- **Never use heredocs** (`<<EOF ... EOF`) in terminal commands — they corrupt the shell session
- **Never use `/tmp` for temporary storage** — use `.scratch/` in the workspace root instead (already in `.gitignore`)
- Use `go run ./cmd/make-ics` for quick smoke-tests; use `go build -o make-ics ./cmd/make-ics` for a binary
- Cross-compile with `GOOS=windows GOARCH=amd64 go build -o make-ics.exe ./cmd/make-ics`

## Pre-commit Hooks

Config: `.pre-commit-config.yaml` — hooks run on Go files only (`types: [go]`).

1. `go build ./...` — must compile
2. `go vet ./...` — must be clean
3. `go test ./...` — all tests must pass

Reinstall after cloning: `pre-commit install`

## Completing a Feature

After every new or changed feature, run these steps before considering the work done:

1. **Build** — `go build ./...` (must succeed)
2. **Vet** — `go vet ./...` (must be clean)
3. **Test** — `go test ./...` (all must pass)
4. **Smoke-test** — `go run ./cmd/make-ics report.xlsx` and verify output
5. **Commit** — `git add -A && git commit -m "<concise description>"` (pre-commit hooks re-run steps 1–3 automatically)
