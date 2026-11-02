package journal

import (
	"regexp"
	"strings"
)

var (
	shortNoVer = regexp.MustCompile(`^-[a-zA-Z]*n[a-zA-Z]*$`)
)

// BypassAttempt reports whether a shell command line tries to skip the
// commit hooks: `git commit --no-verify` / `-n`, overriding core.hooksPath,
// or forcing ISEKAI_MODE. It returns a short reason, or "".
// Used by the Claude Code PreToolUse guard; it is a seatbelt against habit,
// not a security boundary.
func BypassAttempt(cmdline string) string {
	for _, seg := range shellSegments(cmdline) {
		toks := shellFields(seg)
		gitAt := -1
		for i, t := range toks {
			if t == "git" || strings.HasSuffix(t, "/git") || t == "git.exe" {
				gitAt = i
				break
			}
		}
		if gitAt < 0 {
			continue
		}
		for _, t := range toks[:gitAt] {
			if strings.HasPrefix(t, "ISEKAI_MODE=") {
				return "overriding ISEKAI_MODE for a git command"
			}
		}
		commitAt := -1
		for i := gitAt + 1; i < len(toks); i++ {
			t := toks[i]
			if t == "-c" && i+1 < len(toks) && strings.HasPrefix(strings.ToLower(toks[i+1]), "core.hookspath") {
				return "overriding core.hooksPath"
			}
			if strings.HasPrefix(strings.ToLower(t), "-ccore.hookspath") {
				return "overriding core.hooksPath"
			}
			if t == "commit" {
				commitAt = i
				break
			}
		}
		if commitAt < 0 {
			continue
		}
		args := toks[commitAt+1:]
		for i := 0; i < len(args); i++ {
			t := args[i]
			if t == "--" {
				break
			}
			if t == "--no-verify" {
				return "git commit --no-verify"
			}
			// -m/-F/-C/-c take a value; skip it so a message like "-n" is not misread.
			if t == "-m" || t == "-F" || t == "-C" || t == "-c" || t == "--author" || t == "--date" {
				i++
				continue
			}
			if !strings.HasPrefix(t, "--") && shortNoVer.MatchString(t) {
				// -n as a bundled short flag; a value-taking flag earlier in the
				// bundle (e.g. -mn) means "n" is the value, not a flag.
				b := strings.TrimPrefix(t, "-")
				pos := strings.IndexByte(b, 'n')
				if !strings.ContainsAny(b[:pos], "mFCc") {
					return "git commit -n (--no-verify)"
				}
			}
		}
	}
	return ""
}

// shellSegments splits a command line at ; && || | and newlines outside quotes.
func shellSegments(s string) []string {
	var segs []string
	var cur strings.Builder
	var q byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case q != 0:
			if c == q {
				q = 0
			} else if c == '\\' && q == '"' && i+1 < len(s) {
				cur.WriteByte(c)
				i++
				c = s[i]
			}
		case c == '\'' || c == '"':
			q = c
		case c == '\\' && i+1 < len(s):
			cur.WriteByte(c)
			i++
			c = s[i]
		case c == ';' || c == '\n' || c == '|' || c == '&':
			segs = append(segs, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	return append(segs, cur.String())
}

// shellFields splits one segment into words, honouring quotes.
func shellFields(s string) []string {
	var out []string
	var cur strings.Builder
	in := false
	var q byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case q != 0:
			if c == q {
				q = 0
				continue
			}
			if c == '\\' && q == '"' && i+1 < len(s) {
				i++
				c = s[i]
			}
			cur.WriteByte(c)
		case c == '\'' || c == '"':
			q, in = c, true
		case c == '\\' && i+1 < len(s):
			i++
			cur.WriteByte(s[i])
			in = true
		case c == ' ' || c == '\t':
			if in {
				out = append(out, cur.String())
				cur.Reset()
				in = false
			}
		default:
			cur.WriteByte(c)
			in = true
		}
	}
	if in {
		out = append(out, cur.String())
	}
	return out
}
