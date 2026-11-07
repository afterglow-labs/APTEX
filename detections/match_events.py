#!/usr/bin/env python3
"""Match supplied JSONL events against one bundled Sigma rule.

pySigma parses the rule; the existing regression matcher evaluates its parsed
condition tree. This is a local reference matcher, not a SIEM query or proof
that a production sensor collected the event.
"""

import argparse
import hashlib
import importlib.util
import json
from pathlib import Path
import re

ROOT = Path(__file__).resolve().parent
RULES = ROOT / "sigma"
MATCHER = ROOT / "tests" / "sigma_match" / "check.py"
MAX_BYTES = 16 * 1024 * 1024
MAX_EVENTS = 50000


def load_matcher():
    spec = importlib.util.spec_from_file_location("aptex_sigma_matcher", MATCHER)
    if spec is None or spec.loader is None:
        raise RuntimeError("bundled Sigma matcher is unavailable")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    module.SIGMA_DIR = str(RULES)
    module.CORR_DIR = str(RULES / "correlation")
    return module


def read_events(path):
    source = Path(path).resolve(strict=True)
    if not source.is_file() or source.stat().st_size > MAX_BYTES:
        raise ValueError("event file is missing or exceeds 16 MiB")
    events = []
    digest = hashlib.sha256()
    with source.open("rb") as stream:
        for index, raw in enumerate(stream, start=1):
            digest.update(raw)
            if index > MAX_EVENTS:
                raise ValueError("event file exceeds 50000 JSONL records")
            try:
                event = json.loads(raw)
            except (ValueError, UnicodeDecodeError) as error:
                raise ValueError(f"invalid JSON on line {index}: {error}") from error
            if not isinstance(event, dict):
                raise ValueError(f"line {index} must be a JSON object")
            events.append(event)
    return events, digest.hexdigest()


def evaluate_rule(stem, events, digest, correlation=False):
    if not re.fullmatch(r"[a-z0-9][a-z0-9_-]{0,127}", stem):
        raise ValueError("rule must be a bundled filename stem")
    matcher = load_matcher()
    if correlation:
        rules = matcher.load_correlation_rules()
        if stem not in rules:
            raise ValueError("unknown bundled correlation rule")
        matcher.require_ts(events, stem, "operator events")
        fired = matcher.correlation_fires(rules[stem], events)
        return {"rule": stem, "kind": "correlation", "event_count": len(events),
                "event_sha256": digest, "fired": fired,
                "limitations": "Reference sliding-window matcher; compare field mappings and timing with the production SIEM."}

    path = RULES / f"{stem}.yml"
    if not path.is_file():
        raise ValueError("unknown bundled atomic rule")
    collection = matcher.SigmaCollection.from_yaml(path.read_text(encoding="utf-8"))
    rules = [rule for rule in collection.rules if type(rule).__name__ == "SigmaRule"]
    if len(rules) != 1:
        raise ValueError("expected exactly one atomic Sigma rule")
    hits = [i + 1 for i, event in enumerate(events) if matcher.rule_matches(rules[0], event)]
    return {"rule": stem, "kind": "atomic", "event_count": len(events),
            "event_sha256": digest, "match_count": len(hits), "match_lines": hits[:100],
            "lines_truncated": len(hits) > 100,
            "limitations": "Reference case-insensitive matcher; production SIEM field mappings and event collection need separate verification."}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--list", action="store_true", help="list bundled rule stems")
    parser.add_argument("--rule", help="bundled rule filename stem")
    parser.add_argument("--events", help="path to operator-supplied JSONL events")
    parser.add_argument("--correlation", action="store_true", help="evaluate a correlation rule")
    args = parser.parse_args()
    if args.list:
        print(json.dumps({"atomic": sorted(p.stem for p in RULES.glob("*.yml")),
                          "correlation": sorted(p.stem for p in (RULES / "correlation").glob("*.yml"))}))
        return
    if not args.rule or not args.events:
        parser.error("--rule and --events are required unless --list is used")
    try:
        events, digest = read_events(args.events)
        print(json.dumps(evaluate_rule(args.rule, events, digest, args.correlation), ensure_ascii=False))
    except (OSError, ValueError, RuntimeError) as error:
        parser.exit(2, f"match_events: {error}\n")


if __name__ == "__main__":
    main()
