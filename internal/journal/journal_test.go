package journal

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/bauyrzhanospan/why-isekai/internal/config"
)

const specExample = `feat(auth): move sessions to JWT

Server sessions replaced with signed tokens.

Why: server sessions break behind the load balancer with >1 node
Tried: sticky sessions; the LB drops affinity on deploy
Next: drop the old sessions table
Next: rotate the JWT signing key
Agent: claude-opus-5-5
Co-Authored-By: Claude <noreply@anthropic.com>
`

func TestParseSpecExample(t *testing.T) {
	m := Parse(specExample)
	if m.Subject != "feat(auth): move sessions to JWT" || m.Type != "feat" || m.Description != "move sessions to JWT" {
		t.Fatalf("subject parse: %+v", m)
	}
	if m.Body != "Server sessions replaced with signed tokens." {
		t.Fatalf("body = %q", m.Body)
	}
	if got := m.Values("why"); len(got) != 1 || got[0] != "server sessions break behind the load balancer with >1 node" {
		t.Fatalf("why = %q", got)
	}
	if got := m.Values("NEXT"); len(got) != 2 {
		t.Fatalf("next = %q", got)
	}
}

// The parser must agree with git's own trailer parser on spec-shaped messages.
func TestParseMatchesGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	cases := []string{
		specExample,
		"fix: x\n\nWhy: because the thing broke on arm64\n",
		"fix: x\n\nbody line\nWhy: not a trailer block because body line\n",
		"fix: x\n\nWhy: long reason\n  continued on the next line\nNext: a\n",
		"fix: subject only\n",
		"Why: subject that looks like a trailer\n",
		"feat: a\n\nbody\n\nSigned-off-by: A <a@b>\nWhy: reason for the change here\n",
		"feat: a\n\nWhy: reason for the change here\nSigned-off-by: A <a@b>\nsome prose line\n",
		"feat: a\n\nWhy: reason for the change here\nNext: x\n(cherry picked from commit 0123456789abcdef0123456789abcdef01234567)\n",
		"feat: a\n\nWhy: reason for the change here\n# a comment line\nNext: x\n",
		"feat: c\n\nWhy: issue number follows in a comment-looking line\n\n#42\n",
		"feat: a\r\n\r\nWhy: crlf message reason here\r\nNext: y\r\n",
		"feat: a\n\nWhy: trailing spaces reason here   \nNext: z  \n\n\n",
	}
	for _, c := range cases {
		cmd := exec.Command("git", "interpret-trailers", "--parse", "--no-divider")
		cmd.Stdin = strings.NewReader(c)
		out, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		var want []string
		for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if l != "" {
				want = append(want, l)
			}
		}
		var got []string
		for _, tr := range Parse(c).Trailers {
			got = append(got, tr.Key+": "+tr.Value)
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("message %q\n got  %q\n want %q", c, got, want)
		}
	}
}

func TestStripComments(t *testing.T) {
	in := "feat: x\n\n# comment\nWhy: reason long enough here\n# ------------------------ >8 ------------------------\ndiff --git a b\n"
	got := StripComments(in, "#")
	if strings.Contains(got, "comment") || strings.Contains(got, "diff") {
		t.Fatalf("got %q", got)
	}
}

func check(msg string, env map[string]string) Result {
	cfg := config.Default()
	return Check(Parse(msg), cfg, func(k string) string { return env[k] })
}

func codes(r Result) string {
	var c []string
	for _, i := range r.Issues {
		c = append(c, i.Code)
	}
	return strings.Join(c, ",")
}

func TestCheckRules(t *testing.T) {
	agent := map[string]string{"ISEKAI_AGENT": "test-agent"}
	tests := []struct {
		name, msg string
		env       map[string]string
		want      string
		isAgent   bool
		skipped   bool
	}{
		{"missing why", "feat: a\n", agent, "why-missing", true, false},
		{"short why", "feat: a\n\nWhy: fix\n", agent, "why-short", true, false},
		{"filler why", "feat: a\n\nWhy: various small fixes and updates\n", agent, "why-filler", true, false},
		{"russian filler", "feat: a\n\nWhy: мелкие правки и исправления\n", agent, "why-filler", true, false},
		{"repeats subject", "feat: add retry to the uploader\n\nWhy: add retry to the uploader\n", agent, "why-repeats-subject", true, false},
		{"why in body", "feat: a\n\nWhy: separated from the other trailers\n\nAgent: x\n", nil, "why-not-trailer", true, false},
		{"good", "feat: a\n\nWhy: uploads fail on flaky mobile networks\n", agent, "", true, false},
		{"good russian", "feat: a\n\nWhy: загрузки падают в мобильных сетях\n", agent, "", true, false},
		{"docs skipped", "docs: readme\n", agent, "", true, true},
		{"style skipped", "style(x): gofmt\n", agent, "", true, true},
		{"ci skipped", "ci: matrix\n", agent, "", true, true},
		{"merge skipped", "Merge branch 'x'\n", agent, "", true, true},
		{"human", "feat: a\n", nil, "why-missing", false, false},
		{"agent trailer", "feat: a\n\nAgent: codex\n", nil, "why-missing", true, false},
		{"coauthor", "feat: a\n\nCo-Authored-By: Claude <noreply@anthropic.com>\n", nil, "why-missing", true, false},
		{"two whys", "feat: a\n\nWhy: first good reason here\nWhy: second good reason here\n", agent, "why-repeated", true, false},
		{"two agents", "feat: a\n\nWhy: first good reason here\nAgent: a\nAgent: b\n", nil, "agent-repeated", true, false},
		{"env zero", "feat: a\n", map[string]string{"CLAUDECODE": "0"}, "why-missing", false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := check(tc.msg, tc.env)
			if got := codes(r); got != tc.want {
				t.Errorf("issues = %q, want %q", got, tc.want)
			}
			if r.Agent != tc.isAgent {
				t.Errorf("agent = %v, want %v", r.Agent, tc.isAgent)
			}
			if (r.Skipped != "") != tc.skipped {
				t.Errorf("skipped = %q", r.Skipped)
			}
		})
	}
}

func TestBlocking(t *testing.T) {
	r := check("feat: a\n", map[string]string{"AI_AGENT": "x"})
	if !r.Blocking(config.ModeBlock) || r.Blocking(config.ModeWarn) {
		t.Fatal("agent: block only in block mode")
	}
	h := check("feat: a\n", nil)
	if h.Blocking(config.ModeBlock) {
		t.Fatal("humans are never blocked")
	}
	rep := Report(r, config.ModeBlock, "feat: a")
	for _, want := range []string{"REJECTED", "why-missing", "Example:", "Why: server sessions"} {
		if !strings.Contains(rep, want) {
			t.Errorf("report lacks %q:\n%s", want, rep)
		}
	}
}

func TestBypassAttempt(t *testing.T) {
	bad := []string{
		`git commit --no-verify -m "x"`,
		`git add . && git commit -nm "x"`,
		`git commit -an -m x`,
		`git -c core.hooksPath=/dev/null commit -m x`,
		`ISEKAI_MODE=warn git commit -m x`,
		`cd a; /usr/bin/git commit -n`,
	}
	ok := []string{
		`git commit -m "fix -n handling"`,
		`git commit -m x`,
		`git log -n 5`,
		`git commit -mn`,
		`echo --no-verify`,
		`git commit -F msg.txt`,
	}
	for _, c := range bad {
		if BypassAttempt(c) == "" {
			t.Errorf("not caught: %s", c)
		}
	}
	for _, c := range ok {
		if r := BypassAttempt(c); r != "" {
			t.Errorf("false positive %q: %s", c, r)
		}
	}
}
