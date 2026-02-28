# Copilot Instructions

## Project Overview
CLI tool (`make_ics.py`) that converts a Dutch xlsx schedule (`report.xlsx`) to an ICS calendar file, driven by `config.yaml`.

## Python Standards

- **Version**: Python 3.12 (type hints accordingly — use `X | Y` unions, `list[X]`, `dict[K, V]`, etc.; no `Optional`, no `Union`)
- **Line length**: 100 characters
- **Formatter**: ruff (`ruff format`)
- **Linter**: ruff with rule sets E, W, F, I, UP, B, C, SIM, RUF — always lint-clean; prefer auto-fixable patterns; cyclomatic complexity must stay below 10 (C901)
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
- Event label format: `"YYYY-MM-DD HH:MM  <summary>  (−Xmin +Ymin: AxB+CxD=Ymin)"`

## Testing

- **New functionality must be developed test-first (TDD)**: write the failing test(s) before implementing the feature, then make them pass with the minimal correct implementation
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
- Dev: `pytest`, `ruff`, `ty`, `pre-commit`

## Shell / Terminal

- **Never use heredocs** (`<<EOF ... EOF`) in terminal commands — they corrupt the shell session
- **Never inline multi-line Python or shell via quoted strings** (`-c "..."`) for anything beyond a trivial one-liner; write the code to a `.py` file instead and execute that
- **Write temporary files to `.scratch/`** (a hidden, gitignored folder in the workspace root), then run them with the venv Python and delete the file immediately after use. Do NOT use /tmp/ or similar system temp directories.

## Pre-commit Hook

Managed by the [pre-commit](https://pre-commit.com) package. Config lives in `.pre-commit-config.yaml`; hooks are installed into `.git/hooks/pre-commit` via `pre-commit install`.

Hooks (all `local`, pointing to `.venv/bin/`):
1. `ruff format` (auto-format)
2. `ruff check --fix` (lint + auto-fix — if files are modified, stage them and commit again)
3. `ty check` (type check — blocks commit on errors)

To reinstall after cloning or recreating the venv: `pre-commit install`

## Completing a Feature

After every new or changed feature, run these steps in order before considering the work done:

1. **Format** — `ruff format make_ics.py test_make_ics.py`
2. **Lint** — `ruff check make_ics.py test_make_ics.py` (must be clean)
3. **Type-check** — `ty check make_ics.py` (must be clean)
4. **Test** — `.venv/bin/pytest test_make_ics.py -q` (all must pass)
5. **Smoke-test** — `.venv/bin/python make_ics.py report.xlsx` and verify output
6. **Commit** — `git add -A && git commit -m "<concise description>"` (pre-commit hooks re-run steps 1–3 automatically; fix any remaining issues and commit again)
