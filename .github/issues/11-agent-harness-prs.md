---
title: "PRs to open agents: read the brief, write trailers by default"
labels: [adoption, phase-2]
---
Tracking issue. If one agent writes `Why:` by default, that beats any marketing.

For each of OpenCode, Cline, Aider, Goose:
- [ ] Study how it builds session context and commit messages.
- [ ] Open a discussion first, then a PR: read `isekai brief` (or raw trailers via `git log`, no dependency) at session start; add `Why:`/`Agent:` trailers to commits it creates.
- [ ] Keep the change small and optional (a setting), with no dependency on our binary.

Later: Claude Code, Codex, Cursor (closed — via their plugin/rules systems and feedback channels).
