---
title: "`isekai mcp`: MCP server exposing brief, check and wiki"
labels: [integration, phase-2]
---
MCP is how agents pick up tools now. Same binary, new subcommand, stdio transport.

**Tools**: `isekai_brief(n, format)`, `isekai_check(range)`, `isekai_wiki_list(page)`, `isekai_wiki_add(page, text)` (same guards as CLI), `isekai_secrets_list()` (refs only). Optionally a `prompt` that explains the trailer format.
**Rules**: no network, read-only by default except wiki add; secret values never exposed.
**Done when** it works from Claude Code, Cursor and OpenCode configs documented in README, with an integration test speaking JSON-RPC over stdio.
