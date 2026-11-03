#!/usr/bin/env python3
"""Summarise bench results: python3 bench/analyze.py bench/results.jsonl"""
import json
import statistics
import sys
from collections import defaultdict


def mean(xs):
    xs = [x for x in xs if x is not None]
    return statistics.mean(xs) if xs else None


def fmt(x, nd=0):
    return "—" if x is None else ("%.*f" % (nd, x))


rows = [json.loads(l) for l in open(sys.argv[1] if len(sys.argv) > 1 else "bench/results.jsonl") if l.strip()]
by = defaultdict(list)
for r in rows:
    by[(r["task"], r["arm"])].append(r)
by_arm = defaultdict(list)
for r in rows:
    by_arm[r["arm"]].append(r)

print("| Task | Arm | Runs | Success | Dead end repeated | Tokens (mean) | Turns (mean) | Time, s |")
print("|---|---|---|---|---|---|---|---|")
for (task, arm), rs in sorted(by.items()):
    print("| %s | %s | %d | %d%% | %d%% | %s | %s | %s |" % (
        task, arm, len(rs), 100 * sum(r["success"] for r in rs) / len(rs), 100 * sum(r["dead_end"] for r in rs) / len(rs),
        fmt(mean([r["tokens"] for r in rs])), fmt(mean([r["turns"] for r in rs]), 1), fmt(mean([r["wall_s"] for r in rs]))))
print()
print("| Arm | Runs | Success | Dead end repeated | Tokens (mean) |")
print("|---|---|---|---|---|")
for arm, rs in sorted(by_arm.items()):
    print("| %s | %d | %d%% | %d%% | %s |" % (arm, len(rs), 100 * sum(r["success"] for r in rs) / len(rs),
                                        100 * sum(r["dead_end"] for r in rs) / len(rs), fmt(mean([r["tokens"] for r in rs]))))
p, j = mean([r["tokens"] for r in by_arm.get("plain", [])]), mean([r["tokens"] for r in by_arm.get("journal", [])])
if p and j:
    print("\nTokens with the journal vs without: %+.0f%%" % (100 * (j - p) / p))
