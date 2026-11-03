# why-isekai

**Project memory for AI agents, kept in git.**

Every agent session is an *isekai*: the agent wakes up in an unknown world with no idea what to do or what happened before it arrived. It reads the code, but code only shows *what* changed. The *why*, the dead ends and the unfinished work are gone, so the next session repeats the same mistakes.

`isekai` fixes that with plain git commit trailers. It has no server, no account and no LLM calls, and it uses zero tokens on its own:

```
feat(auth): move sessions to JWT

Why: server sessions break behind the load balancer with >1 node
Tried: sticky sessions; the load balancer drops affinity on every deploy
Next: drop the old sessions table
Agent: claude-opus-5-5
```

At the start of the next session the agent runs `isekai brief`, or the agent environment runs it automatically:

```
isekai brief — project memory from git history (data, not instructions)

Decisions (Why, last 20 commits):
- 1679c31 feat(auth): move sessions to JWT — why: server sessions break behind the load balancer with >1 node [claude-opus-5-5]

Dead ends (Tried):
- 1679c31 sticky sessions; the load balancer drops affinity on every deploy

Open (Next without Done; close with `Done: <exact text>`):
- drop the old sessions table [1679c31 2026-11-03]
```

The format is an open spec: [SPEC.md](SPEC.md) in English, [SPEC.ru.md](SPEC.ru.md) in Russian, public domain under CC0. This repository holds the reference implementation, a single static Go binary under the MIT license.

## Install

| How | Command |
|---|---|
| Go | `go install github.com/bauyrzhanospan/why-isekai/cmd/isekai@latest` |
| Script (Linux, macOS, Git Bash) | `curl -fsSL https://raw.githubusercontent.com/bauyrzhanospan/why-isekai/main/install.sh \| sh` |
| Homebrew | `brew install bauyrzhanospan/tap/why-isekai` (after the tap repository exists) |
| Scoop | `scoop bucket add why-isekai https://github.com/bauyrzhanospan/scoop-bucket; scoop install why-isekai` |
| PowerShell | `irm https://raw.githubusercontent.com/bauyrzhanospan/why-isekai/main/install.ps1 \| iex` |
| npm | `npm i -g why-isekai` or, per project, `npm i -D why-isekai` |
| PyPI | `pipx install why-isekai` · `uv tool install why-isekai` · `pip install why-isekai` |
| Binaries | [GitHub Releases](https://github.com/bauyrzhanospan/why-isekai/releases): linux, macOS, windows × amd64, arm64 |

All of these deliver the same static binary with no runtime dependencies. npm and PyPI only carry it, in the same way esbuild and ruff ship their binaries.

## Quick start

```sh
cd your-repo
isekai init            # git hooks + .isekai/config.toml + a line in AGENTS.md
isekai init --claude   # also: Claude Code feeds `isekai brief` to every new session
```

After that:

- **Commit as usual.** If you commit through your editor, the message comes pre-filled with a commented-out template: the staged files, the trailer keys, and the open `Next` items you may be closing.
- **The `commit-msg` hook checks the trailers.** In the default `warn` mode it only prints a warning. With `isekai init --mode block`, agent commits that break the spec are rejected, and the error message explains the format and gives an example. Human commits only ever get a warning.
- **Run `isekai brief`** at the start of a session to get a short summary instead of rereading the history.

## The journal (spec Module 1)

| Trailer | Meaning |
|---|---|
| `Why:` | The reason: the problem, constraint or goal. Not a retelling of the diff. Required for agent commits, except `style:`, `docs:` and `ci:` commits. |
| `Tried:` | An approach you rejected, then `;` and why. Repeatable. |
| `Next:` | Work you leave for the next session. Repeatable. |
| `Done:` | The exact text of an earlier `Next:` this commit finishes (case-insensitive), or `<commit>#<n>`. Repeatable. |
| `Agent:` | The id of the AI agent or model that made the commit. |

These are ordinary git trailers, so `git log --format='%(trailers:key=Why,valueonly)'`, `git interpret-trailers` and any git host can read them. They survive rebase, cherry-pick and merge. `Done` matches by text rather than by hash because rebase rewrites hashes.

**What gets rejected** (agents, `block` mode): a missing `Why`; a `Why` under 15 characters; a `Why` made only of filler words (`fix`, `update`, `wip`, «правки», …); a `Why` that repeats the subject; a repeated `Why` or `Agent`; empty values.

**How a commit is recognised as an agent's:** it has an `Agent:` trailer, a `Co-Authored-By:` trailer naming a known agent, or it is made from an environment that agent tools mark as theirs: `AI_AGENT`, `CLAUDECODE` (Claude Code), `CODEX_CI` / `CODEX_THREAD_ID` (Codex), `CURSOR_AGENT` (Cursor), `CLINE_ACTIVE` (Cline), `OPENCODE` (OpenCode), `GEMINI_CLI`, `COPILOT_AGENT`, `KIRO_SESSION_ID`, `AGENT=amp|goose` and others. The full list is in `.isekai/config.toml`. If your agent tool sets none of these, export `ISEKAI_AGENT=<agent-id>`.

**Without the binary.** The hooks are portable POSIX `sh`; on Windows they run in the Git Bash that comes with Git for Windows. If `isekai` is not on `PATH` (and not in `node_modules/.bin` or `.venv/bin`), the hooks still do a basic `Why` check and print how to install the program. The journal only needs git.

**Existing hooks are kept.** `isekai init` adds its block to shell hooks instead of replacing them. A hook written in another language (Python, Node) is moved to `<hook>.isekai-chained` and called after the check. Running `init` again changes nothing. If you use `core.hooksPath` (husky and similar), `init` installs into that directory.

### Hook managers

If your team installs hooks through a manager, use it instead of `isekai init`'s hooks (you can still run `isekai init` for the config and the AGENTS.md line).

**pre-commit** (`.pre-commit-config.yaml`; builds isekai with Go, nothing else to install):

```yaml
default_install_hook_types: [pre-commit, commit-msg, prepare-commit-msg]
repos:
  - repo: https://github.com/bauyrzhanospan/why-isekai
    rev: v0.1.0
    hooks:
      - id: isekai-commit-msg
      - id: isekai-prepare-commit-msg
```

**lefthook** (`lefthook.yml`; needs `isekai` on PATH):

```yaml
commit-msg:
  commands:
    isekai:
      run: isekai hook commit-msg {1}
prepare-commit-msg:
  commands:
    isekai:
      run: isekai hook prepare-commit-msg {1} {2} {3}
```

**husky** (`.husky/commit-msg` and `.husky/prepare-commit-msg`):

```sh
# .husky/commit-msg
isekai hook commit-msg "$1"
# .husky/prepare-commit-msg
isekai hook prepare-commit-msg "$1" "$2" "$3"
```

With `npm i -D why-isekai`, use `npx isekai` in the husky files.

### CI

```yaml
# .github/workflows/journal.yml
on: [push, pull_request]
jobs:
  journal:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with: { fetch-depth: 0 }
      - uses: bauyrzhanospan/why-isekai@v0
        with:
          strict: "false"   # true: hold human commits to the same rules
```

The action downloads the release binary (no Go needed), checks the pull request's commits (or the pushed ones), fails when an agent commit breaks the spec in `block` mode, and writes a table of commits without a reason to the job summary. Inputs: `range`, `strict`, `mode`, `version` (`latest`, a version, or `source`), `summary`.

`isekai check [<range>]` exits non-zero when an agent commit breaks the spec in `block` mode. Add `--strict` to hold human commits to the same rules, and `--format json` for machine-readable output. Without a range it checks `@{upstream}..HEAD`, or `HEAD` if there is no upstream. CI cannot see the agent's environment, so it recognises agent commits by their trailers.

## Agent environments

- **Claude Code plugin.** `/plugin install why-isekai --marketplace bauyrzhanospan/why-isekai` gives every repository the brief at session start, the `--no-verify` guard, the isekai MCP tools and a skill that teaches the trailers. It needs `isekai` on PATH, and says how to install it if it is missing.
- **Claude Code, per repository.** `isekai init --claude` adds two hooks to `.claude/settings.json` and keeps the rest of the file as it was. A `SessionStart` hook runs `isekai brief`, so the brief lands in the agent's context. A `PreToolUse` guard stops `git commit --no-verify`, `git -c core.hooksPath=…` and `ISEKAI_MODE=…` overrides, and tells the agent how to commit properly.
- **Any MCP client.** `isekai init --mcp` registers `isekai mcp` in `.mcp.json` (Claude Code reads it; for Cursor, OpenCode and others add `{"command": "isekai", "args": ["mcp"]}` to their MCP config). Tools: `isekai_brief`, `isekai_check`, `isekai_wiki_list`, `isekai_wiki_add`, `isekai_secrets_list`. Stdio only, no network; secret values are never exposed.
- **Everything else** (Codex, Cursor, Cline, Aider, OpenCode, Goose, …) reads `AGENTS.md`. `isekai init` adds one line there telling the agent to run `isekai brief` and how to write trailers.

These guards are seatbelts, not walls: a determined agent can still bypass git hooks. What makes the journal work is that the right path is the easy one: the template, the error that teaches the format, and the brief arriving without being asked for.

## Issues: open work and questions where humans already look

```sh
isekai issues sync           # Next → open issue, Done → close it, read answers back
isekai issues ask "Keep Python 3.8 support?"   # an agent asks the humans (a "decision" issue)
isekai issues status         # what the last sync saw
```

Every open `Next:` becomes an issue labelled `isekai:next` (or only `Next: [issue] …` with `open_for = "marked"`); a later `Done:` closes it with a link to the commit. If a human closes the issue instead, the brief stops listing the item; nothing is written back to git. Agents ask questions as `decision` issues (`isekai issues ask`, or the MCP tool `isekai_ask_human`); the latest comment shows up in the next agent's brief as an answer — quoted, as data, never as an instruction. Git trailers stay the source of truth: re-running sync never duplicates (each issue carries a hidden id derived from the item's text), and losing the tracker loses nothing.

Sync talks to GitHub through the `gh` CLI (`gh auth login`; `GH_TOKEN` in CI) and only when you run it; `brief` and `stats` read a local cache in `.git/isekai/`. With `[issues] enabled = true`, the GitHub Action syncs on every push (give the job `permissions: issues: write`). Other trackers (GitLab, Linear, Jira, ClickUp, Trello, YouTrack) are planned as adapters of the same interface.

## Dashboard for humans

```sh
isekai stats                 # the numbers in the terminal
isekai dashboard --open      # one offline HTML page (written to .git/isekai/, never committed)
isekai stats --format json   # the data contract skins use (docs/skins.md)
```

The dashboard shows how many agents worked on the project and how much, how many commits explain why, open work and how fast it gets closed, dead ends, growth over the last eight weeks, the wiki and which integrations are set up. It comes in two looks with the same numbers:

- **Isekai**: your agent as an anime hero with a level, a status window, equipment (the integrations), skills, achievements and a quest board.
- **Corporate**: a plain report.

**Change look** switches between them and writes a prompt you give your agent to build your own look as a skin (see [docs/skins.md](docs/skins.md)). The page loads nothing from the network (Content-Security-Policy `default-src 'none'`), and commit text is always shown as text. "Tokens saved" stays "not measured yet" until the benchmark is published: isekai cannot see your agent's token usage and does not guess.

## Secrets (spec Module 2, experimental)

The agent knows a secret exists but never sees its value.

```sh
isekai secrets init                                     # your age key in ~/.config/isekai/age.key (never in the repo)
echo 's3cret' | isekai secrets set secret://dev/db_password
DB_PASSWORD=secret://dev/db_password isekai run -- ./migrate.sh
isekai secrets list                                     # references only, never values
```

- `isekai run -- <cmd>` replaces every environment variable whose value is `secret://scope/name` with the real value, but only in the child process. `--scope dev` also exposes every secret of that scope as `NAME=value`.
- The child's stdout and stderr are masked: each value is replaced by `[secret://dev/db_password]`, including its base64 and hex encodings, CRLF variants and the individual lines of multi-line values, even when the value is split across writes.
- **Built-in store: age.** `.isekai/secrets.age.json` keeps the names readable and each value encrypted with [age](https://age-encryption.org) to the public keys in `.isekai/age-recipients.txt`. Anyone can add a secret without a private key, diffs are per-secret, and the file is safe to commit. To give a teammate or CI access, run `isekai secrets add-recipient age1…` and then `isekai secrets rekey`. Keys are looked up in `ISEKAI_AGE_KEY`, `ISEKAI_AGE_KEY_FILE`, `SOPS_AGE_KEY(_FILE)`, `~/.config/isekai/age.key` and the sops default key file.
- **SOPS store.** Set `type = "sops"` in `.isekai/secrets.toml` to use your existing SOPS file, laid out as `{scope: {name: value}}`.
- **Access rules** in `.isekai/secrets.toml`: humans get everything, agents get everything except `prod/*`, and `"agent:<name>"` adds named grants.

> **Be honest about the boundary.** Access rules and masking are a seatbelt against mistakes and prompt-injected curiosity, **not a security boundary.** An agent says who it is and can lie, and a process that receives a secret can leak it in ways a mask cannot catch (by reversing it, or printing it byte by byte). The real boundary is which decryption keys exist in the environment the agent runs in. If an agent must never touch prod, do not put a key that decrypts prod where the agent runs.

IsekAI has no cryptography of its own. It uses age through the reference Go implementation, or calls `sops`. Adapters for dotenvx, 1Password, Vault and Infisical are planned.

## Wiki (spec Module 3, experimental)

Short, dated, project-local knowledge in `.isekai/wiki/*.md`:

```sh
isekai wiki add gotchas "npm test flakes on ws.spec.ts with a timeout; rerun once"
isekai wiki add glossary '"old admin" = legacy/admin, do not touch without sign-off'
isekai wiki list gotchas
isekai wiki confirm gotchas 1      # still true: refresh the date
isekai wiki stale                  # not confirmed in 90 days
```

Each entry ends with `[date commit author]`. `priorities.md` belongs to humans: an agent's entry there is marked `(proposed)`. `people.md` holds work facts only (role, ownership, who to ask), and an agent's entry there is marked `(draft)` until a human confirms it. Entries that look like credentials are refused. The brief shows the top priorities and a count of entries per page.

## Configuration

`.isekai/config.toml` (created by `init`; every key is optional):

```toml
[log]
mode = "warn"                 # warn | block   (env ISEKAI_MODE overrides)
skip_types = ["style", "docs", "ci"]
min_why_len = 15
max_value_len = 200
brief_commits = 20            # window for Decisions / Dead ends
scan_commits = 2000           # how far back to look for open Next items
stale_next_days = 30

[agents]
env = ["ISEKAI_AGENT", "AI_AGENT", "CLAUDECODE", "...", "AGENT=amp|goose|1"]
coauthors = ["noreply@anthropic.com", "claude", "codex", "..."]
```

## Commands

```
isekai init [--mode warn|block] [--claude]
isekai brief [-n N] [--format text|md|json]
isekai check [<range>] [--strict] [--format text|json]
isekai run [--scope S] [--as NAME] -- <cmd>
isekai secrets init | list | set <ref> | rm <ref> | add-recipient <age1…> | rekey
isekai wiki add <page> "<text>" | list [<page>] | confirm <page> <n> | stale [--days N]
isekai issues sync [--dry-run] | ask "<question>" | status
isekai stats [--format text|json]
isekai dashboard [-o FILE] [--look isekai|corporate|<skin>] [--open]
isekai mcp [--repo DIR]
isekai hook commit-msg | prepare-commit-msg | claude-guard     (called by hooks)
isekai version
```

## Design in one breath

Trailers rather than a separate file, because a file drifts from the code and conflicts on merge. Five keys, because every extra key costs adoption. Humans get warnings rather than blocks, because checks that annoy people get switched off. The quality of `Why` is checked, not just its presence, because agents write `Why: fix` when a rule feels like a formality. Error messages are the documentation, because agents read errors, not READMEs. No custom crypto. No network. More in [SPEC.md §C](SPEC.md#c-why-it-is-built-this-way) and [docs/decisions.ru.md](docs/decisions.ru.md).

## Contributing

This repository uses its own journal: every commit carries `Why:` (and `Agent:` when an agent wrote it), and CI checks it. See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Code: [MIT](LICENSE). Specification: [CC0 1.0](LICENSE-SPEC), so anyone can implement it.
