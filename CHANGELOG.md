# Changelog

All notable changes to make-ics are documented here.
## Unreleased

### Bug Fixes

- DST-safe timezone, wide-row safety, get_trips cleanup, suppress zero-break rationale
- Improve typing for config/shift mappings and clean ruff/ty issues
- Remove stale tripSeg comment in list-shifts/main.go
- **range**: Skip slots whose start_times don't contain the requested time

### Build

- Update Nix dev shell to Go 1.25 and add tooling
- Add git-cliff changelog automation

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
- Ignore platform binaries; remove duplicate comment

### Refactoring

- Add mccabe C901 complexity check; extract _collect_rows to reduce iter_events complexity
- Require explicit config file; remove embedded default
- Remove shift_preparation_duration; rename last_shift_aftercare
- **pipeline**: Split IterEvents into three focused phases

### Testing

- Increase coverage to 100%
- Regenerate test.md with updated list-shifts output


