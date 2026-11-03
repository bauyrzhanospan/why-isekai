// Package config loads .isekai/config.toml.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Dir is the per-repository directory IsekAI owns.
const Dir = ".isekai"

// File is the config path relative to the repository root.
const File = Dir + "/config.toml"

// Mode controls what the validator does with violations in agent commits.
type Mode string

const (
	ModeWarn  Mode = "warn"
	ModeBlock Mode = "block"
)

type Log struct {
	Mode          Mode     `toml:"mode"`
	SkipTypes     []string `toml:"skip_types"`
	MinWhyLen     int      `toml:"min_why_len"`
	MaxValueLen   int      `toml:"max_value_len"`
	BriefCommits  int      `toml:"brief_commits"`
	ScanCommits   int      `toml:"scan_commits"`
	StaleNextDays int      `toml:"stale_next_days"`
}

type Agents struct {
	// Env lists environment variables that mark the committer as an agent:
	// "NAME" matches when NAME is set and not "0"/"false"; "NAME=a|b" matches
	// only those values (for generic names like AGENT).
	Env []string `toml:"env"`
	// Coauthors lists case-insensitive substrings of a Co-Authored-By value
	// that mark the commit as agent-authored.
	Coauthors []string `toml:"coauthors"`
}

// UI configures the dashboard.
type UI struct {
	Look string `toml:"look"` // isekai, corporate or a skin in .isekai/ui/skins
}

type Config struct {
	Log    Log    `toml:"log"`
	Agents Agents `toml:"agents"`
	UI     UI     `toml:"ui"`
}

// Default returns the built-in configuration.
func Default() Config {
	return Config{
		Log: Log{
			Mode:          ModeWarn,
			SkipTypes:     []string{"style", "docs", "ci"},
			MinWhyLen:     15,
			MaxValueLen:   200,
			BriefCommits:  20,
			ScanCommits:   2000,
			StaleNextDays: 30,
		},
		UI: UI{Look: "isekai"},
		Agents: Agents{
			Env: DefaultAgentEnv,
			Coauthors: []string{
				"noreply@anthropic.com", "claude", "codex", "openai", "cursor",
				"copilot", "aider", "devin", "gemini", "jules", "opencode",
				"cline", "goose",
			},
		},
	}
}

// Load reads <root>/.isekai/config.toml over the defaults. A missing file is
// not an error. ISEKAI_MODE overrides log.mode.
func Load(root string) (Config, error) {
	c := Default()
	data, err := os.ReadFile(filepath.Join(root, File))
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return c, err
	default:
		if _, err := toml.Decode(string(data), &c); err != nil {
			return c, fmt.Errorf("%s: %w", File, err)
		}
	}
	if m := os.Getenv("ISEKAI_MODE"); m != "" {
		c.Log.Mode = Mode(strings.ToLower(m))
	}
	if c.Log.Mode != ModeWarn && c.Log.Mode != ModeBlock {
		return c, fmt.Errorf("%s: log.mode must be \"warn\" or \"block\", got %q", File, c.Log.Mode)
	}
	return c, nil
}

// Template is written by `isekai init`.
func Template(mode Mode) string {
	return fmt.Sprintf(`# why-isekai config. Spec: https://github.com/bauyrzhanospan/why-isekai/blob/main/SPEC.md
# All keys are optional; the values below are the defaults.

[log]
# warn: print problems, never reject. block: reject agent commits that break the spec.
# Human commits only ever get warnings. Env ISEKAI_MODE overrides this.
mode = %q
# Conventional Commit types that do not need a Why trailer.
skip_types = ["style", "docs", "ci"]
min_why_len = 15
max_value_len = 200
# isekai brief: how many recent commits to summarise, how far back to look for open Next items.
brief_commits = 20
scan_commits = 2000
# Open Next items older than this are flagged as possibly stale.
stale_next_days = 30

[agents]
# Env vars that mark the committer as an agent: "NAME" = set and not "0"/"false";
# "NAME=a|b" = only these values. Add your agent's variable here.
env = [%s]
# Substrings of Co-Authored-By values that mark a commit as agent-authored.
coauthors = ["noreply@anthropic.com", "claude", "codex", "openai", "cursor", "copilot", "aider", "devin", "gemini", "jules", "opencode", "cline", "goose"]

[ui]
# isekai dashboard look: "isekai" (game), "corporate", or a skin in .isekai/ui/skins (docs/skins.md).
look = "isekai"
`, string(mode), quoteList(DefaultAgentEnv))
}

// DefaultAgentEnv: variables agent environments set for the commands they
// run (as of 2026-11). ISEKAI_AGENT is ours: set it to your agent's id.
var DefaultAgentEnv = []string{
	"ISEKAI_AGENT",
	"AI_AGENT",          // emerging cross-vendor convention (Claude Code, Copilot)
	"CLAUDECODE",        // Claude Code (also Amp)
	"CODEX_CI",          // OpenAI Codex CLI
	"CODEX_THREAD_ID",   // OpenAI Codex CLI
	"CODEX_SANDBOX",     // OpenAI Codex CLI (sandboxed runs)
	"CURSOR_AGENT",      // Cursor agent
	"CLINE_ACTIVE",      // Cline
	"OPENCODE",          // OpenCode
	"GEMINI_CLI",        // Gemini CLI
	"QWEN_CODE",         // Qwen Code
	"COPILOT_AGENT",     // GitHub Copilot agent in VS Code
	"AUGMENT_AGENT",     // Augment
	"ANTIGRAVITY_AGENT", // Google Antigravity
	"KIRO_SESSION_ID",   // Kiro
	"TABNINE_CLI",       // Tabnine CLI
	"GOOSE_TERMINAL",    // Goose
	"AGENT=amp|goose|1", // Amp, Goose, OpenCode; generic name, so values are listed
}

func quoteList(xs []string) string {
	q := make([]string, len(xs))
	for i, x := range xs {
		q[i] = fmt.Sprintf("%q", x)
	}
	return strings.Join(q, ", ")
}

// MatchEnv returns the first variable from a.Env that marks an agent.
func (a Agents) MatchEnv(getenv func(string) string) (name, value string, ok bool) {
	for _, spec := range a.Env {
		k, vals, hasVals := strings.Cut(spec, "=")
		v := getenv(k)
		if v == "" || v == "0" || strings.EqualFold(v, "false") {
			continue
		}
		if hasVals {
			match := false
			for _, want := range strings.Split(vals, "|") {
				if strings.EqualFold(v, want) {
					match = true
				}
			}
			if !match {
				continue
			}
		}
		return k, v, true
	}
	return "", "", false
}

// AgentName turns a matched variable into an agent id: the value when it
// names the agent (AI_AGENT=claude-code_2-1_agent, ISEKAI_AGENT=codex), else
// a name derived from the variable (CURSOR_AGENT=1 -> "cursor").
func AgentName(name, value string) string {
	lv := strings.ToLower(value)
	if name != "" && lv != "1" && lv != "true" && !strings.HasSuffix(name, "_ID") && !strings.HasSuffix(name, "_PID") && name != "CODEX_SANDBOX" {
		return value
	}
	n := strings.ToLower(name)
	for _, suf := range []string{"_session_id", "_thread_id", "_agent", "_active", "_terminal", "_cli", "_ci", "_sandbox"} {
		n = strings.TrimSuffix(n, suf)
	}
	return strings.ReplaceAll(n, "_", "-")
}
