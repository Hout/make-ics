# make-ics

[🇬🇧 English](README.md)

Zet een Nederlandse xlsx-dienstregelingexport (`report.xlsx`) om naar een ICS-kalenderbestand.

## Gebruik

```text
./make-ics-macos [OPTIES] [report.xlsx]
```

| Optie           | Standaard     | Omschrijving                          |
| --------------- | ------------- | ------------------------------------- |
| `-c`, `-config` | `config.yaml` | Pad naar het YAML-configuratiebestand |
| `-input`        | `report.xlsx` | Pad naar het xlsx-invoerbestand       |

Het uitvoerbestand wordt naast het invoerbestand opgeslagen met de extensie `.ics` (bijv. `report.ics`).

### Voorbeelden

```bash
# Snel starten — positioneel argument, gebruikt ingebouwde configuratie
./make-ics-macos report.xlsx

# Eigen configuratiebestand opgeven
./make-ics-macos -c mijn-config.yaml report.xlsx
```

## Configuratie

Standaard is er geen configuratie nodig — een standaard `config.yaml` is ingebakken in het programma en wordt automatisch gebruikt. Om dit te overschrijven, plaats je een `config.yaml` naast het programma of geef je `-c <pad>` op.

```yaml
timezone: Europe/Amsterdam
locale: nl_NL

exceptions:
  2026-04-06:
    description: "Pasen"
    weekday: "Sun" # behandel deze datum als zondag bij het zoeken naar het rooster

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

### Velden per dienstsoort

| Veld                               | Omschrijving                                                                          |
| ---------------------------------- | ------------------------------------------------------------------------------------- |
| `summary`                          | VEVENT `SUMMARY` (calendertitel)                                                      |
| `description`                      | Vaste tekst toegevoegd aan de beschrijving                                            |
| `trips`                            | Aantal ritten per vertrek (standaard 1)                                               |
| `trip_duration`                    | Duur van elke rit in minuten (standaard 0)                                            |
| `break_duration`                   | Pauze tussen ritten in minuten (standaard 0)                                          |
| `first_shift_preparation_duration` | Extra minuten vóór het eerste vertrek van de dag                                      |
| `first_shift_preparation_time`     | Absolute aankomsttijd (HH:MM) vóór het eerste vertrek                                 |
| `first_shift_preparation_count`    | Hoeveel vroege vertrekken per dag de eerste-dienst-aankomsttijd krijgen (standaard 1) |
| `last_shift_aftercare_duration`    | Extra minuten ná het laatste vertrek van de dag                                       |
| `schedules`                        | Seizoensgebonden roosterlijst (zie hieronder)                                         |

Duurformule: `ritten × ritduur + max(0, ritten − 1) × pauze`

Als zowel `first_shift_preparation_time` als `first_shift_preparation_duration` zijn ingesteld (op verschillende configuratieniveaus), heeft `first_shift_preparation_time` voorrang en wordt er een waarschuwing getoond.

### Seizoenen en uitzonderingen

`seasons` zijn benoemde datumvensters waarnaar `schedules` verwijzen. Een seizoen kan meerdere `{from, to}`-bereiken bevatten (inclusief grenzen).

`exceptions` koppelen specifieke kalenderdatums aan een andere weekdag voor het zoeken naar het rooster — handig voor feestdagen die een weekendrooster volgen.

### Roosters, slots en begintijden

Elke invoer onder `schedules` geldt wanneer de dienstdatum binnen een van de bijbehorende `seasons` valt. Binnen een rooster beperken `slots` het tot weekdagen:

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
            trips: 2 # overschrijft trips voor deze begintijden
```

`start_times` groepeert vertrektijden die dezelfde ritparameters delen. Ontbrekende velden in een groep worden overgenomen van het slot, daarna van de dienstsoort.

## Webinterface

Een kleine HTTP-server waarmee iedereen een xlsx kan uploaden en het resulterende ICS-bestand kan downloaden, zonder iets lokaal te installeren.

```bash
go run ./cmd/web            # start op http://localhost:8080
go run ./cmd/web -port 9000 # aangepaste poort
```

| Vlag      | Standaard | Omschrijving                                                    |
| --------- | --------- | --------------------------------------------------------------- |
| `-port`   | `8080`    | TCP-poort om op te luisteren                                    |
| `-config` | —         | Pad naar een config.yaml-override (gebruikt ingebouwd als leeg) |

Open `http://localhost:8080` in een browser, kies je `.xlsx`-bestand, klik op **Omzetten & downloaden** en het `.ics`-bestand wordt meteen opgeslagen. Alle verwerking gebeurt in het geheugen — er worden geen bestanden naar schijf geschreven.

De server is ontworpen om achter een reverse proxy (nginx, Caddy, …) te draaien.

## Programmabestanden

| Bestand              | Platform                    |
| -------------------- | --------------------------- |
| `make-ics-macos`     | macOS arm64 (Apple Silicon) |
| `make-ics.exe`       | Windows amd64               |
| `make-ics-web-macos` | macOS arm64 webserver       |
| `make-ics-web.exe`   | Windows amd64 webserver     |

## Bouwen vanuit broncode

Vereist Go 1.24+.

```bash
# CLI
go run ./cmd/make-ics report.xlsx          # snel uitvoeren
go build -o make-ics-macos ./cmd/make-ics  # lokaal bouwen

# Webserver
go run ./cmd/web                           # snel uitvoeren
go build -o make-ics-web-macos ./cmd/web   # lokaal bouwen
```

Kruiscompilatie:

```bash
# CLI
GOOS=darwin  GOARCH=arm64 go build -o make-ics-macos ./cmd/make-ics
GOOS=windows GOARCH=amd64 go build -o make-ics.exe   ./cmd/make-ics

# Webserver
GOOS=darwin  GOARCH=arm64 go build -o make-ics-web-macos ./cmd/web
GOOS=windows GOARCH=amd64 go build -o make-ics-web.exe   ./cmd/web
```

## Ontwikkeling

De repo bevat een [Nix flake](flake.nix) en [direnv](https://direnv.net/)-configuratie. Met beide geïnstalleerd: `cd` naar de map en voer eenmalig `direnv allow` uit — Go staat dan automatisch in je PATH.

```bash
go test ./...   # alle tests uitvoeren
go vet ./...    # statische analyse
```

Pre-commit hooks (go fmt, go build, go vet, go test, binaries bouwen) worden automatisch uitgevoerd bij `git commit` na installatie:

```bash
pre-commit install
```
