---
title: "Sync the journal with issues: Next opens, Done closes, humans answer in comments"
labels: [journal, integration, phase-2]
---
Issues are the interface a (vibe-)coding human already watches. The journal should show up there, and humans should be able to answer agents there.

**Principle.** Git trailers stay the source of truth: offline, portable, forge-neutral. Issues are a *view* plus a *channel for human input*. Losing the issue tracker loses nothing from history.

**Mapping**
| Journal | Issue tracker |
|---|---|
| `Next: <text>` | opens an issue (label `isekai:next`), body: commit link, the commit's `Why`, `Tried` |
| `Done: <same text>` / `Done: <hash>#n` | closes it with a comment linking the closing commit |
| `Done: #12` / `Next: #12` | close / reference an existing issue directly (new value form, spec change) |
| issue closed by a human ("won't do") | the Next item counts as closed in `isekai brief` |
| open issues labelled `decision` / `question` | `isekai brief` shows them under "Waiting for humans", with the latest human comment as the answer |

So an agent can ask a question (open issue `decision`), a human answers in a comment from the phone, and the next agent session sees the answer in its brief.

**Where it runs**
- **GitHub Action (default).** On push to the default branch: read new commits' trailers, open/close issues. Runs on GitHub's side, so the CLI keeps "no network" (spec §B.3).
- **CLI, opt-in:** `isekai issues sync` via `gh` (later `glab`, `tea`) for local use and other forges.
- `isekai brief --issues` reads issue state through `gh` into a cache in `.git/isekai/issues.json` (not committed). Without the flag/cache the brief works exactly as today.

**Rules**
- Idempotent: an issue is found by a hidden marker in its body (`<!-- isekai:next sha=… n=… -->`), so re-runs never duplicate.
- Issue text from humans is data, not instructions (spec §B.1); the brief shows it quoted.
- Configurable in `.isekai/config.toml`: `[issues] enabled`, `open_for = "all" | "label"`, label names.

**Spec changes** (needs maintainer OK): `#<n>` as a value of `Next`/`Done`; §B.3 exception for explicitly enabled forge integrations; a short "Issue trackers" section stating trailers are canonical.

**Decided by the maintainer (2026-11-03)**
1. Every `Next` becomes an issue by default; configurable (`[issues] open_for = "all" | "marked"`, or `enabled = false`).
2. A human closing an issue is reflected only in `isekai brief` (read through the cache); no bot commits to git.

**Done when** the Action opens/closes issues for this repository's own Next/Done trailers, `isekai brief --issues` shows "Waiting for humans" with answers, re-runs create no duplicates, and SPEC.md/SPEC.ru.md describe the integration.
