"""
Reads an xlsx schedule file and generates a single ICS calendar file
containing all appointments.
"""

import argparse
import re
import uuid
from collections.abc import Iterator
from datetime import UTC, date, datetime, timedelta
from pathlib import Path

import dateparser
import openpyxl
import yaml
from icalendar import Calendar, Event

# --- Configuration ---
DEFAULT_DURATION_HOURS = 4
DEFAULT_ADVANCE_MINUTES = 30
TIMEZONE = datetime.now().astimezone().tzinfo
TRANSLATIONS_FILE = Path(__file__).parent / "config.yaml"


def load_config() -> dict:
    """Load config.yaml, returning an empty dict if the file is missing."""
    if not TRANSLATIONS_FILE.exists():
        return {}
    with TRANSLATIONS_FILE.open(encoding="utf-8") as f:
        return yaml.safe_load(f) or {}


def find_date_range(
    date_ranges: list[dict], appt_date: date, start_time: str | None = None
) -> dict | None:
    """Return a merged date range entry for appt_date.

    Finds the first date range entry whose from/to span covers appt_date.
    If that entry has a start_times list of groups and start_time matches a
    group's times list, the group's fields are merged on top of the entry
    (group takes precedence). start_times and times are excluded from the result.
    """
    for entry in date_ranges:
        if not (entry["from"] <= appt_date <= entry["to"]):
            continue
        if start_time:
            for group in entry.get("start_times") or []:
                times = [str(t).strip() for t in (group.get("times") or [])]
                if start_time in times:
                    base = {k: v for k, v in entry.items() if k != "start_times"}
                    overrides = {k: v for k, v in group.items() if k != "times"}
                    return {**base, **overrides}
        return entry
    return None


def get_trips(shift: dict, range_entry: dict | None) -> int | None:
    """Return trip count from the resolved range entry, falling back to shift default."""
    default = (range_entry or {}).get("trips") if range_entry else None
    if default is None:
        default = shift.get("trips")
    return int(default) if default is not None else None


def get_shift_duration_minutes(
    shift: dict,
    range_entry: dict | None,
    trips: int | None,
    default_minutes: int,
) -> int:
    """Return shift duration in minutes.

    Computes trips * trip_duration + (trips - 1) * break_duration when both
    trips and trip_duration are available. Falls back to default_minutes otherwise.
    Lookup order: range_entry fields override shift-level fields.
    """
    if not trips:
        return default_minutes
    merged = {**shift, **(range_entry or {})}
    trip_duration = merged.get("trip_duration")
    if trip_duration is None:
        return default_minutes
    break_duration = int(merged.get("break_duration", 0))
    return int(trips) * int(trip_duration) + max(0, trips - 1) * break_duration


def get_last_shift_remains(shift: dict, range_entry: dict | None) -> int:
    """Return the extra minutes appended to the last shift of the day, or 0."""
    merged = {**shift, **(range_entry or {})}
    return int(merged.get("last_shift_remains", 0))


def get_duration_rationale(
    shift: dict,
    range_entry: dict | None,
    trips: int | None,
    default_minutes: int,
    last_shift_remains: int = 0,
) -> str:
    """Return a human-readable string explaining how the duration was calculated."""
    if trips:
        merged = {**shift, **(range_entry or {})}
        trip_duration = merged.get("trip_duration")
        if trip_duration is not None:
            trip_duration = int(trip_duration)
            break_duration = int(merged.get("break_duration", 0))
            n_breaks = max(0, trips - 1)
            parts = f"{trips}x{trip_duration}"
            if n_breaks:
                parts += f"+{n_breaks}x{break_duration}"
            base = trips * trip_duration + n_breaks * break_duration
            rationale = f"{parts}={base}min"
            if last_shift_remains:
                rationale += f"+{last_shift_remains}min"
            return rationale
    return f"{default_minutes}min (default)"


def parse_dutch_date(date_str: str) -> date:
    """Parse a Dutch date string like '03-apr-26' using dateparser."""
    parsed = dateparser.parse(date_str.strip(), languages=["nl"])
    if parsed is None:
        raise ValueError(f"Could not parse date: {date_str!r}")
    return parsed.date()


def parse_time(time_str: str) -> tuple[int, int]:
    """Parse a time string like '14:40 uur' and return (hour, minute)."""
    time_str = time_str.strip()
    match = re.match(r"(\d{1,2}):(\d{2})", time_str)
    if not match:
        raise ValueError(f"Unexpected time format: {time_str!r}")
    return int(match.group(1)), int(match.group(2))


def is_data_row(row: tuple) -> bool:
    """Return True if the row looks like an appointment row (has date and time)."""
    date_val, _, time_val = row
    if not date_val or not time_val:
        return False
    date_str = str(date_val).strip()
    # Must match dd-mon-yy pattern
    return bool(re.match(r"\d{2}-[a-zA-Z]{3}-\d{2}", date_str))


def make_calendar(name: str) -> Calendar:
    """Return a new empty Calendar."""
    cal = Calendar()
    cal.add("prodid", f"-//make-ics//{name}//NL")
    cal.add("version", "2.0")
    cal.add("calscale", "GREGORIAN")
    cal.add("method", "PUBLISH")
    return cal


def iter_events(
    ws,
    duration_hours: float = DEFAULT_DURATION_HOURS,
    advance_minutes: int = DEFAULT_ADVANCE_MINUTES,
    shift_types: dict[str, dict] | None = None,
) -> Iterator[tuple[str, Event]]:
    """Yield (label, Event) for each appointment row in the worksheet."""
    # Pre-collect all parseable rows so we can identify first/last per (code, date).
    parsed_rows: list[tuple[str, date, int, int]] = []
    for row in ws.iter_rows(values_only=True):
        if not is_data_row(row):
            continue
        date_str, dienst_str, time_str = row
        code = str(dienst_str).strip() if dienst_str else "Afspraak"
        try:
            appt_date = parse_dutch_date(str(date_str))
            hour, minute = parse_time(str(time_str))
        except ValueError as exc:
            print(f"  [SKIP] Could not parse row {row}: {exc}")
            continue
        parsed_rows.append((code, appt_date, hour, minute))

    # Build maps: first and last row index per (code, date).
    first_idx: dict[tuple[str, date], int] = {}
    last_idx: dict[tuple[str, date], int] = {}
    for idx, (code, appt_date, _, _) in enumerate(parsed_rows):
        key = (code, appt_date)
        if key not in first_idx:
            first_idx[key] = idx
        last_idx[key] = idx

    for idx, (code, appt_date, hour, minute) in enumerate(parsed_rows):
        key = (code, appt_date)
        is_first = idx == first_idx[key]
        is_last = idx == last_idx[key]

        tr = (shift_types or {}).get(code, {})
        summary = tr.get("summary", code)
        tr_description = tr.get("description")

        start_time = f"{hour:02d}:{minute:02d}"
        raw_ranges = tr.get("date_ranges")
        date_ranges: list[dict] = raw_ranges if isinstance(raw_ranges, list) else []
        range_entry = find_date_range(date_ranges, appt_date, start_time)

        if is_first and range_entry and "first_shift_advance" in range_entry:
            advance = int(range_entry["first_shift_advance"])
        elif is_first and "first_shift_advance" in tr:
            advance = int(tr["first_shift_advance"])
        else:
            advance = advance_minutes

        trips = get_trips(tr, range_entry)
        duration_minutes = get_shift_duration_minutes(
            tr, range_entry, trips, int(duration_hours * 60)
        )
        remains = get_last_shift_remains(tr, range_entry) if is_last else 0
        rationale = get_duration_rationale(
            tr, range_entry, trips, int(duration_hours * 60), remains
        )
        duration_minutes += remains

        description = f"Start {hour:02d}:{minute:02d}"
        if trips is not None:
            description += f"  |  Ritten: {trips}"
        if tr_description:
            description += f"\n{tr_description}"

        dt_appt = datetime(
            appt_date.year, appt_date.month, appt_date.day, hour, minute, tzinfo=TIMEZONE
        )
        dt_start = dt_appt - timedelta(minutes=advance)
        dt_end = dt_appt + timedelta(minutes=duration_minutes)

        event = Event()
        event.add("summary", summary)
        event.add("description", description)
        event.add("dtstart", dt_start)
        event.add("dtend", dt_end)
        event.add("dtstamp", datetime.now(tz=UTC))
        event["uid"] = str(uuid.uuid4())

        label = (
            f"{appt_date} {hour:02d}:{minute:02d}  {summary}"
            f"  (-{advance}min +{duration_minutes}min: {rationale})"
        )
        yield label, event


def main():
    parser = argparse.ArgumentParser(
        description="Convert an xlsx schedule to a single ICS calendar file."
    )
    parser.add_argument(
        "input",
        nargs="?",
        default="report.xlsx",
        help="Path to the input xlsx file (default: report.xlsx)",
    )
    parser.add_argument(
        "-d",
        "--duration",
        type=float,
        default=DEFAULT_DURATION_HOURS,
        metavar="HOURS",
        help=f"Duration of each appointment in hours (default: {DEFAULT_DURATION_HOURS})",
    )
    parser.add_argument(
        "-a",
        "--advance",
        type=int,
        default=DEFAULT_ADVANCE_MINUTES,
        metavar="MINUTES",
        help=(
            f"Start event N minutes before the appointment time"
            f" (default: {DEFAULT_ADVANCE_MINUTES})"
        ),
    )
    args = parser.parse_args()

    input_path = Path(args.input)
    if not input_path.exists():
        raise FileNotFoundError(f"Input file not found: {input_path.resolve()}")

    print(f"Reading {input_path} …")
    wb = openpyxl.load_workbook(input_path)
    ws = wb.active
    print(f"Sheet: {ws.title}\n")

    ics_path = input_path.with_suffix(".ics")
    config = load_config()
    shift_types = config.get("shift_type") or {}
    cal = make_calendar(input_path.stem)
    count = 0
    for label, event in iter_events(
        ws,
        duration_hours=args.duration,
        advance_minutes=args.advance,
        shift_types=shift_types,
    ):
        cal.add_component(event)
        count += 1
        print(f"  + {label}")

    ics_path.write_bytes(cal.to_ical())
    print(f"\nTotal events written: {count}")
    print(f"Written to {ics_path.resolve()}")


if __name__ == "__main__":
    main()
