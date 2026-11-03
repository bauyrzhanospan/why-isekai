---
title: "Claude Code plugin / skill for why-isekai"
status: done
labels: [integration, phase-2]
---
Package what `isekai init --claude` does (SessionStart brief, PreToolUse guard) plus a skill that teaches the trailer format, as an installable Claude Code plugin, so users get it without editing settings.

**Done when** installing the plugin in a repo with isekai gives the brief at session start and trailers in the agent's commits, documented in README.

**Status (2026-11-03):** Done in fe62572: plugins/claude-code (SessionStart brief, --no-verify guard, MCP server, why-isekai skill) and .claude-plugin/marketplace.json; both pass `claude plugin validate`.
