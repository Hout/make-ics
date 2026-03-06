# make-ics

[🇳🇱 Nederlands](README.nl.md)

Converts a Dutch xlsx schedule (`report.xlsx`) into an ICS calendar file.

> **Note:** The built-in configuration currently covers HRM shifts only.
> Support for other shift types may be added later on request.

## Usage

```text
./make-ics-macos [OPTIONS] [report.xlsx]
```

| Option          | Default       | Description              |
| --------------- | ------------- | ------------------------ |
| `-c`, `-config` | `config.yaml` | Path to YAML config file |
| `-input`        | `report.xlsx` | Path to input xlsx file  |

The output file is written next to the input file with a `.ics`
extension (e.g. `report.ics`).

### Examples

```bash
# Quickest — positional argument, uses built-in config
./make-ics-macos report.xlsx

# External config override
./make-ics-macos -c my-config.yaml report.xlsx
```

## Config

No configuration is needed out of the box — a default `config.yaml`
is compiled into the binary and used automatically. To override it,
place a `config.yaml` next to the binary or pass `-c <path>`.

```yaml
timezone: Europe/Amsterdam
locale: nl_NL

shift_type:
  HRm_:
    summary: "Binnendieze HRM"
    description: "Binnendieze; Historische route Molenstraat"
    trips: 3
    trip_duration: 50 # minutes per trip
    break_duration: 30 # minutes between trips
    first_shift_advance: 30 # add minutes before first shift of the day
    last_shift_remains: 30 # add minutes after last shift of the day
    date_ranges:
      - from: 2026-04-01
        to: 2026-04-17
        start_times:
          - times: ["10:20", "14:40"]
            trips: 1
          - times: ["10:40", "11:00", "14:00", "14:20"]
```

### Shift type fields

| Field                 | Description                                     |
| --------------------- | ----------------------------------------------- |
| `summary`             | VEVENT `SUMMARY` (calendar title)               |
| `description`         | Static text appended to the event description   |
| `trips`               | Number of trips per shift (default 1)           |
| `trip_duration`       | Duration of each trip in minutes (default 0)    |
| `break_duration`      | Break between trips in minutes (default 0)      |
| `first_shift_advance` | Extra minutes before the first shift of the day |
| `last_shift_remains`  | Extra minutes after the last shift of the day   |
| `date_ranges`         | Period-specific overrides (see below)           |

Duration formula:
`trips × trip_duration + max(0, trips − 1) × break_duration`

### Date ranges

Each entry under `date_ranges` applies when the shift date falls within
`[from, to]` (inclusive). Fields set inside a range override the
top-level shift defaults for that period.

`start_times` groups start times by how many trips they use:

```yaml
start_times:
  - times: ["10:20", "14:40"]
    trips: 1 # these start times get 1 trip
  - times: ["10:40"] # no trips key → inherits shift/range default
```

## Web interface

A small HTTP server lets anyone upload an xlsx and download the resulting ICS
without needing to install anything locally.

```bash
go run ./cmd/web            # starts on http://localhost:8080
go run ./cmd/web -port 9000 # custom port
```

| Flag      | Default | Description                                             |
| --------- | ------- | ------------------------------------------------------- |
| `-port`   | `8080`  | TCP port to listen on                                   |
| `-config` | —       | Path to a config.yaml override (uses built-in if empty) |

Open `http://localhost:8080` in a browser, choose or drag your `.xlsx` file,
click **Convert & download**, and the `.ics` file is saved immediately.
All processing happens in memory — no files are written to disk.

The server is designed to run behind a reverse proxy (nginx, Caddy, …).

## Binaries

| File                 | Platform                    |
| -------------------- | --------------------------- |
| `make-ics-macos`     | macOS arm64 (Apple Silicon) |
| `make-ics.exe`       | Windows amd64               |
| `make-ics-web-macos` | macOS arm64 web server      |
| `make-ics-web.exe`   | Windows amd64 web server    |

## Building from source

Requires Go 1.24+.

```bash
# CLI
go run ./cmd/make-ics report.xlsx          # quick run
go build -o make-ics-macos ./cmd/make-ics  # local build

# Web server
go run ./cmd/web                           # quick run
go build -o make-ics-web-macos ./cmd/web   # local build
```

Cross-compile:

```bash
# CLI
GOOS=darwin  GOARCH=arm64 go build -o make-ics-macos ./cmd/make-ics
GOOS=windows GOARCH=amd64 go build -o make-ics.exe   ./cmd/make-ics

# Web server
GOOS=darwin  GOARCH=arm64 go build -o make-ics-web-macos ./cmd/web
GOOS=windows GOARCH=amd64 go build -o make-ics-web.exe   ./cmd/web
```

## Development

```bash
go test ./...   # run all tests
go vet ./...    # static analysis
```

Pre-commit hooks (go fmt, go build, go vet, go test, binary rebuild)
run automatically on `git commit` after installing:

```bash
pre-commit install
```
