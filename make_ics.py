"""
Reads an xlsx schedule file and generates an ICS calendar file for all
appointment rows, packaged in a zip file with the same name as the input.
"""

import argparse
import re
import uuid
import zipfile
from datetime import date, datetime, timedelta, timezone
from pathlib import Path

import dateparser
import openpyxl
from icalendar import Calendar, Event

# --- Configuration ---
DEFAULT_DURATION_HOURS = 4
TIMEZONE = datetime.now().astimezone().tzinfo


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


def build_calendar(ws, name: str = "calendar", duration_hours: float = DEFAULT_DURATION_HOURS) -> Calendar:
    cal = Calendar()
    cal.add("prodid", f"-//make-ics//{name}//NL")
    cal.add("version", "2.0")
    cal.add("calscale", "GREGORIAN")
    cal.add("method", "PUBLISH")

    count = 0
    for row in ws.iter_rows(values_only=True):
        if not is_data_row(row):
            continue

        date_str, dienst_str, time_str = row
        dienst = str(dienst_str).strip() if dienst_str else "Afspraak"

        try:
            appt_date = parse_dutch_date(str(date_str))
            hour, minute = parse_time(str(time_str))
        except ValueError as exc:
            print(f"  [SKIP] Could not parse row {row}: {exc}")
            continue

        dt_start = datetime(appt_date.year, appt_date.month, appt_date.day,
                            hour, minute, tzinfo=TIMEZONE)
        dt_end = dt_start + timedelta(hours=duration_hours)

        event = Event()
        event.add("summary", dienst)
        event.add("dtstart", dt_start)
        event.add("dtend", dt_end)
        event.add("dtstamp", datetime.now(tz=timezone.utc))
        event["uid"] = str(uuid.uuid4())

        cal.add_component(event)
        count += 1
        print(f"  + {appt_date} {hour:02d}:{minute:02d}  →  {dienst}")

    print(f"\nTotal events added: {count}")
    return cal


def main():
    parser = argparse.ArgumentParser(
        description="Convert an xlsx schedule to an ICS file inside a zip archive."
    )
    parser.add_argument(
        "input",
        nargs="?",
        default="report.xlsx",
        help="Path to the input xlsx file (default: report.xlsx)",
    )
    parser.add_argument(
        "-d", "--duration",
        type=float,
        default=DEFAULT_DURATION_HOURS,
        metavar="HOURS",
        help=f"Duration of each appointment in hours (default: {DEFAULT_DURATION_HOURS})",
    )
    args = parser.parse_args()

    input_path = Path(args.input)
    if not input_path.exists():
        raise FileNotFoundError(f"Input file not found: {input_path.resolve()}")

    print(f"Reading {input_path} …")
    wb = openpyxl.load_workbook(input_path)
    ws = wb.active
    print(f"Sheet: {ws.title}\n")

    cal = build_calendar(ws, name=input_path.stem, duration_hours=args.duration)

    stem = input_path.stem
    ics_name = stem + ".ics"
    zip_path = input_path.with_suffix(".zip")

    ics_bytes = cal.to_ical()
    with zipfile.ZipFile(zip_path, "w", compression=zipfile.ZIP_DEFLATED) as zf:
        zf.writestr(ics_name, ics_bytes)

    print(f"\nWritten {ics_name} into {zip_path.resolve()}")


if __name__ == "__main__":
    main()
