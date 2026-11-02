# Changelog

## Unreleased (0.1.0)

First version of the reference implementation of the why-isekai spec 0.1.0-draft.

- Journal: `isekai init` (git hooks that keep existing hooks, config, AGENTS.md line, `--claude`), `commit-msg` validation with warn/block modes and quality checks for `Why`, `prepare-commit-msg` template, POSIX `sh` fallback when the binary is missing, `isekai brief` (text/md/json), `isekai check` for CI.
- Agent detection: `Agent` / `Co-Authored-By` trailers and environment variables of Claude Code, Codex, Cursor, Cline, OpenCode, Gemini CLI, Copilot, Amp, Goose, Kiro and others.
- Claude Code: `SessionStart` brief and a `PreToolUse` guard against `--no-verify`.
- Secrets (experimental): `secret://scope/name` references, built-in age store, SOPS store, `isekai run` with output masking (raw, base64, hex, per line), access rules.
- Wiki (experimental): `isekai wiki add | list | confirm | stale`, guards for `priorities.md` and `people.md`, credential detection.
- Packaging: GoReleaser for linux/darwin/windows × amd64/arm64, npm (esbuild scheme) and PyPI (ruff scheme) packagers, `install.sh`, `install.ps1`.
