---
title: "Project dashboard for humans: `isekai stats` and a one-page HTML view"
labels: [journal, ux, phase-2]
---
A vibe coder should see at a glance what the agents have been doing and what IsekAI gives them, without reading git log.

**Numbers (all computed from git, deterministic, offline)**
- Agents: how many distinct agents worked on the project (`Agent:` trailers, `Co-Authored-By`), commits per agent, agent vs human share over time.
- Journal health: % of commits with Why (overall, last 30 days), filler Whys rejected (from hook logs, if kept), dead ends recorded.
- Open work: open Next items, oldest open item, average time from Next to Done, items closed this week; with issues sync (task 9): open issues and "waiting for humans".
- Growth: commits per week, active days, files and lines changed per week, contributors (agents and humans).
- Wiki: entries per page, stale entries. Secrets: number of references (never values).

**Tokens saved — honest estimate only.** We cannot see an agent's token usage. Show `briefs served × average saving per session`, where the saving comes from our published benchmark (tasks 3–4), labelled as an estimate with a link to the method. Briefs served are counted locally in `.git/isekai/stats.json` (never committed, never sent anywhere). Until the benchmark exists, show briefs served and dead ends surfaced, not a token number.

**Surfaces**
- `isekai stats [--format text|json]` — terminal summary.
- `isekai dashboard [-o isekai.html]` — one self-contained HTML file (inline CSS/JS, no CDN, works offline, light/dark), opens in any browser.
- The GitHub Action (task 8) writes the same numbers to the job summary and can publish the page to GitHub Pages.

**Done when** `isekai dashboard` on this repository produces a page a non-programmer understands in 10 seconds, with tests for every metric on a synthetic repo.
