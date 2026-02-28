"""
Tests for make_ics.py

Focus: first_shift_advance is applied only to the first occurrence of a shift
code on a given day; subsequent same-day events fall back to the default
advance_minutes.
"""

import re
from datetime import date
from pathlib import Path
from unittest.mock import MagicMock

import pytest
from icalendar import Event

from make_ics import (
    DEFAULT_ADVANCE_MINUTES,
    find_date_range,
    get_duration_rationale,
    get_last_shift_remains,
    get_shift_duration_minutes,
    get_trips,
    is_data_row,
    iter_events,
    load_config,
    main,
    make_calendar,
    parse_dutch_date,
    parse_time,
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


def collect(ws, advance: int = DEFAULT_ADVANCE_MINUTES) -> list[tuple[str, Event]]:
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
# get_last_shift_remains
# ---------------------------------------------------------------------------


def test_last_shift_remains_returns_zero_when_not_set():
    assert get_last_shift_remains({}, None) == 0


def test_last_shift_remains_returns_shift_level_value():
    assert get_last_shift_remains({"last_shift_remains": 30}, None) == 30


def test_last_shift_remains_range_entry_overrides_shift():
    shift = {"last_shift_remains": 30}
    range_entry = {"last_shift_remains": 15}
    assert get_last_shift_remains(shift, range_entry) == 15


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


def test_trip_override_applied_to_all_shifts_with_matching_start_time():
    """range_entry is resolved for every shift, so all shifts at the same time get the override."""
    ws = make_ws(
        ("03-apr-26", "HRm_", "14:40 uur"),
        ("03-apr-26", "HRm_", "14:40 uur"),
    )
    events = collect(ws)
    descriptions = [str(event.get("description")) for _, event in events]
    assert "Ritten: 3" in descriptions[0]
    assert "Ritten: 3" in descriptions[1]


# ---------------------------------------------------------------------------
# iter_events — last_shift_remains behaviour
# ---------------------------------------------------------------------------


def test_last_shift_of_day_gets_remains_added_to_duration():
    shift_types = {
        "HRm_": {
            "trip_duration": 50,
            "break_duration": 0,
            "trips": 1,
            "last_shift_remains": 30,
        }
    }
    ws = make_ws(("03-apr-26", "HRm_", "10:00 uur"))
    events = list(iter_events(ws, advance_minutes=0, shift_types=shift_types))
    _, event = events[0]
    dtstart = event.get("dtstart").dt
    dtend = event.get("dtend").dt
    assert (dtend - dtstart).seconds // 60 == 80  # 50 trip + 30 remains


def test_only_last_shift_gets_remains():
    shift_types = {
        "HRm_": {
            "trip_duration": 50,
            "break_duration": 0,
            "trips": 1,
            "last_shift_remains": 30,
        }
    }
    ws = make_ws(
        ("03-apr-26", "HRm_", "10:00 uur"),
        ("03-apr-26", "HRm_", "14:00 uur"),
    )
    events = list(iter_events(ws, advance_minutes=0, shift_types=shift_types))
    dur_first = (events[0][1].get("dtend").dt - events[0][1].get("dtstart").dt).seconds // 60
    dur_last = (events[1][1].get("dtend").dt - events[1][1].get("dtstart").dt).seconds // 60
    assert dur_first == 50  # no remains
    assert dur_last == 80  # 50 + 30


def test_each_day_has_its_own_last_shift():
    shift_types = {
        "HRm_": {
            "trip_duration": 50,
            "break_duration": 0,
            "trips": 1,
            "last_shift_remains": 30,
        }
    }
    ws = make_ws(
        ("03-apr-26", "HRm_", "10:00 uur"),
        ("04-apr-26", "HRm_", "10:00 uur"),
    )
    events = list(iter_events(ws, advance_minutes=0, shift_types=shift_types))
    for _, event in events:
        dur = (event.get("dtend").dt - event.get("dtstart").dt).seconds // 60
        assert dur == 80  # both are last on their own day


def test_last_shift_remains_zero_when_not_configured():
    shift_types = {"HRm_": {"trip_duration": 50, "break_duration": 0, "trips": 1}}
    ws = make_ws(("03-apr-26", "HRm_", "10:00 uur"))
    events = list(iter_events(ws, advance_minutes=0, shift_types=shift_types))
    dur = (events[0][1].get("dtend").dt - events[0][1].get("dtstart").dt).seconds // 60
    assert dur == 50


# ---------------------------------------------------------------------------
# load_config
# ---------------------------------------------------------------------------


def test_load_config_returns_empty_dict_when_file_missing(monkeypatch, tmp_path):
    import make_ics

    monkeypatch.setattr(make_ics, "TRANSLATIONS_FILE", tmp_path / "nonexistent.yaml")
    assert load_config() == {}


def test_load_config_returns_empty_dict_when_file_is_empty(monkeypatch, tmp_path):
    import make_ics

    empty = tmp_path / "config.yaml"
    empty.write_text("")
    monkeypatch.setattr(make_ics, "TRANSLATIONS_FILE", empty)
    assert load_config() == {}


# ---------------------------------------------------------------------------
# get_duration_rationale
# ---------------------------------------------------------------------------


def test_duration_rationale_default_when_no_trips():
    assert get_duration_rationale({}, None, None, 240) == "240min (default)"


def test_duration_rationale_default_when_trips_but_no_trip_duration():
    assert get_duration_rationale({"trips": 2}, None, 2, 240) == "240min (default)"


def test_duration_rationale_single_trip_no_breaks():
    shift = {"trip_duration": 50, "break_duration": 15}
    assert get_duration_rationale(shift, None, 1, 240) == "1x50=50min"


def test_duration_rationale_multiple_trips_with_breaks():
    shift = {"trip_duration": 50, "break_duration": 15}
    assert get_duration_rationale(shift, None, 2, 240) == "2x50+1x15=115min"


def test_duration_rationale_with_last_shift_remains():
    shift = {"trip_duration": 50, "break_duration": 15}
    assert get_duration_rationale(shift, None, 2, 240, 30) == "2x50+1x15=115min+30min"


def test_duration_rationale_no_break_duration_field():
    shift = {"trip_duration": 60}
    # break_duration defaults to 0, n_breaks=1 but 0-valued so branch not printed
    assert get_duration_rationale(shift, None, 2, 240) == "2x60+1x0=120min"


# ---------------------------------------------------------------------------
# parse_dutch_date
# ---------------------------------------------------------------------------


def test_parse_dutch_date_valid():
    assert parse_dutch_date("03-apr-26") == date(2026, 4, 3)


def test_parse_dutch_date_raises_on_unparseable():
    with pytest.raises(ValueError, match="Could not parse date"):
        parse_dutch_date("not-a-date")


# ---------------------------------------------------------------------------
# parse_time
# ---------------------------------------------------------------------------


def test_parse_time_valid():
    assert parse_time("14:40 uur") == (14, 40)


def test_parse_time_raises_on_invalid_format():
    with pytest.raises(ValueError, match="Unexpected time format"):
        parse_time("geen tijd")


# ---------------------------------------------------------------------------
# is_data_row
# ---------------------------------------------------------------------------


def test_is_data_row_valid():
    assert is_data_row(("03-apr-26", "HRm_", "14:40 uur")) is True


def test_is_data_row_false_when_no_date():
    assert is_data_row((None, "HRm_", "14:40 uur")) is False


def test_is_data_row_false_when_no_time():
    assert is_data_row(("03-apr-26", "HRm_", None)) is False


def test_is_data_row_false_when_date_pattern_mismatch():
    assert is_data_row(("Datum", "Dienst", "Tijd")) is False


# ---------------------------------------------------------------------------
# make_calendar
# ---------------------------------------------------------------------------


def test_make_calendar_returns_calendar_with_correct_version():
    cal = make_calendar("test")
    assert cal.get("version").to_ical() == b"2.0"


def test_make_calendar_includes_name_in_prodid():
    cal = make_calendar("myfile")
    assert b"myfile" in cal.get("prodid").to_ical()


# ---------------------------------------------------------------------------
# iter_events — additional branches
# ---------------------------------------------------------------------------


def test_iter_events_skips_non_data_rows():
    """Header rows (da pattern mismatch) must be silently skipped."""
    ws = make_ws(
        ("Datum", "Dienst", "Tijd"),  # header - not a data row
        ("03-apr-26", "HRm_", "14:40 uur"),
    )
    events = collect(ws)
    assert len(events) == 1


def test_iter_events_skips_row_with_unparseable_time(capsys):
    """A row that passes is_data_row but has bad time is skipped with a SKIP message."""
    ws = make_ws(("03-apr-26", "HRm_", "geen-tijd"))
    events = collect(ws)
    assert events == []
    captured = capsys.readouterr()
    assert "[SKIP]" in captured.out


def test_iter_events_uses_afspraak_when_code_is_none():
    """When dienst column is None the code falls back to 'Afspraak'."""
    ws = make_ws(("03-apr-26", None, "14:40 uur"))
    events = list(iter_events(ws, shift_types={}))
    assert len(events) == 1
    label, _ = events[0]
    assert "Afspraak" in label


def test_iter_events_appends_description_when_shift_has_description():
    """tr_description is appended to event description on a new line."""
    shift_types = {
        "HRm_": {
            "summary": "Test",
            "description": "Some route detail",
            "trips": 1,
            "trip_duration": 50,
        }
    }
    ws = make_ws(("03-apr-26", "HRm_", "14:40 uur"))
    events = list(iter_events(ws, shift_types=shift_types))
    _, event = events[0]
    assert "Some route detail" in str(event.get("description"))


# ---------------------------------------------------------------------------
# main()
# ---------------------------------------------------------------------------

REPORT = Path(__file__).parent / "report.xlsx"


def test_main_raises_on_missing_input_file(monkeypatch):
    monkeypatch.setattr("sys.argv", ["make_ics.py", "no_such_file.xlsx"])
    with pytest.raises(FileNotFoundError, match="not found"):
        main()


@pytest.mark.skipif(not REPORT.exists(), reason="report.xlsx not present")
def test_main_runs_successfully_on_real_report(monkeypatch, tmp_path):
    import shutil

    xlsx = tmp_path / "report.xlsx"
    shutil.copy(REPORT, xlsx)
    monkeypatch.setattr("sys.argv", ["make_ics.py", str(xlsx)])
    main()
    assert (tmp_path / "report.ics").exists()


@pytest.mark.skipif(not REPORT.exists(), reason="report.xlsx not present")
def test_main_accepts_custom_duration_and_advance(monkeypatch, tmp_path):
    import shutil

    xlsx = tmp_path / "report.xlsx"
    shutil.copy(REPORT, xlsx)
    monkeypatch.setattr("sys.argv", ["make_ics.py", str(xlsx), "-d", "2", "-a", "15"])
    main()
    assert (tmp_path / "report.ics").exists()
