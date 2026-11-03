---
title: "Publish benchmark results (docs/benchmark.md + chart)"
status: open
labels: [benchmark, launch, phase-1]
---
Depends on the benchmark harness issue.

- [ ] Run the full benchmark, publish raw data.
- [ ] Write `docs/benchmark.md`: method, results table, chart, limitations, how to reproduce.
- [ ] One headline sentence for README and the launch post — only if the numbers support it. If they do not, say so and fix the product first.

**Status (2026-11-03):** Harness ready (task 3). Needs agent runs, which cost tokens: `python3 bench/run.py --runs 3 --agent ...`.
