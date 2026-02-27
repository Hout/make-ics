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


def find_date_range(date_ranges: list[dict], appt_date: date) -> dict | None:
    """Return the first matching date range entry for appt_date, or None."""
    for entry in date_ranges:
        if entry["from"] <= appt_date <= entry["to"]:
            return entry
    return None


def get_trips(shift: dict, range_entry: dict | None, hour: int, minute: int) -> int | None:
    """Return trip count for a start time, checking range trip_overrides first."""
    start = f"{hour:02d}:{minute:02d}"
    for override in (range_entry or {}).get("trip_overrides") or []:
        if str(override.get("start_time", "")).strip() == start:
            return int(override["trips"])
    default = shift.get("trips")
    return int(default) if default is not None else None


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
    first_shift_seen: dict[tuple[str, date], bool] = {}
    for row in ws.iter_rows(values_only=True):
        if not is_data_row(row):
            continue

        date_str, dienst_str, time_str = row
        code = str(dienst_str).strip() if dienst_str else "Afspraak"
        tr = (shift_types or {}).get(code, {})
        summary = tr.get("summary", code)
        tr_description = tr.get("description")

        try:
            appt_date = parse_dutch_date(str(date_str))
            hour, minute = parse_time(str(time_str))
        except ValueError as exc:
            print(f"  [SKIP] Could not parse row {row}: {exc}")
            continue

        is_first = not first_shift_seen.get((code, appt_date), False)
        first_shift_seen[(code, appt_date)] = True
        raw_ranges = tr.get("date_ranges")
        date_ranges: list[dict] = raw_ranges if isinstance(raw_ranges, list) else []
        range_entry = find_date_range(date_ranges, appt_date) if is_first else None
        advance = int(range_entry["first_shift_advance"]) if range_entry else advance_minutes

        trips = get_trips(tr, range_entry, hour, minute)
        description = f"Start {hour:02d}:{minute:02d}"
        if trips is not None:
            description += f"  |  Ritten: {trips}"
        if tr_description:
            description += f"\n{tr_description}"

        dt_appt = datetime(
            appt_date.year, appt_date.month, appt_date.day, hour, minute, tzinfo=TIMEZONE
        )
        dt_start = dt_appt - timedelta(minutes=advance)
        dt_end = dt_appt + timedelta(hours=duration_hours)

        event = Event()
        event.add("summary", summary)
        event.add("description", description)
        event.add("dtstart", dt_start)
        event.add("dtend", dt_end)
        event.add("dtstamp", datetime.now(tz=UTC))
        event["uid"] = str(uuid.uuid4())

        label = f"{appt_date} {hour:02d}:{minute:02d}  {summary}  (-{advance}min)"
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
