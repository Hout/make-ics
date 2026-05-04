# make-ics

[🇳🇱 Nederlands](README.nl.md)

Converts a Dutch xlsx schedule (`report.xlsx`) into an ICS calendar file.

## Usage

```text
./make-ics-macos [OPTIONS] [report.xlsx]
```

| Option          | Default       | Description              |
| --------------- | ------------- | ------------------------ |
| `-c`, `-config` | `config.yaml` | Path to YAML config file |
| `-input`        | `report.xlsx` | Path to input xlsx file  |

The output file is written next to the input file with a `.ics` extension (e.g. `report.ics`).

### Examples

```bash
# Quickest — positional argument, uses built-in config
./make-ics-macos report.xlsx

# External config override
./make-ics-macos -c my-config.yaml report.xlsx
```

## Config

No configuration is needed out of the box — a default `config.yaml` is compiled into the binary and used automatically. To override it, place a `config.yaml` next to the binary or pass `-c <path>`.

```yaml
timezone: Europe/Amsterdam
locale: nl_NL

exceptions:
  2026-04-06:
    description: "Pasen"
    weekday: "Sun" # treat this date as Sunday for schedule matching

seasons:
  laagseizoen:
    - { from: 2026-04-01, to: 2026-06-28 }
  hoogseizoen:
    - { from: 2026-06-29, to: 2026-08-30 }

shift_type:
  HRv_:
    summary: "Binnendieze HRV"
    description: "Binnendieze; Historische route Voldersgat"
    season_schedules:
      - seasons: [laagseizoen]
        day_schedules:
          - weekdays: ["Sat", "Sun"]
            shifts:
              "1": { trips: ["13:15"], arrive: "12:45", leave: "14:15" }
              "2": { trips: ["15:15"], arrive: "14:45", leave: "16:15" }
      - seasons: [hoogseizoen]
        day_schedules:
          - weekdays: ["Tue", "Wed", "Thu", "Fri", "Sat", "Sun"]
            shifts:
              "1": { trips: ["11:15"], arrive: "10:45", leave: "12:15" }
              "2": { trips: ["13:15"], arrive: "12:45", leave: "14:15" }
              "3": { trips: ["15:15"], arrive: "14:45", leave: "16:15" }
```

### Shift type fields

| Field              | Description                                    |
| ------------------ | ---------------------------------------------- |
| `summary`          | VEVENT `SUMMARY` (calendar title)              |
| `description`      | Static text appended to the event description  |
| `season_schedules` | Season-based schedule list (see below)         |

### Shift fields

Each entry in a `shifts` map defines the calendar event for one shift number as it appears in the xlsx:

| Field        | Description                                                                            |
| ------------ | -------------------------------------------------------------------------------------- |
| `trips`      | List of departure times (`HH:MM`) — one per trip                                       |
| `trip_times` | Alternative to `trips`: list of `{start: "HH:MM", duration: <minutes>}` structs        |
| `arrive`     | Fixed clock time (HH:MM) the event starts; defaults to first departure − advance time  |
| `leave`      | Fixed clock time (HH:MM) the event ends; required for single-trip, or computed from last trip end for multi-trip |

### Seasons and exceptions

`seasons` are named date windows referenced by `schedules`. A season can contain multiple `{from, to}` ranges (inclusive).

`exceptions` remap specific calendar dates to a different weekday for schedule matching — useful for public holidays that follow a weekend timetable.

### Schedules, day schedules and shifts

Each entry under `season_schedules` applies when the shift date falls within one of its `seasons`. Inside a schedule, `day_schedules` narrow by weekday:

```yaml
season_schedules:
  - seasons: [laagseizoen]
    day_schedules:
      - weekdays: ["Tue", "Wed", "Thu", "Fri"]
        shifts:
          "1":
            trips: ["10:20", "11:40", "13:00"]
            arrive: "09:15"
            leave: "14:20"
          "2":
            trips: ["10:40", "12:00", "13:20"]
            arrive: "09:15"
            leave: "15:00"
```

`shifts` maps shift numbers (as they appear in the xlsx) to event definitions. `arrive` and `leave` set the calendar event boundaries. `leave` is required for single-trip shifts; for multi-trip shifts it can be omitted and is computed from the last trip's end time (requires explicit `trip_times` with a `duration`).

## Web interface

A small HTTP server lets anyone upload an xlsx and download the resulting ICS without installing anything locally.

```bash
go run ./cmd/web            # starts on http://localhost:8080
go run ./cmd/web -port 9000 # custom port
```

| Flag      | Default | Description                                             |
| --------- | ------- | ------------------------------------------------------- |
| `-port`   | `8080`  | TCP port to listen on                                   |
| `-config` | —       | Path to a config.yaml override (uses built-in if empty) |

Open `http://localhost:8080` in a browser, choose your `.xlsx` file, click **Convert & download**, and the `.ics` file is saved immediately. All processing happens in memory — no files are written to disk.

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

The repo includes a [Nix flake](flake.nix) and [direnv](https://direnv.net/) config. With both installed, `cd` into the directory and run `direnv allow` once — Go will be on your PATH automatically.

```bash
go test ./...   # run all tests
go vet ./...    # static analysis
```

Pre-commit hooks (go fmt, go build, go vet, go test, binary rebuild) run automatically on `git commit` after installing:

```bash
pre-commit install
```
