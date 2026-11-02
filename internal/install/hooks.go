// Package install implements `isekai init`: git hooks, config, AGENTS.md
// and agent-environment integrations. Every step is idempotent and never
// overwrites a user's existing hook.
package install

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	beginMarker = "# >>> isekai >>>"
	endMarker   = "# <<< isekai <<<"
	// ChainSuffix is appended to a non-shell hook we had to move aside.
	ChainSuffix = ".isekai-chained"
)

// Action describes what happened to one file.
type Action string

const (
	Created   Action = "created"
	Updated   Action = "updated"
	Unchanged Action = "unchanged"
	Chained   Action = "chained"  // existing non-shell hook moved aside and called from ours
	Inserted  Action = "inserted" // block added to an existing shell hook
)

const installHint = `npx why-isekai --help | pip install why-isekai | brew install why-isekai | go install github.com/bauyrzhanospan/why-isekai/cmd/isekai@latest`

// commitMsgBlock runs `isekai hook commit-msg`, or a basic POSIX sh check of
// the Why trailer when isekai is not on PATH, so the journal keeps working
// with nothing but git (handoff §5).
const commitMsgBlock = beginMarker + ` managed by "isekai init"; edit outside this block
_isekai=$(command -v isekai 2>/dev/null)
for _c in node_modules/.bin/isekai .venv/bin/isekai venv/bin/isekai; do
  if [ -z "$_isekai" ] && [ -x "$_c" ]; then _isekai=$_c; fi
done
if [ -n "$_isekai" ]; then
  "$_isekai" hook commit-msg "$@" || exit $?
else
  isekai_basic_check() {
    _msg=$(sed -e '/^# -\{24\} >8 -\{24\}$/,$d' "$1" | grep -v '^#')
    _subject=$(printf '%s\n' "$_msg" | sed -n '/[^[:space:]]/{p;q;}')
    case "$_subject" in
      ''|'Merge '*|'fixup! '*|'squash! '*|'amend! '*|'Revert "'*) return 0 ;;
    esac
    _type=$(printf '%s\n' "$_subject" | sed -n 's/^\([A-Za-z]*\)\(([^)]*)\)\{0,1\}!\{0,1\}:.*/\1/p' | tr 'A-Z' 'a-z')
    case "$_type" in style|docs|ci) return 0 ;; esac
    _why=$(printf '%s\n' "$_msg" | sed -n 's/^[Ww][Hh][Yy][[:space:]]*:[[:space:]]*//p' | sed -n '1p')
    _agent=0
    if printf '%s\n' "$_msg" | grep -Eqi '^(agent[[:space:]]*:|co-authored-by:.*(anthropic|claude|codex|openai|cursor|copilot|aider|devin|gemini|opencode|cline))'; then _agent=1; fi
    for _v in "${ISEKAI_AGENT:-}" "${AI_AGENT:-}" "${CLAUDECODE:-}" "${CODEX_CI:-}" "${CODEX_THREAD_ID:-}" "${CURSOR_AGENT:-}" "${CLINE_ACTIVE:-}" "${OPENCODE:-}" "${GEMINI_CLI:-}" "${COPILOT_AGENT:-}"; do
      case "$_v" in ''|0|false) ;; *) _agent=1 ;; esac
    done
    _mode=warn
    if [ -f .isekai/config.toml ] && grep -Eq '^[[:space:]]*mode[[:space:]]*=[[:space:]]*"block"' .isekai/config.toml; then _mode=block; fi
    [ -n "${ISEKAI_MODE:-}" ] && _mode=$ISEKAI_MODE
    if [ -z "$_why" ]; then _problem='missing "Why:" trailer'
    elif [ "${#_why}" -lt 15 ]; then _problem='"Why:" is too short to explain the reason'
    else return 0; fi
    {
      echo "isekai (basic check; the isekai program is not installed): $_problem"
      echo '  Add a last paragraph, after a blank line:  Why: <the problem, constraint or goal>'
      echo '  Example:  Why: server sessions break behind the load balancer with >1 node'
      echo '  Commits typed style:, docs:, ci: do not need Why.'
      echo '  Install isekai for full checks and "isekai brief": ` + installHint + `'
    } >&2
    if [ "$_mode" = block ] && [ "$_agent" = 1 ]; then return 1; fi
    return 0
  }
  isekai_basic_check "$1" || exit 1
fi
` + endMarker + "\n"

const prepareMsgBlock = beginMarker + ` managed by "isekai init"; edit outside this block
_isekai=$(command -v isekai 2>/dev/null)
for _c in node_modules/.bin/isekai .venv/bin/isekai venv/bin/isekai; do
  if [ -z "$_isekai" ] && [ -x "$_c" ]; then _isekai=$_c; fi
done
if [ -n "$_isekai" ]; then
  "$_isekai" hook prepare-commit-msg "$@" || true
else
  case "${2:-}" in
    ''|template) printf '\n# isekai: end the message with "Why: <reason>" (optional: Tried, Next, Done, Agent).\n' >> "$1" ;;
  esac
fi
` + endMarker + "\n"

// Hooks maps hook name to the block we manage in it.
var Hooks = map[string]string{
	"prepare-commit-msg": prepareMsgBlock,
	"commit-msg":         commitMsgBlock,
}

// HookNames in install order.
var HookNames = []string{"prepare-commit-msg", "commit-msg"}

func isShellShebang(line string) bool {
	if !strings.HasPrefix(line, "#!") {
		return false
	}
	f := strings.Fields(strings.TrimPrefix(line, "#!"))
	if len(f) == 0 {
		return false
	}
	prog := filepath.Base(f[0])
	if prog == "env" {
		for _, a := range f[1:] {
			if !strings.HasPrefix(a, "-") {
				prog = filepath.Base(a)
				break
			}
		}
	}
	switch prog {
	case "sh", "bash", "dash", "ash", "ksh", "zsh", "mksh", "busybox":
		return true
	}
	return false
}

// InstallHook puts block into the hook at path without disturbing anything
// else in it.
func InstallHook(path, block string) (Action, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", err
		}
		return Created, writeExec(path, "#!/bin/sh\n"+block)
	}
	if err != nil {
		return "", err
	}
	s := strings.ReplaceAll(string(data), "\r\n", "\n")

	// Already ours: refresh the block in place.
	if i := strings.Index(s, beginMarker); i >= 0 {
		j := strings.Index(s[i:], endMarker)
		if j < 0 {
			return "", fmt.Errorf("%s: found %q without %q; fix the hook by hand", path, beginMarker, endMarker)
		}
		end := i + j + len(endMarker)
		if end < len(s) && s[end] == '\n' {
			end++
		}
		ns := s[:i] + block + s[end:]
		if ns == string(data) {
			return Unchanged, ensureExec(path)
		}
		return Updated, writeExec(path, ns)
	}

	first, rest, _ := strings.Cut(s, "\n")
	switch {
	case isShellShebang(first):
		return Inserted, writeExec(path, first+"\n"+block+rest)
	case !strings.HasPrefix(first, "#!"):
		// No shebang: git runs it with sh (husky-style hook files).
		return Inserted, writeExec(path, "#!/bin/sh\n"+block+s)
	default:
		// Python, node, etc.: move it aside and call it after our block.
		chained := path + ChainSuffix
		if _, err := os.Stat(chained); err == nil {
			return "", fmt.Errorf("%s exists; remove it or merge the hooks by hand", chained)
		}
		if err := os.Rename(path, chained); err != nil {
			return "", err
		}
		name := filepath.Base(chained)
		wrapper := "#!/bin/sh\n" + block + `exec "$(dirname "$0")/` + name + `" "$@"` + "\n"
		return Chained, writeExec(path, wrapper)
	}
}

func writeExec(path, content string) error {
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		return err
	}
	return ensureExec(path)
}

func ensureExec(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	if st.Mode()&0o111 != 0o111 {
		return os.Chmod(path, st.Mode()|0o111)
	}
	return nil
}
