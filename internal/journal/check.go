package journal

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode"

	"github.com/bauyrzhanospan/why-isekai/internal/config"
)

// Severity of an Issue.
type Severity int

const (
	Warning Severity = iota
	Error
)

// Issue is one spec violation found in a commit message.
type Issue struct {
	Code     string // stable machine-readable id, e.g. "why-missing"
	Severity Severity
	Text     string
}

// Result of checking one message.
type Result struct {
	Agent       bool
	AgentReason string
	Skipped     string // non-empty: why the Why rules were not applied
	Issues      []Issue
}

// Blocking reports whether the commit must be rejected under mode.
// Only agent commits are ever blocked (spec §1.5.1-1.5.2).
func (r Result) Blocking(mode config.Mode) bool {
	if mode != config.ModeBlock || !r.Agent {
		return false
	}
	for _, i := range r.Issues {
		if i.Severity == Error {
			return true
		}
	}
	return false
}

// DetectAgent decides whether a commit is agent-authored (spec §1.5.1).
// env may be nil (e.g. in CI, where the environment says nothing about the
// commit author).
func DetectAgent(m Message, cfg config.Config, env func(string) string) (bool, string) {
	if v := m.Values(KeyAgent); len(v) > 0 {
		return true, "Agent trailer: " + v[0]
	}
	for _, co := range m.Values("Co-Authored-By") {
		lc := strings.ToLower(co)
		for _, s := range cfg.Agents.Coauthors {
			if s != "" && strings.Contains(lc, strings.ToLower(s)) {
				return true, "Co-Authored-By: " + co
			}
		}
	}
	if env != nil {
		if k, v, ok := cfg.Agents.MatchEnv(env); ok {
			return true, "environment variable " + k + "=" + v
		}
	}
	return false, ""
}

// OSEnv is os.Getenv, for DetectAgent.
var OSEnv = os.Getenv

var whyInBodyRe = regexp.MustCompile(`(?im)^\s*why\s*:`)

// fillerWords: a Why made only of these says nothing (spec §1.3.4).
var fillerWords = map[string]bool{
	"fix": true, "fixes": true, "fixed": true, "fixing": true, "bug": true, "bugfix": true,
	"update": true, "updates": true, "updated": true, "change": true, "changes": true,
	"changed": true, "wip": true, "misc": true, "refactor": true, "refactoring": true,
	"cleanup": true, "clean": true, "up": true, "tweak": true, "tweaks": true, "stuff": true,
	"improve": true, "improvement": true, "improvements": true, "minor": true, "small": true,
	"some": true, "various": true, "code": true, "it": true, "the": true, "a": true,
	"needed": true, "required": true, "necessary": true, "todo": true, "tbd": true,
	"n/a": true, "na": true, "none": true, "see": true, "above": true, "commit": true,
	"исправление": true, "исправления": true, "исправил": true, "исправлено": true,
	"правки": true, "правка": true, "фикс": true, "фиксы": true, "обновление": true,
	"обновил": true, "изменения": true, "изменение": true, "рефакторинг": true,
	"мелкие": true, "мелочи": true, "разное": true, "доработка": true, "доработки": true,
	"улучшения": true, "надо": true, "нужно": true, "код": true,
	"and": true, "or": true, "of": true, "to": true, "for": true, "in": true, "on": true, "with": true, "more": true, "few": true,
	"и": true, "в": true, "на": true, "по": true, "для": true, "с": true, "ещё": true, "еще": true, "немного": true,
}

func words(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '/'
	})
}

func normalized(s string) string { return strings.Join(words(s), " ") }

// Check validates m against spec §1.1-1.5.
func Check(m Message, cfg config.Config, env func(string) string) Result {
	var r Result
	r.Agent, r.AgentReason = DetectAgent(m, cfg, env)
	add := func(code string, sev Severity, f string, a ...any) {
		r.Issues = append(r.Issues, Issue{Code: code, Severity: sev, Text: fmt.Sprintf(f, a...)})
	}

	// Format rules that apply to every commit.
	maxLen := cfg.Log.MaxValueLen
	for _, t := range m.Trailers {
		if !isIsekaiKey(t.Key) {
			continue
		}
		if maxLen > 0 && len([]rune(t.Value)) > maxLen {
			add("value-too-long", Warning, "%s is %d chars; keep each value under %d (repeat the key for more items)", canonical(t.Key), len([]rune(t.Value)), maxLen)
		}
		if strings.TrimSpace(t.Value) == "" {
			add("value-empty", Error, "%s has an empty value; fill it in or delete the line", canonical(t.Key))
		}
	}
	if n := len(m.Values(KeyAgent)); n > 1 {
		add("agent-repeated", Error, "Agent appears %d times; it is not repeatable", n)
	}

	if s := skipReason(m, cfg); s != "" {
		r.Skipped = s
		return r
	}

	whys := m.Values(KeyWhy)
	switch {
	case len(whys) == 0:
		if whyInBodyRe.MatchString(m.Body) {
			add("why-not-trailer", Error, "`Why:` is in the body, not in the trailer block; put it in the last paragraph together with the other trailers, with no blank line between them")
			return r
		}
		add("why-missing", Error, "missing `Why:` trailer")
		return r
	case len(whys) > 1:
		add("why-repeated", Error, "Why appears %d times; give one reason (use Tried/Next for the rest)", len(whys))
	}
	why := strings.TrimSpace(whys[0])
	if why == "" {
		return r // already reported as value-empty
	}
	min := cfg.Log.MinWhyLen
	if n := len([]rune(why)); min > 0 && n < min {
		add("why-short", Error, "`Why: %s` is %d chars; at least %d needed to say what problem this solves", why, n, min)
		return r
	}
	allFiller := true
	for _, w := range words(why) {
		if !fillerWords[w] {
			allFiller = false
			break
		}
	}
	if allFiller {
		add("why-filler", Error, "`Why: %s` is a placeholder; name the problem, constraint or goal", why)
		return r
	}
	nw := normalized(why)
	if nw == normalized(m.Subject) || nw == normalized(m.Description) {
		add("why-repeats-subject", Error, "`Why:` repeats the subject; the subject says what changed, Why says why")
	}
	return r
}

func skipReason(m Message, cfg config.Config) string {
	s := m.Subject
	switch {
	case strings.HasPrefix(s, "Merge "):
		return "merge commit"
	case strings.HasPrefix(s, "fixup! "), strings.HasPrefix(s, "squash! "), strings.HasPrefix(s, "amend! "):
		return "fixup/squash commit"
	case strings.HasPrefix(s, `Revert "`):
		return "revert commit"
	}
	for _, t := range cfg.Log.SkipTypes {
		if m.Type != "" && strings.EqualFold(m.Type, t) {
			return "type " + m.Type + ": does not need Why"
		}
	}
	return ""
}

func isIsekaiKey(k string) bool {
	for _, c := range []string{KeyWhy, KeyTried, KeyNext, KeyDone, KeyAgent} {
		if strings.EqualFold(k, c) {
			return true
		}
	}
	return false
}

func canonical(k string) string {
	for _, c := range []string{KeyWhy, KeyTried, KeyNext, KeyDone, KeyAgent} {
		if strings.EqualFold(k, c) {
			return c
		}
	}
	return k
}

// Example is shown in every rejection (spec §1.5.3: error = documentation).
const Example = `  feat(auth): move sessions to JWT

  Server sessions replaced with signed tokens.

  Why: server sessions break behind the load balancer with >1 node
  Tried: sticky sessions; the LB drops affinity on every deploy
  Next: drop the old sessions table
  Agent: claude-opus-5-5`

// Report renders r for a human or agent reading hook output.
func Report(r Result, mode config.Mode, subject string) string {
	if len(r.Issues) == 0 {
		return ""
	}
	var b strings.Builder
	blocking := r.Blocking(mode)
	who := "human commit"
	if r.Agent {
		who = "agent commit (" + r.AgentReason + ")"
	}
	if blocking {
		fmt.Fprintf(&b, "isekai: commit REJECTED — %s, mode=block\n", who)
	} else {
		fmt.Fprintf(&b, "isekai: warning — %s, mode=%s; commit allowed\n", who, mode)
	}
	if subject != "" {
		fmt.Fprintf(&b, "  subject: %s\n", subject)
	}
	needWhyHelp := false
	for _, i := range r.Issues {
		mark := "warn"
		if i.Severity == Error {
			mark = "error"
		}
		fmt.Fprintf(&b, "  %s[%s]: %s\n", mark, i.Code, i.Text)
		if strings.HasPrefix(i.Code, "why-") {
			needWhyHelp = true
		}
	}
	if needWhyHelp && (r.Agent || blocking) {
		b.WriteString(`
Why records the reason for the change: the problem, constraint or goal —
not a summary of the diff. Put it in the last paragraph of the message, one
line, after a blank line: "Why: <reason>". Optional keys: Tried (rejected
approach; why), Next (work left), Done (exact text of an earlier Next),
Agent (your model id).

Example:
` + Example + `

Commits typed style:, docs: or ci: do not need Why.
`)
	}
	if needWhyHelp && !r.Agent && !blocking {
		b.WriteString("  hint: end the message with \"Why: <the problem, constraint or goal>\" (see: isekai --help)\n")
	}
	if blocking {
		b.WriteString("Your message is saved; add the trailers and retry: git commit -e -F \"$(git rev-parse --git-dir)/COMMIT_EDITMSG\"\n")
	}
	return b.String()
}
