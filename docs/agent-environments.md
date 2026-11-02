# How isekai tells agents from humans

The spec (§1.5.1) treats a commit as an agent's if it has an `Agent:` trailer, a `Co-Authored-By:` trailer naming a known agent, or is made in an environment an agent tool marks as its own. The third signal depends on environment variables the tools set for the commands they run. As of 2026-11:

| Tool | Variable | Value |
|---|---|---|
| any (our own) | `ISEKAI_AGENT` | your agent id, e.g. `my-bot` |
| cross-vendor convention | `AI_AGENT` | e.g. `claude-code_<version>_agent`, `github_copilot_vscode_agent` |
| Claude Code | `CLAUDECODE` | `1` (Amp sets it too) |
| OpenAI Codex CLI | `CODEX_CI`, `CODEX_THREAD_ID`, `CODEX_SANDBOX` | `1`, a thread id, sandbox name |
| Cursor agent | `CURSOR_AGENT` | `1` |
| Cline | `CLINE_ACTIVE` | `true` |
| OpenCode | `OPENCODE` (and `AGENT=1`) | `1` |
| Gemini CLI | `GEMINI_CLI` | `1` |
| Qwen Code | `QWEN_CODE` | set |
| GitHub Copilot (VS Code) | `COPILOT_AGENT` | `1` |
| Augment | `AUGMENT_AGENT` | set |
| Antigravity | `ANTIGRAVITY_AGENT` | set |
| Kiro | `KIRO_SESSION_ID` | a session id |
| Tabnine CLI | `TABNINE_CLI` | `1` |
| Goose | `GOOSE_TERMINAL`, `AGENT=goose` | `1`, `goose` |
| Amp | `AGENT=amp` | `amp` |

Not used, because every IDE terminal sets them and they would mark humans as agents: `CURSOR_TRACE_ID`, `TERM_PROGRAM`, `REPL_ID`. Aider sets no deliberate marker; use `ISEKAI_AGENT` or the `Agent:` trailer (Aider's own `--attribute-co-authored-by` also works).

The list lives in `[agents] env` in `.isekai/config.toml` and is easy to extend. Values `0` and `false` never count; `NAME=a|b` matches only those values.

Sources: [dynatrace-oss/dtctl#622](https://github.com/dynatrace-oss/dtctl/pull/622), [agent-detect/ai-agent-detect](https://github.com/agent-detect/ai-agent-detect), [agents.md#136](https://github.com/agentsmd/agents.md/issues/136), and observing Claude Code directly.
