#!/usr/bin/env python3
"""Compare two k6 JSON summary exports and print a comparison table.

Usage:
    k6 run --summary-export=no-cache.json scripts/loadtest/redirect.js
    # enable cache, restart server
    k6 run --summary-export=with-cache.json scripts/loadtest/redirect.js
    python3 scripts/loadtest/compare.py no-cache.json with-cache.json
"""

import json
import sys


def load_metrics(path):
    """Load k6 summary JSON and extract key metrics."""
    with open(path) as f:
        data = json.load(f)

    m = data["metrics"]
    reqs = m.get("http_reqs", {})
    failed = m.get("http_req_failed", {})
    dur = m.get("http_req_duration", {})

    return {
        "requests": reqs.get("count", 0),
        "rate": reqs.get("rate", 0),
        "error_rate": failed.get("value", 0) * 100,
        "med": dur.get("med", 0),
        "p90": dur.get("p(90)", 0),
        "p95": dur.get("p(95)", 0),
        "p99": dur.get("p(99)", 0),
        "avg": dur.get("avg", 0),
        "max": dur.get("max", 0),
    }


def pct_change(old, new):
    """Return improvement percentage (positive = faster)."""
    if old == 0:
        return 0
    return ((old - new) / old) * 100


def main():
    if len(sys.argv) != 3:
        print(f"Usage: {sys.argv[0]} <no-cache.json> <with-cache.json>")
        sys.exit(1)

    before = load_metrics(sys.argv[1])
    after = load_metrics(sys.argv[2])

    print()
    print("=" * 70)
    print("  LOAD TEST COMPARISON: No Cache vs With Cache")
    print("=" * 70)
    print()
    print(f"  {'Metric':<20} {'No Cache':>12} {'With Cache':>12} {'Improvement':>14}")
    print(f"  {'-'*20} {'-'*12} {'-'*12} {'-'*14}")

    # Rate
    rate_change = ((after["rate"] - before["rate"]) / before["rate"] * 100) if before["rate"] else 0
    print(f"  {'Requests/sec':<20} {before['rate']:>11.1f}/s {after['rate']:>11.1f}/s {rate_change:>+13.1f}%")

    # Error rate
    err_diff = before["error_rate"] - after["error_rate"]
    print(f"  {'Error rate':<20} {before['error_rate']:>11.2f}% {after['error_rate']:>11.2f}% {err_diff:>+13.2f}pp")

    # Latencies
    for key, label in [
        ("med", "p50 (median)"),
        ("p90", "p90 latency"),
        ("p95", "p95 latency"),
        ("p99", "p99 latency"),
        ("avg", "Avg latency"),
        ("max", "Max latency"),
    ]:
        imp = pct_change(before[key], after[key])
        print(f"  {label:<20} {before[key]:>11.2f}ms {after[key]:>11.2f}ms {imp:>+13.1f}%")

    print()
    print(f"  Total requests: {before['requests']} (no cache) vs {after['requests']} (with cache)")
    print("=" * 70)

    # Summary
    p95_imp = pct_change(before["p95"], after["p95"])
    p50_imp = pct_change(before["med"], after["med"])
    print(f"\n  Key takeaway: p50 improved by {p50_imp:.1f}%, p95 by {p95_imp:.1f}%")
    print(f"                Throughput change: {rate_change:+.1f}%")
    print()


if __name__ == "__main__":
    main()
