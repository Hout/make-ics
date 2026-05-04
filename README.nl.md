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

### Velden per dienstsoort

| Veld               | Omschrijving                                              |
| ------------------ | --------------------------------------------------------- |
| `summary`          | VEVENT `SUMMARY` (calendertitel)                          |
| `description`      | Vaste tekst toegevoegd aan de beschrijving                |
| `season_schedules` | Seizoensgebonden roosterlijst (zie hieronder)             |

### Dienstvelden

Elke invoer in een `shifts`-map beschrijft het kalenderevenement voor één dienstnummer zoals dat in de xlsx verschijnt:

| Veld         | Omschrijving                                                                                          |
| ------------ | ----------------------------------------------------------------------------------------------------- |
| `trips`      | Lijst van vertrektijden (`HH:MM`) — één per rit                                                       |
| `trip_times` | Alternatief voor `trips`: lijst van `{start: "HH:MM", duration: <minuten>}`-structuren               |
| `arrive`     | Vaste kloktijd (HH:MM) waarop het evenement begint; standaard eerste vertrek − aankomsttijd           |
| `leave`      | Vaste kloktijd (HH:MM) waarop het evenement eindigt; verplicht bij één rit, of berekend uit de laatste rit bij meerdere ritten |

### Seizoenen en uitzonderingen

`seasons` zijn benoemde datumvensters waarnaar `schedules` verwijzen. Een seizoen kan meerdere `{from, to}`-bereiken bevatten (inclusief grenzen).

`exceptions` koppelen specifieke kalenderdatums aan een andere weekdag voor het zoeken naar het rooster — handig voor feestdagen die een weekendrooster volgen.

### Roosters, dagroosters en diensten

Elke invoer onder `season_schedules` geldt wanneer de dienstdatum binnen een van de bijbehorende `seasons` valt. Binnen een rooster beperken `day_schedules` het tot weekdagen:

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

`shifts` koppelt dienstnummers (zoals ze in de xlsx staan) aan evenementdefinities. `arrive` en `leave` bepalen de grenzen van het kalenderevenement. `leave` is verplicht bij enkelvoudige ritten; bij meerdere ritten kan het worden weggelaten en wordt het berekend uit het eindtijdstip van de laatste rit (vereist expliciete `trip_times` met een `duration`).

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
