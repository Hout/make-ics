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
    get_shift_duration_minutes,
    get_trips,
    iter_events,
)

# ---------------------------------------------------------------------------
# Shared date range fixtures
# ---------------------------------------------------------------------------

RANGES = [
    {"from": date(2026, 4, 1), "to": date(2026, 4, 17), "first_shift_advance": 30},
    {"from": date(2026, 4, 18), "to": date(2026, 6, 26), "first_shift_advance": 45},
]
RANGES_WITH_SPECIFIC = [
    {
        "from": date(2026, 4, 1),
        "to": date(2026, 4, 30),
        "first_shift_advance": 30,
        "start_times": [
            {"times": ["14:40"], "trips": 3},
        ],
    },
]

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
                "start_times": [
                    {"times": ["14:40"], "trips": 3},
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
    return list(iter_events(ws, duration_hours=4, advance_minutes=advance, shift_types=SHIFT_TYPES))


def advance_of(label: str) -> int:
    """Extract advance minutes from a label like '... (-45min +195min)'."""
    m = re.search(r"\(-(\d+)min", label)
    assert m, f"No advance found in label: {label!r}"
    return int(m.group(1))


# ---------------------------------------------------------------------------
# find_date_range
# ---------------------------------------------------------------------------


def test_find_date_range_matches_first_entry():
    assert find_date_range(RANGES, date(2026, 4, 10)) is RANGES[0]


def test_find_date_range_matches_second_entry():
    assert find_date_range(RANGES, date(2026, 5, 1)) is RANGES[1]


@pytest.mark.parametrize(
    "d, expected_idx",
    [
        (date(2026, 4, 1), 0),  # start boundary first range
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


def test_find_date_range_returns_merged_when_start_time_matches():
    result = find_date_range(RANGES_WITH_SPECIFIC, date(2026, 4, 10), "14:40")
    assert result is not None
    assert result["first_shift_advance"] == 30  # from entry
    assert result["trips"] == 3  # from group
    assert "start_times" not in result  # stripped


def test_find_date_range_returns_merged_when_one_of_multiple_start_times_matches():
    ranges = [
        {
            "from": date(2026, 4, 1),
            "to": date(2026, 4, 30),
            "first_shift_advance": 30,
            "start_times": [
                {"times": ["10:00", "14:40"], "trips": 3},
            ],
        },
    ]
    result = find_date_range(ranges, date(2026, 4, 10), "14:40")
    assert result is not None
    assert result["trips"] == 3
    result2 = find_date_range(ranges, date(2026, 4, 10), "10:00")
    assert result2 is not None
    assert result2["trips"] == 3


def test_find_date_range_returns_general_when_start_time_no_match():
    result = find_date_range(RANGES_WITH_SPECIFIC, date(2026, 4, 10), "10:00")
    assert result is RANGES_WITH_SPECIFIC[0]  # entry returned directly, no group match


def test_find_date_range_returns_general_when_no_start_time_given():
    result = find_date_range(RANGES_WITH_SPECIFIC, date(2026, 4, 10))
    assert result is RANGES_WITH_SPECIFIC[0]


def test_find_date_range_specific_field_overrides_general():
    ranges = [
        {
            "from": date(2026, 4, 1),
            "to": date(2026, 4, 30),
            "trips": 2,
            "first_shift_advance": 30,
            "start_times": [
                {"times": ["09:00"], "trips": 5},
            ],
        },
    ]
    result = find_date_range(ranges, date(2026, 4, 10), "09:00")
    assert result is not None
    assert result["trips"] == 5  # specific overrides general


# ---------------------------------------------------------------------------
# get_trips
# ---------------------------------------------------------------------------

SHIFT = {"trips": 2}
RANGE_WITH_TRIPS = {"from": date(2026, 4, 1), "to": date(2026, 4, 30), "trips": 3}
RANGE_WITHOUT_TRIPS = {"from": date(2026, 4, 1), "to": date(2026, 4, 30), "first_shift_advance": 45}


def test_get_trips_returns_shift_default_when_no_range():
    assert get_trips(SHIFT, None) == 2


def test_get_trips_returns_shift_default_when_range_has_no_trips():
    assert get_trips(SHIFT, RANGE_WITHOUT_TRIPS) == 2


def test_get_trips_returns_range_trips_when_present():
    assert get_trips(SHIFT, RANGE_WITH_TRIPS) == 3


def test_get_trips_range_trips_overrides_shift_trips():
    shift_with_trips = {"trips": 2}
    range_with_different = {"trips": 5}
    assert get_trips(shift_with_trips, range_with_different) == 5


def test_get_trips_returns_none_when_no_trips_anywhere():
    assert get_trips({}, None) is None


# ---------------------------------------------------------------------------
# get_shift_duration_minutes
# ---------------------------------------------------------------------------


def test_duration_computes_single_trip():
    shift = {"trip_duration": 90, "break_duration": 15}
    assert get_shift_duration_minutes(shift, None, 1, 240) == 90


def test_duration_computes_multiple_trips():
    # 2 trips x 90min + 1 break x 15min = 195min
    shift = {"trip_duration": 90, "break_duration": 15}
    assert get_shift_duration_minutes(shift, None, 2, 240) == 195


def test_duration_no_break_duration_defaults_to_zero():
    # 3 trips x 60min + 2 breaks x 0min = 180min
    shift = {"trip_duration": 60}
    assert get_shift_duration_minutes(shift, None, 3, 240) == 180


def test_duration_range_entry_overrides_shift():
    shift = {"trip_duration": 90, "break_duration": 15, "trips": 2}
    range_entry = {"trip_duration": 60, "break_duration": 10}
    # 2 x 60 + 1 x 10 = 130
    assert get_shift_duration_minutes(shift, range_entry, 2, 240) == 130


def test_duration_falls_back_to_default_when_no_trip_duration():
    shift = {"trips": 2}
    assert get_shift_duration_minutes(shift, None, 2, 240) == 240


def test_duration_falls_back_to_default_when_trips_is_none():
    shift = {"trip_duration": 90}
    assert get_shift_duration_minutes(shift, None, None, 240) == 240


def test_duration_falls_back_to_default_when_trips_is_zero():
    shift = {"trip_duration": 90}
    assert get_shift_duration_minutes(shift, None, 0, 240) == 240


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


def test_date_outside_all_ranges_uses_shift_level_first_shift_advance():
    """Shift-level first_shift_advance is used when no date range matches."""
    shift_types = {
        "HRm_": {
            "first_shift_advance": 25,
            "date_ranges": [
                {"from": date(2026, 4, 1), "to": date(2026, 4, 30), "first_shift_advance": 45},
            ],
        }
    }
    ws = make_ws(("03-jul-26", "HRm_", "10:00 uur"))
    labels = [label for label, _ in iter_events(ws, advance_minutes=30, shift_types=shift_types)]
    assert advance_of(labels[0]) == 25


def test_range_first_shift_advance_overrides_shift_level():
    """Date range first_shift_advance wins over shift-level when both are set."""
    shift_types = {
        "HRm_": {
            "first_shift_advance": 25,
            "date_ranges": [
                {"from": date(2026, 4, 1), "to": date(2026, 4, 30), "first_shift_advance": 45},
            ],
        }
    }
    ws = make_ws(("03-apr-26", "HRm_", "10:00 uur"))
    labels = [label for label, _ in iter_events(ws, advance_minutes=30, shift_types=shift_types)]
    assert advance_of(labels[0]) == 45


def test_shift_level_first_shift_advance_not_applied_to_second_shift():
    """Shift-level first_shift_advance must not affect subsequent same-day shifts."""
    shift_types = {
        "HRm_": {
            "first_shift_advance": 25,
            "date_ranges": [],
        }
    }
    ws = make_ws(
        ("03-apr-26", "HRm_", "10:00 uur"),
        ("03-apr-26", "HRm_", "14:00 uur"),
    )
    labels = [label for label, _ in iter_events(ws, advance_minutes=30, shift_types=shift_types)]
    assert advance_of(labels[0]) == 25  # first shift → shift level
    assert advance_of(labels[1]) == 30  # second shift → CLI default


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
