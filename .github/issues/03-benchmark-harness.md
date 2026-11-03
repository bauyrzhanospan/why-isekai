---
title: "Benchmark harness: agent sessions with vs without the journal"
status: done
labels: [benchmark, phase-1]
---
Without numbers this is "yet another commit convention". We need a reproducible measurement.

**Design**
- 10–20 two-session tasks on real open-source repos: session 1 does part of the work (and hits a dead end on purpose); session 2 must continue in a fresh context.
- Two arms: A = plain git history; B = same history with trailers + `isekai brief` at session start.
- Metrics: total tokens, tool calls/attempts, wall time, task success (tests pass), how often session 2 retries the known dead end.
- Agents: at least Claude Code; ideally also Codex and OpenCode. ≥3 runs per task per arm.
- Everything scripted and published: tasks, prompts, seeds, raw logs, analysis notebook.

**Done when** `bench/` runs end to end with one command and produces a results table.

**Status (2026-11-03):** Done in 2857084: bench/run.py with two arms, three tasks with reference solutions, --self-test in CI, bench/analyze.py. Growing to 10–20 tasks is tracked in task 4.
