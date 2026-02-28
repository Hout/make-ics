#!/usr/bin/env python3
"""
Minimal .po -> go-i18n JSON converter.

Place this in `scripts/` and run it from the repo root:

  ./scripts/convert_po_to_goi18n.py locale/nl_NL/LC_MESSAGES/make_ics.po

It produces `locales/<lang>.json` where <lang> is the primary language tag
extracted from the path (e.g. nl_NL -> nl).

This script implements a lightweight .po parser sufficient for the simple
translation files used in this project (no fuzzy/obsolete handling).
"""

import json
import os
import re
import sys


def unquote_po_string(s: str) -> str:
    # PO strings are quoted, possibly broken into multiple "..." parts.
    # Input passed here will already be the interior text, so just strip quotes.
    return s


def convert_placeholders(s: str) -> str:
    # replace Python-style {name} with go-i18n template {{.name}}
    return re.sub(r"\{([^}]+)\}", r"{{.\1}}", s)


def _collect_continuation(lines: list[str], idx: int) -> tuple[str, int]:
    """Consume consecutive continuation-quoted lines; return joined text and new index."""
    text = ""
    while idx < len(lines) and lines[idx].strip().startswith('"'):
        segment = lines[idx].strip()
        text += segment[1:-1]
        idx += 1
    return text, idx


def _parse_line(line: str, lines: list[str], idx: int, cur: dict) -> int:
    """Process one non-blank, non-comment .po line; returns updated idx."""
    if line.startswith("msgid_plural"):
        m = re.match(r'msgid_plural\s+"(.*)"', line)
        if m:
            cur["msgid_plural"] = m.group(1)
        return idx
    if line.startswith("msgid"):
        m = re.match(r'msgid\s+"(.*)"', line)
        base = m.group(1) if m else ""
        cont, idx = _collect_continuation(lines, idx)
        cur["msgid"] = base + cont
        return idx
    if line.startswith("msgstr["):
        m = re.match(r'msgstr\[(\d+)\]\s+"(.*)"', line)
        if m:
            cont, idx = _collect_continuation(lines, idx)
            cur.setdefault("msgstr_plural", {})[int(m.group(1))] = m.group(2) + cont
        return idx
    if line.startswith("msgstr"):
        m = re.match(r'msgstr\s+"(.*)"', line)
        base = m.group(1) if m else ""
        cont, idx = _collect_continuation(lines, idx)
        cur["msgstr"] = base + cont
        return idx
    return idx


def parse_po(path: str) -> list[dict]:
    with open(path, encoding="utf-8") as f:
        lines = f.readlines()

    entries: list[dict] = []
    idx = 0
    cur: dict = {}
    while idx < len(lines):
        line = lines[idx].rstrip("\n")
        idx += 1
        if not line.strip():
            if cur:
                entries.append(cur)
                cur = {}
            continue
        if line.startswith("#"):
            continue
        idx = _parse_line(line, lines, idx, cur)
    if cur:
        entries.append(cur)
    return entries


def convert_po_to_goi18n(po_path, out_dir):
    entries = parse_po(po_path)
    # derive lang code
    parts = po_path.split(os.sep)
    # expect locale/<lang>/_... find the folder after 'locale'
    lang = None
    if "locale" in parts:
        i = parts.index("locale")
        if i + 1 < len(parts):
            lang_full = parts[i + 1]
            lang = lang_full.split("_")[0]
    if lang is None:
        lang = "en"

    out = []
    for e in entries:
        if "msgid" not in e:
            continue
        msgid = e.get("msgid", "")
        # skip header
        if msgid == "":
            continue
        if "msgstr_plural" in e:
            one = e["msgstr_plural"].get(0, "")
            other = e["msgstr_plural"].get(1, "")
            one = convert_placeholders(one)
            other = convert_placeholders(other)
            out.append(
                {
                    "id": msgid,
                    "translation": {
                        "one": one,
                        "other": other,
                    },
                }
            )
        else:
            tr = e.get("msgstr", "")
            tr = convert_placeholders(tr)
            out.append({"id": msgid, "translation": tr})

    os.makedirs(out_dir, exist_ok=True)
    out_path = os.path.join(out_dir, f"{lang}.json")
    with open(out_path, "w", encoding="utf-8") as f:
        json.dump(out, f, ensure_ascii=False, indent=2)
    print(f"Wrote {out_path}")


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: convert_po_to_goi18n.py path/to/file.po [outdir=locales]")
        sys.exit(2)
    po = sys.argv[1]
    out = sys.argv[2] if len(sys.argv) > 2 else "locales"
    convert_po_to_goi18n(po, out)
