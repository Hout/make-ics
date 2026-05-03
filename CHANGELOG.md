# Changelog

All notable changes to make-ics are documented here.
## Unreleased

### Bug Fixes

- DST-safe timezone, wide-row safety, get_trips cleanup, suppress zero-break rationale
- Fix shift list table
- Remove stale tripSeg comment in list-shifts/main.go
- **range**: Skip slots whose start_times don't contain the requested time

### Build

- Update Nix dev shell to Go 1.25 and add tooling

### Configuration

- Remove duplicate times entries
- No shifts on Mondays before 14:00

### Documentation

- Update README and Copilot instructions for new config schema

### Features

- Include duration rationale in printed event label
- Weekday filter for date ranges
- Add list-shifts command (Markdown weekly shift overview)
- Implement exceptions — remap dates to a different weekday schedule
- Replace date_ranges with seasons + schedules/slots model
- Include config.yaml line numbers in errors and warnings
- Add cmd/web HTTP server for xlsx→ICS conversion
- **config**: Update 2026 schedule — exceptions, seasons, trip durations

### Maintenance

- Add ruff & ty checking
- Remove redundant stem variable in main()
- Add YAML-based abbreviation translations
- Rename translations.yaml to config.yaml
- Rename 'translations' to 'trip_type' in config.yaml and make_ics.py
- Rename time_in_advance to advance_minutes in config.yaml and make_ics.py
- Rename trip_type to shift_type; add trips and trip_overrides
- Add tests for first_shift_advance and trip_overrides logic
- Rename start_time to start_times (list) in date_ranges entries
- Add trip_duration and break_duration for computed shift duration
- Add last_shift_remains: extra duration on last shift of day
- Add Copilot instructions
- Warn against heredoc and multi-line shell quoting
- Use scripts/ subfolder instead of /tmp for temp scripts
- Use .scratch/ for temp scripts, gitignore it, clean up after use
- Add pytest-cov to dev requirements
- Mandate TDD for new functionality
- Switch to pre-commit package
- Add pytest hook to pre-commit
- Always run pytest hook regardless of staged file types
- Add i18n (nl_NL/en_GB) and rich trip-schedule description
- Remove duration rationale from printed label
- Add Go implementation of make-ics
- Add Go hooks to pre-commit; scope all language checks to their own file types
- Add TZID to DTSTART/DTEND in ICS output
- Remove accidentally committed diff_ics.py debug script
- Add go-build-binaries pre-commit hook to rebuild cross-platform executables
- Add README
- Add Dutch README (README.nl.md) with language switcher
- Remove windows tag on exe
- Untrack test.md (covered by .gitignore)
- Remove dead code and fix import alias (best practices review)
- Remove archived Python implementation and pycache
- Remove tmp usage
- Rename first_shift_advance/time fields; remove first_shift_* from StartTimeGroup
- Add shift_preparation field; rename last_shift_remains to last_shift_aftercare
- Rename first_shift_advance_* -> first_shift_preparation_*; shift_preparation -> shift_preparation_duration; fallback first-shift to shift_preparation_duration
- Ignore krv_docs/, tighten VS Code settings, set Caddy domain

### Other

- Initial commit: xlsx-to-ICS converter

- Reads Dutch-language xlsx schedule (openpyxl + dateparser)
- Generates ICS events with local timezone (icalendar)
- Packages output into a zip named after the input file
- Configurable appointment duration via -d/--duration CLI flag
- Gitignore
- One ICS per appointment, named by date; add --advance flag

- Refactor build_calendar into iter_appointment_calendars generator
- Each appointment gets its own dated ICS file in the zip
- Add -a/--advance (default 30 min) to start events early
- Fix UP035: Iterator from collections.abc
- Write single ICS file directly instead of per-file zip
- Clean up stale zip_path variable and argparse description
- Prepend 'Start HH:MM' as first line of event description
- Support per-date-range time_in_advance in config.yaml

- config.yaml: restructure with 'translations' and 'date_ranges' sections
- load_config() replaces load_translations(), returns full config dict
- get_advance_minutes() picks the matching date range or falls back to CLI default
- iter_events computes advance per appointment; label shows actual advance used
- Move time_in_advance overrides under each translation as first_shift_advance

- config.yaml: first_shift_advance date ranges now live under each abbreviation
- iter_events tracks first occurrence of each (code, date) pair
- Only the first shift of that type on a day uses the first_shift_advance ranges
- Subsequent shifts on the same day always use the default advance
- Restructure config: date_ranges with first_shift_advance and trip_overrides per entry

- config.yaml: first_shift_advance list renamed to date_ranges; advance value
  is now a scalar field first_shift_advance per entry; trip_overrides also per entry
- make_ics.py: find_date_range looks up date_ranges; range_entry[first_shift_advance]
  used for advance; description/trips resolved after range_entry is known; remove
  duplicate get_trips call and fix broken dt_appt statement
- Flatten trip_overrides into date_ranges entries with start_time

- config.yaml: remove trip_overrides wrapper; add a sibling date_range entry
  with start_time field instead; fields on more-specific entries take precedence
- make_ics.py: find_date_range now accepts optional start_time; finds general
  (no start_time) and specific (matching start_time) entries and merges them
  with specific taking precedence; get_trips simplified to just read trips from
  range_entry (already merged), falling back to shift-level trips; iter_events
  passes start_time string to find_date_range
- test_make_ics.py: updated fixtures and tests to reflect new structure; added
  4 new find_date_range tests for merging and precedence; get_trips tests
  simplified (no hour/minute params); total 23 tests
- Allow first_shift_advance at shift level as middle fallback

- make_ics.py: advance resolution now checks range_entry > shift-level
  tr['first_shift_advance'] > CLI default (shift level only on first shift)
- test_make_ics.py: 3 new tests for shift-level first_shift_advance precedence
- Nest start_times groups inside date_range entry (DRY)

- config.yaml: start_times is now a list of groups nested inside the date
  range entry; each group has 'times' (list) + override fields
- make_ics.py: find_date_range finds the single matching entry and merges
  a matching start_times group on top; start_times/times keys excluded from result
- test_make_ics.py: fixtures and tests updated for nested structure
- Translate trip/trips as tocht/tochten in nl_NL
- Show last_shift_remains in description as '+ Xmin → HH:MM'
- Move tr_description to top of event description
- Rewrite description rationale as bullet list
- Replace bullet description with time-ordered programme
- Shorten preparation msgid; recompile .mo
- Use IANA timezone from config.yaml for DST-correct DTSTART/DTEND
- Move locale to config.yaml; remove -l/-a CLI args
- Improve typing for config/shift mappings and clean ruff/ty issues
- Apply Go review recommendations
- Archive Python implementation; Go is now the sole active codebase
- Apply remaining Go best practices: docs, any, errors, constants, helpers
- Format
- Fix BuildProgram i18n keys, separator, code trimming, and summary lookup

- Add formatTimeLine helper that uses the '{time} {text}' i18n key so
  every schedule line gets the '  -  ' separator from nl.json translation
- Fix Trip/Break message IDs: 'Trip' → 'Trip {n}', 'Break' → 'Break {n}'
- Fix aftercare key: 'aftercare' → 'aftercare → {time}'
- Add locales/en.json with English translations for testing
- Add TDD tests: TestBuildProgram_English and TestBuildProgram_Dutch using
  real i18n.Localizer; cases match exact events from example.ics
- Trim whitespace from xlsx shift code so 'HRm_ ' matches config key 'HRm_'
- Use shift.Summary from config for VEVENT SUMMARY instead of raw code
- Add pipeline tests: summary-from-config assertion and CodeTrimsWhitespace
- Extend TZID test to cover winter DST (UTC+1) in addition to summer (UTC+2)
- Format schedule_test.go struct literal alignment
- Auto-format Go files in pre-commit hook

- Add scripts/go-fmt-stage.sh: runs gofmt -w then re-stages only the
  files it changed, so formatted code is included in the same commit
  rather than requiring a follow-up commit
- Add go-fmt hook to .pre-commit-config.yaml, runs before build/vet/test
- Fix .gitignore: anchor /make-ics and /make-ics.exe to repo root so the
  cmd/make-ics/ source directory is not incorrectly ignored
- Embed locales and config.yaml into binary using go:embed

- Move locales/ into pkg/i18n/locales/ and embed via //go:embed locales
- Move config.yaml into cmd/make-ics/ and embed via //go:embed config.yaml
- NewLocalizer(locale string) — path argument removed, always uses embedded FS
- LoadConfigFromBytes and IsEmpty added to pkg/config
- Embedded config used as fallback when config.yaml not found on disk
- Update all test call sites to new NewLocalizer signature
- Simplify binaries: macOS arm64 + Windows only, renamed to make-ics-macos / make-ics.exe
- Fix README markdownlint warnings
- Clarify no configuration needed by default in both READMEs
- Note that built-in config covers HRM shifts only
- Revove trivial exe
- Andere routes
- Sweep-line produces disjunct period sections
- One table per shift type within each period
- Use full summary as shift section heading
- Merge split weekday ranges into one table per shift type
- Add --trips and --mermaid output modes
- Mermaid shows one bar per departure (full span)
- Remove --mermaid flag and related code
- Fix --trips seq numbers; departure starts always seq=1
- Collapse consecutive equal-times days into range labels (Tue–Sun)
- Use dd-Mmm-yy date format with compact same-month range notation
- Determine first shift from schedule times, not xlsx row position
- Caddy config

### Refactoring

- Add mccabe C901 complexity check; extract _collect_rows to reduce iter_events complexity
- Require explicit config file; remove embedded default

### Testing

- Increase coverage to 100%
- Regenerate test.md with updated list-shifts output


