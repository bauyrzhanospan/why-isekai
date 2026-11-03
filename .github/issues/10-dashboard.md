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

**Two looks, same numbers** (`[ui] look = "game" | "corporate"`)
- **Isekai (game):** the agent is a hero. Level and XP come from the journal (10 XP per commit, +5 per Why, +15 per dead end, +20 per closed Next). Attributes are the metrics: Wisdom = Why coverage, Discipline = commits passing the check, Memory = dead ends, Quests = open Next, Lore = wiki entries, Secrecy = secrets by reference. Equipment slots are integrations (hooks, block mode, Claude Code brief, no-verify guard, secrets, tracker sync, MCP, notifications): an empty slot says how to equip it. Abilities are the features an agent has learned; achievements reward habits (first Why, dead ends mapped, quests closed). Original character art, no third-party IP.
- **Corporate:** the plain dashboard for teams and reports; companies can make it the default and hide the game look.
- The dashboard also edits `.isekai/config.toml` (settings page with a live preview of the file).

**Surfaces**
- `isekai stats [--format text|json]` — terminal summary.
- `isekai dashboard [-o isekai.html]` — one self-contained HTML file (inline CSS/JS, no CDN, works offline, light/dark), opens in any browser.
- The GitHub Action (task 8) writes the same numbers to the job summary and can publish the page to GitHub Pages.

**Done when** `isekai dashboard` on this repository produces a page a non-programmer understands in 10 seconds, with tests for every metric on a synthetic repo.
