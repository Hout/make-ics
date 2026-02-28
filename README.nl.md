# make-ics

[🇬🇧 English](README.md)

Zet een Nederlandse xlsx-dienstregelingexport (`report.xlsx`) om naar
een ICS-kalenderbestand.

## Gebruik

```text
./make-ics-macos [OPTIES] [report.xlsx]
```

| Optie           | Standaard     | Omschrijving                          |
| --------------- | ------------- | ------------------------------------- |
| `-c`, `-config` | `config.yaml` | Pad naar het YAML-configuratiebestand |
| `-input`        | `report.xlsx` | Pad naar het xlsx-invoerbestand       |

Het uitvoerbestand wordt naast het invoerbestand opgeslagen met de
extensie `.ics` (bijv. `report.ics`).

### Voorbeelden

```bash
# Snel starten — positioneel argument, gebruikt ingebouwde configuratie
./make-ics-macos report.xlsx

# Eigen configuratiebestand opgeven
./make-ics-macos -c mijn-config.yaml report.xlsx
```

## Configuratie

Een standaard `config.yaml` is ingebakken in het programma en wordt
automatisch gebruikt als er geen extern bestand wordt gevonden. Om dit
te overschrijven, plaats je een `config.yaml` naast het programma of
geef je `-c <pad>` op.

```yaml
timezone: Europe/Amsterdam
locale: nl_NL

shift_type:
  HRm_:
    summary: "Binnendieze HRM"
    description: "Binnendieze; Historische route Molenstraat"
    trips: 3
    trip_duration: 50       # minuten per rit
    break_duration: 30      # minuten pauze tussen ritten
    first_shift_advance: 30 # extra minuten vóór de eerste dienst
    last_shift_remains: 30  # extra minuten ná de laatste dienst
    date_ranges:
      - from: 2026-04-01
        to:   2026-04-17
        start_times:
          - times: ["10:20", "14:40"]
            trips: 1
          - times: ["10:40", "11:00", "14:00", "14:20"]
```

### Velden per dienstsoort

| Veld                  | Omschrijving                                     |
| --------------------- | ------------------------------------------------ |
| `summary`             | VEVENT `SUMMARY` (calendertitel)                 |
| `description`         | Vaste tekst toegevoegd aan de beschrijving       |
| `trips`               | Aantal ritten per dienst (standaard 1)           |
| `trip_duration`       | Duur van elke rit in minuten (standaard 0)       |
| `break_duration`      | Pauze tussen ritten in minuten (standaard 0)     |
| `first_shift_advance` | Extra minuten vóór eerste dienst van de dag      |
| `last_shift_remains`  | Extra minuten ná laatste dienst van de dag       |
| `date_ranges`         | Periodespecifieke overschrijvingen (zie onder)   |

Duurformule:
`ritten × ritduur + max(0, ritten − 1) × pauze`

### Datumbereiken

Elke invoer onder `date_ranges` geldt wanneer de dienstdatum binnen
`[from, to]` (inclusief) valt. Velden in een bereik overschrijven de
standaardwaarden van de dienstsoort voor die periode.

Met `start_times` geef je per begintijd aan hoeveel ritten er zijn:

```yaml
start_times:
  - times: ["10:20", "14:40"]
    trips: 1          # deze begintijden krijgen 1 rit
  - times: ["10:40"]  # geen trips → erft van dienst/bereik
```

## Programmabestanden

| Bestand          | Platform                    |
| ---------------- | --------------------------- |
| `make-ics-macos` | macOS arm64 (Apple Silicon) |
| `make-ics.exe`   | Windows amd64               |

## Bouwen vanuit broncode

Vereist Go 1.24+.

```bash
go run ./cmd/make-ics report.xlsx          # snel uitvoeren
go build -o make-ics-macos ./cmd/make-ics  # lokaal bouwen
```

Kruiscompilatie:

```bash
GOOS=darwin  GOARCH=arm64 go build -o make-ics-macos ./cmd/make-ics
GOOS=windows GOARCH=amd64 go build -o make-ics.exe   ./cmd/make-ics
```

## Ontwikkeling

```bash
go test ./...   # alle tests uitvoeren
go vet ./...    # statische analyse
```

Pre-commit hooks (go fmt, go build, go vet, go test, binaries bouwen)
worden automatisch uitgevoerd bij `git commit` na installatie:

```bash
pre-commit install
```
