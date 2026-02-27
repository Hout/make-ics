"""
Tests for make_ics.py

Focus: first_shift_advance is applied only to the first occurrence of a shift
code on a given day; subsequent same-day events fall back to the default
advance_minutes.
"""

import re
from datetime import date
from unittest.mock import MagicMock

import pytest

from make_ics import (
    DEFAULT_ADVANCE_MINUTES,
    find_date_range,
    get_trips,
    iter_events,
)

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

SHIFT_TYPES = {
    "HRm_": {
        "summary": "Binnendieze HRM",
        "trips": 2,
        "date_ranges": [
            {
                "from": date(2026, 4, 1),
                "to": date(2026, 4, 30),
                "first_shift_advance": 45,
                "trip_overrides": [
                    {"start_time": "14:40", "trips": 3},
                ],
            },
        ],
    }
}


def make_ws(*rows: tuple) -> MagicMock:
    ws = MagicMock()
    ws.iter_rows.return_value = list(rows)
    return ws


def collect(ws, advance: int = DEFAULT_ADVANCE_MINUTES) -> list[tuple[str, object]]:
    return list(
        iter_events(ws, duration_hours=4, advance_minutes=advance, shift_types=SHIFT_TYPES)
    )


def advance_of(label: str) -> int:
    """Extract advance minutes from a label like '2026-04-03 10:00  ... (-45min)'."""
    m = re.search(r"\(-(\d+)min\)", label)
    assert m, f"No advance found in label: {label!r}"
    return int(m.group(1))


# ---------------------------------------------------------------------------
# find_date_range
# ---------------------------------------------------------------------------

RANGES = [
    {"from": date(2026, 4, 1), "to": date(2026, 4, 17), "first_shift_advance": 30},
    {"from": date(2026, 4, 18), "to": date(2026, 6, 26), "first_shift_advance": 45},
]


def test_find_date_range_matches_first_entry():
    assert find_date_range(RANGES, date(2026, 4, 10)) is RANGES[0]


def test_find_date_range_matches_second_entry():
    assert find_date_range(RANGES, date(2026, 5, 1)) is RANGES[1]


@pytest.mark.parametrize(
    "d, expected_idx",
    [
        (date(2026, 4, 1), 0),   # start boundary first range
        (date(2026, 4, 17), 0),  # end boundary first range
        (date(2026, 4, 18), 1),  # start boundary second range
        (date(2026, 6, 26), 1),  # end boundary second range
    ],
)
def test_find_date_range_boundaries(d, expected_idx):
    assert find_date_range(RANGES, d) is RANGES[expected_idx]


def test_find_date_range_no_match():
    assert find_date_range(RANGES, date(2026, 7, 1)) is None


def test_find_date_range_empty_list():
    assert find_date_range([], date(2026, 4, 1)) is None


# ---------------------------------------------------------------------------
# get_trips
# ---------------------------------------------------------------------------

SHIFT = {"trips": 2}
RANGE_WITH_OVERRIDE = {
    "from": date(2026, 4, 1),
    "to": date(2026, 4, 30),
    "first_shift_advance": 45,
    "trip_overrides": [{"start_time": "14:40", "trips": 3}],
}
RANGE_WITHOUT_OVERRIDE = {"from": date(2026, 4, 1), "to": date(2026, 4, 30), "first_shift_advance": 45}


def test_get_trips_returns_default_when_no_range():
    assert get_trips(SHIFT, None, 10, 0) == 2


def test_get_trips_returns_default_when_range_has_no_overrides():
    assert get_trips(SHIFT, RANGE_WITHOUT_OVERRIDE, 10, 0) == 2


def test_get_trips_override_matches_start_time():
    assert get_trips(SHIFT, RANGE_WITH_OVERRIDE, 14, 40) == 3


def test_get_trips_override_does_not_match_start_time():
    assert get_trips(SHIFT, RANGE_WITH_OVERRIDE, 10, 0) == 2


def test_get_trips_returns_none_when_no_trips_field():
    assert get_trips({}, None, 10, 0) is None


# ---------------------------------------------------------------------------
# iter_events — first_shift_advance behaviour
# ---------------------------------------------------------------------------


def test_first_shift_of_day_gets_range_advance():
    ws = make_ws(("03-apr-26", "HRm_", "10:00 uur"))
    labels = [label for label, _ in collect(ws)]
    assert len(labels) == 1
    assert advance_of(labels[0]) == 45


def test_second_shift_same_day_gets_default_advance():
    """Only the first HRm_ event per day uses first_shift_advance."""
    ws = make_ws(
        ("03-apr-26", "HRm_", "10:00 uur"),
        ("03-apr-26", "HRm_", "14:40 uur"),
    )
    labels = [label for label, _ in collect(ws, advance=30)]
    assert advance_of(labels[0]) == 45  # first shift → range
    assert advance_of(labels[1]) == 30  # second shift → CLI default


def test_first_shift_on_each_day_independently_gets_range_advance():
    """Each calendar day resets the first-shift tracker."""
    ws = make_ws(
        ("03-apr-26", "HRm_", "10:00 uur"),
        ("04-apr-26", "HRm_", "11:00 uur"),
    )
    labels = [label for label, _ in collect(ws)]
    assert advance_of(labels[0]) == 45
    assert advance_of(labels[1]) == 45


def test_date_outside_all_ranges_gets_default_advance():
    ws = make_ws(("03-jul-26", "HRm_", "10:00 uur"))
    labels = [label for label, _ in collect(ws, advance=30)]
    assert advance_of(labels[0]) == 30


def test_unknown_shift_code_gets_default_advance():
    ws = make_ws(("03-apr-26", "UNKNOWN", "10:00 uur"))
    labels = [label for label, _ in collect(ws, advance=30)]
    assert advance_of(labels[0]) == 30


def test_trip_override_applied_only_on_first_shift():
    """trip_overrides in range_entry are only available for the first shift."""
    ws = make_ws(
        ("03-apr-26", "HRm_", "14:40 uur"),  # first shift, override → 3 trips
        ("03-apr-26", "HRm_", "14:40 uur"),  # second shift, no range_entry → 2 trips
    )
    events = collect(ws)
    descriptions = [str(event.get("description")) for _, event in events]
    assert "Ritten: 3" in descriptions[0]
    assert "Ritten: 2" in descriptions[1]
