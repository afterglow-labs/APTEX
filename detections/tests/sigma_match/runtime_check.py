#!/usr/bin/env python3
"""Exercise the shipped JSONL matcher with positive and negative source fixtures."""

import json
from pathlib import Path
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[2]
CLI = ROOT / "match_events.py"
FIXTURES = ROOT / "tests" / "sigma_match" / "events"


def call(rule, events, correlation=False):
    with tempfile.TemporaryDirectory() as area:
        source = Path(area) / "events.jsonl"
        source.write_text("".join(json.dumps(event) + "\n" for event in events), encoding="utf-8")
        args = [sys.executable, str(CLI), "--rule", rule, "--events", str(source)]
        if correlation:
            args.append("--correlation")
        output = subprocess.check_output(args, text=True)
        return json.loads(output)


def main():
    for rule_file in sorted((ROOT / "sigma").glob("*.yml")):
        fixture = json.loads((FIXTURES / f"{rule_file.stem}.json").read_text(encoding="utf-8"))
        events = [fixture["positive"][0], fixture["negative"][0]]
        result = call(rule_file.stem, events)
        if result["match_count"] != 1 or result["match_lines"] != [1] or result["event_count"] != 2:
            raise AssertionError((rule_file.stem, result))
    for rule_file in sorted((ROOT / "sigma" / "correlation").glob("*.yml")):
        fixture = json.loads((FIXTURES / "correlation" / f"{rule_file.stem}.json").read_text(encoding="utf-8"))
        positive = call(rule_file.stem, fixture["positive"][0]["events"], True)
        negative = call(rule_file.stem, fixture["negative"][0]["events"], True)
        if not positive["fired"] or negative["fired"]:
            raise AssertionError((rule_file.stem, positive, negative))
    with tempfile.TemporaryDirectory() as area:
        source = Path(area) / "events.jsonl"
        source.write_text("{}\n", encoding="utf-8")
        rejected = subprocess.run([sys.executable, str(CLI), "--rule", "../escape", "--events", str(source)],
                                  capture_output=True, text=True)
        if rejected.returncode != 2 or "bundled filename stem" not in rejected.stderr:
            raise AssertionError("rule traversal was not rejected")
    print("PASS runtime matcher: atomic positives/negatives, correlation timelines, and rule isolation")


if __name__ == "__main__":
    main()
