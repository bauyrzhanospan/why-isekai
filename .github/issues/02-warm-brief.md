---
title: "Warm brief: `isekai brief` is useful on day one, before any trailers exist"
status: done
labels: [journal, adoption, phase-1]
---
Cold start is the main adoption killer: on a fresh install `brief` is empty, and people give up before trailers accumulate.

**Idea** — when trailers are missing or few, fill the brief from what git already has:
- recent commit subjects (grouped, deduplicated);
- hot files/directories of the last N commits;
- commits made by agents (`Co-Authored-By`, `Agent`) and by whom;
- reverts (`Revert "…"`) as implicit dead ends;
- `TODO`/`FIXME` added in recent diffs as implicit open items (clearly labelled as inferred);
- a one-line nudge: "N of the last 20 commits have no Why — see AGENTS.md".

**Rules**: deterministic, no LLM, no network; inferred items are marked as inferred; stays short (target < 1500 tokens for `-n 20`).

**Done when** `isekai brief` on a large repo with zero trailers (e.g. a clone of cobra or ripgrep) gives a summary a human would call useful, with tests.

**Status (2026-11-03):** Done in c9d4fd8: recent commits without Why, reverts as inferred dead ends, TODO/FIXME still in the code, hot spots, agents. On spf13/cobra the brief is 1.1 KB and takes 20 ms.
