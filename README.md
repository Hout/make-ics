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
    trips: 1
    trip_duration: 60
    first_shift_preparation_duration: 30
    last_shift_aftercare_duration: 30
    schedules:
      - seasons: [laagseizoen]
        slots:
          - weekdays: ["Sat", "Sun"]
            start_times:
              - times: ["11:15", "13:15", "15:15"]
      - seasons: [hoogseizoen]
        slots:
          - weekdays: ["Tue", "Wed", "Thu", "Fri", "Sat", "Sun"]
            start_times:
              - times: ["11:15", "13:15", "15:15"]
```

### Shift type fields

| Field                              | Description                                                                     |
| ---------------------------------- | ------------------------------------------------------------------------------- |
| `summary`                          | VEVENT `SUMMARY` (calendar title)                                               |
| `description`                      | Static text appended to the event description                                   |
| `trips`                            | Number of trips per departure (default 1)                                       |
| `trip_duration`                    | Duration of each trip in minutes (default 0)                                    |
| `break_duration`                   | Break between trips in minutes (default 0)                                      |
| `first_shift_preparation_duration` | Extra minutes before the first departure of the day                             |
| `first_shift_preparation_time`     | Absolute clock time (HH:MM) to start before the first departure                 |
| `first_shift_preparation_count`    | How many leading departures per day receive the first-shift advance (default 1) |
| `last_shift_aftercare_duration`    | Extra minutes added after the last departure of the day                         |
| `schedules`                        | Season-based schedule list (see below)                                          |

Duration formula: `trips × trip_duration + max(0, trips − 1) × break_duration`

When both `first_shift_preparation_time` and `first_shift_preparation_duration` are set (at different config levels), `first_shift_preparation_time` takes precedence and a warning is emitted.

### Seasons and exceptions

`seasons` are named date windows referenced by `schedules`. A season can contain multiple `{from, to}` ranges (inclusive).

`exceptions` remap specific calendar dates to a different weekday for schedule matching — useful for public holidays that follow a weekend timetable.

### Schedules, slots and start times

Each entry under `schedules` applies when the shift date falls within one of its `seasons`. Inside a schedule, `slots` narrow by weekday:

```yaml
schedules:
  - seasons: [laagseizoen]
    slots:
      - weekdays: ["Tue", "Wed", "Thu", "Fri"]
        first_shift_preparation_time: "9:15"
        first_shift_preparation_count: 2
        start_times:
          - times: ["10:20", "10:40", "11:00", "14:00", "14:20"]
          - times: ["14:40", "15:00"]
            trips: 2 # override trips for these start times only
```

`start_times` groups departure times that share the same trip parameters. Any field omitted inside a group inherits from the enclosing slot, then the shift type.

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
