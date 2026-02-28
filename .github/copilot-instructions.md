# Copilot Instructions

## Project Overview
CLI tool (`make_ics.py`) that converts a Dutch xlsx schedule (`report.xlsx`) to an ICS calendar file, driven by `config.yaml`.

## Python Standards

- **Version**: Python 3.12 (type hints accordingly — use `X | Y` unions, `list[X]`, `dict[K, V]`, etc.; no `Optional`, no `Union`)
- **Line length**: 100 characters
- **Formatter**: ruff (`ruff format`)
- **Linter**: ruff with rule sets E, W, F, I, UP, B, SIM, RUF — always lint-clean; prefer auto-fixable patterns
- **Type checker**: ty (`ty check`) — all code must pass without errors

## Code Style

- Use type annotations on all function signatures and key variables
- Prefer `pathlib.Path` over `os.path`
- Use f-strings; avoid `%` formatting and `.format()`
- Avoid bare `except`; catch specific exceptions
- Keep functions small and single-purpose
- Use `dict.get()` with defaults instead of `key in dict` guards where natural
- Prefer early returns over deeply nested `if` blocks
- Constants in `UPPER_SNAKE_CASE` at module level

## Project Conventions

- Config is loaded from `config.yaml` via PyYAML; structure is `shift_type.<code>` with optional `date_ranges` list
- `find_date_range(date_ranges, appt_date, start_time)` returns a merged dict (range entry + matching `start_times` group); callers should treat the result as the authoritative source for all per-shift overrides
- Duration formula: `trips × trip_duration + max(0, trips − 1) × break_duration`
- `iter_events` pre-collects all rows before yielding, to support first/last-shift detection per `(code, date)` pair
- Event label format: `"YYYY-MM-DD HH:MM  <summary>  (−Xmin +Ymin)"`

## Testing

- Framework: pytest
- Test file: `test_make_ics.py` (same directory)
- Tests must pass before committing (`git commit` triggers the pre-commit hook)
- Cover edge cases: missing config keys, date range boundaries, first/last shift logic
- Use `pytest.mark.parametrize` for data-driven cases
- Do not use mocks unless I/O cannot be avoided; prefer pure function testing

## Dependencies

- `openpyxl` — read xlsx
- `icalendar` — build ICS
- `dateparser` — parse Dutch date strings (e.g. `03-apr-26`)
- `pyyaml` — load config
- Dev: `pytest`, `ruff`, `ty`

## Shell / Terminal

- **Never use heredocs** (`<<EOF ... EOF`) in terminal commands — they corrupt the shell session
- **Never inline multi-line Python or shell via quoted strings** (`-c "..."`) for anything beyond a trivial one-liner; write the code to a `.py` file instead and execute that
- Write temporary scripts to `.scratch/` (a hidden, gitignored folder in the workspace root), then run them with the venv Python and delete the file immediately after use

## Pre-commit Hook

`.git/hooks/pre-commit` runs automatically:
1. `ruff format` (auto-format)
2. `ruff check --fix` (lint + auto-fix, re-stages changed files)
3. `ty check` (type check — blocks commit on errors)
