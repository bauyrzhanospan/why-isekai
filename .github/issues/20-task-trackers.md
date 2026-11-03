---
title: "Optional: task tracker adapters (Jira, Linear, ClickUp, Trello, GitLab, YouTrack)"
labels: [integration, optional, later]
---
Many teams live in a tracker, not in GitHub issues. Same idea as task 9, through adapters.

**Design**
- Task 9 defines one adapter interface: `open(item)`, `close(item, commit)`, `list_open()`, `list_answers(decision)`. GitHub Issues is the first adapter; every tracker here is another one.
- Mapping per tracker: Next → task/card/issue; Done → move to done column/status; `decision` → a task assigned to a human with a "question" label; the human's comment → answer in the brief.
- Configured in `.isekai/config.toml` `[tracker] type = "github" | "gitlab" | "jira" | "linear" | "clickup" | "trello" | "youtrack"`, plus project/board ids. Tokens never in the repo: through the secrets module (`secret://ci/jira_token`).
- Runs where task 9 runs: in CI (GitHub Action / GitLab CI) or via explicit `isekai tracker sync`. The core CLI stays offline (spec §B.3).
- Git trailers stay the source of truth; a tracker is a view plus an answer channel. Switching trackers loses nothing.

**Order**: GitLab issues (closest to GitHub) → Linear → Jira → ClickUp → Trello → YouTrack. Each adapter ships with tests against a recorded fake API, never live calls in CI.

**Done when** a team on Jira sees agents' open work and questions as Jira tasks and answers them there, with no change to how agents commit.
