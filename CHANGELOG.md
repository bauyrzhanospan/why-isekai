# Changelog

## 0.1.0 (2026-11-03)

First version of the reference implementation of the why-isekai spec 0.1.0-draft.

- Journal: `isekai init` (git hooks that keep existing hooks, config, AGENTS.md line, `--claude`), `commit-msg` validation with warn/block modes and quality checks for `Why`, `prepare-commit-msg` template, POSIX `sh` fallback when the binary is missing, `isekai brief` (text/md/json), `isekai check` for CI.
- Agent detection: `Agent` / `Co-Authored-By` trailers and environment variables of Claude Code, Codex, Cursor, Cline, OpenCode, Gemini CLI, Copilot, Amp, Goose, Kiro and others.
- Claude Code: `SessionStart` brief and a `PreToolUse` guard against `--no-verify`.
- Secrets (experimental): `secret://scope/name` references, built-in age store, SOPS store, `isekai run` with output masking (raw, base64, hex, per line), access rules.
- Wiki (experimental): `isekai wiki add | list | confirm | stale`, guards for `priorities.md` and `people.md`, credential detection.
- Warm brief from day one (agents, hot spots, recent TODOs, commits without a reason).
- `isekai mcp`: MCP server with brief, check, wiki, ask-a-human and secrets list.
- Claude Code plugin and marketplace entry; pre-commit, lefthook and husky recipes; GitHub Action `bauyrzhanospan/why-isekai@v0`.
- Issue sync (GitHub, through `gh`): Next opens an issue, Done closes it, maintainers answer agents in comments.
- `isekai stats` and `isekai dashboard`: one offline HTML page with the isekai and corporate looks and project skins (`isekai ui new | check | use | list`).
- Secrets: command-based stores for dotenvx, 1Password, Vault and Infisical (presets tested against fake CLIs only).
- Benchmark harness in `bench/` (results not published yet).
- Packaging: GoReleaser for linux/darwin/windows × amd64/arm64, npm (esbuild scheme) and PyPI (ruff scheme) packagers (this release is published to npm only; PyPI comes later), `install.sh`, `install.ps1`.
