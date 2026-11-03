package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/bauyrzhanospan/why-isekai/internal/config"
	"github.com/bauyrzhanospan/why-isekai/internal/gitx"
	"github.com/bauyrzhanospan/why-isekai/internal/install"
	"github.com/bauyrzhanospan/why-isekai/internal/journal"
	"github.com/bauyrzhanospan/why-isekai/internal/secrets"
	"github.com/bauyrzhanospan/why-isekai/internal/stats"
	"github.com/bauyrzhanospan/why-isekai/internal/wiki"
)

func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root, err := gitx.Root(wd)
	if err != nil {
		return "", fmt.Errorf("not inside a git work tree (%v)", err)
	}
	return root, nil
}

func newFlags(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

// parseInterspersed lets flags appear after positional arguments.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return pos, nil
		}
		if args[0] == "--" {
			return append(pos, args[1:]...), nil
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}

func cmdInit(args []string, out io.Writer) error {
	fs := newFlags("init")
	mode := fs.String("mode", "warn", "warn or block")
	claude := fs.Bool("claude", false, "also install Claude Code hooks")
	mcp := fs.Bool("mcp", false, "also register the isekai MCP server in .mcp.json")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("init: %v (usage: isekai init [--mode warn|block] [--claude] [--mcp])", err)
	}
	m := config.Mode(*mode)
	if m != config.ModeWarn && m != config.ModeBlock {
		return fmt.Errorf("init: --mode must be warn or block")
	}
	wd, _ := os.Getwd()
	steps, err := install.Init(wd, install.Options{Mode: m, Claude: *claude, MCP: *mcp})
	for _, s := range steps {
		fmt.Fprintf(out, "%-10s %s\n", s.Action, s.Path)
	}
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "isekai is set up. Commit messages now end with trailers (Why/Tried/Next/Done/Agent); run `isekai brief` to read them back.")
	return nil
}

func cmdBrief(args []string, out io.Writer) error {
	fs := newFlags("brief")
	n := fs.Int("n", 0, "number of commits")
	format := fs.String("format", "text", "text, md or json")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("brief: %v (usage: isekai brief [-n N] [--format text|md|json])", err)
	}
	root, err := repoRoot()
	if err != nil {
		return err
	}
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	b, err := journal.BuildBrief(root, cfg, *n, time.Now())
	if err != nil {
		return err
	}
	if pages, err := wiki.Summary(root, wiki.DefaultStaleDays, time.Now()); err == nil {
		for _, p := range pages {
			b.Wiki = append(b.Wiki, journal.WikiPage{Page: p.Page, Entries: p.Entries, Stale: p.Stale})
		}
	}
	if top, err := wiki.Top(root, "priorities", 5); err == nil {
		b.Priorities = top
	}
	if refs, err := secrets.AgentVisibleRefs(root, agentName()); err == nil {
		b.Secrets = refs
	}
	stats.CountBrief(root)
	return b.Render(out, *format)
}

func cmdStats(args []string, out io.Writer) error {
	fs := newFlags("stats")
	format := fs.String("format", "text", "text or json")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("stats: %v (usage: isekai stats [--format text|json])", err)
	}
	root, err := repoRoot()
	if err != nil {
		return err
	}
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	s, err := stats.Build(root, cfg, time.Now())
	if err != nil {
		return err
	}
	if *format == "json" {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(s)
	}
	j := s.Journal
	p := func(f string, a ...any) { fmt.Fprintf(out, f+"\n", a...) }
	p("isekai stats — %s (%s)", s.Repo.Name, s.Repo.Branch)
	p("level        %d (%d/%d XP; %s)", s.Level.Level, s.Level.XP, s.Level.NextXP, s.Level.Rules)
	p("commits      %d; with Why %d (%d%%); passing the check %d%%", j.Commits, j.WithWhy, j.WhyPct, j.PassingPct)
	p("dead ends    %d recorded", j.DeadEnds)
	avg := "n/a"
	if j.AvgCloseDays != nil {
		avg = fmt.Sprintf("%.1f days", *j.AvgCloseDays)
	}
	p("open work    %d open, %d closed (avg %s), oldest %d days, %d waiting for humans", j.OpenCount, j.Closed, avg, j.OldestOpenDays, j.WaitingForHumans)
	var ag []string
	for _, a := range s.Agents {
		ag = append(ag, fmt.Sprintf("%s %d", a.ID, a.Commits))
	}
	if len(ag) == 0 {
		ag = []string{"none"}
	}
	p("agents       %s; humans %d commits", strings.Join(ag, ", "), s.Humans.Commits)
	p("growth       %d active days, %d files, +%d lines", s.Growth.ActiveDays, s.Growth.Files, s.Growth.LinesAdded)
	var eq []string
	for _, g := range s.Gear {
		if g.Equipped {
			eq = append(eq, g.Name)
		}
	}
	p("equipped     %s", strings.Join(eq, ", "))
	p("briefs       %d served in this clone; tokens saved: not measured yet", s.TokensSaved.BriefsServed)
	return nil
}

type checkedCommit struct {
	Commit  string          `json:"commit"`
	Subject string          `json:"subject"`
	Agent   bool            `json:"agent"`
	Skipped string          `json:"skipped,omitempty"`
	Issues  []journal.Issue `json:"issues"`
}

func cmdCheck(args []string, out io.Writer) (int, error) {
	fs := newFlags("check")
	strict := fs.Bool("strict", false, "treat problems in human commits as errors too")
	format := fs.String("format", "text", "text or json")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return 2, fmt.Errorf("check: %v (usage: isekai check [<range>] [--strict] [--format text|json|md])", err)
	}
	root, err := repoRoot()
	if err != nil {
		return 2, err
	}
	cfg, err := config.Load(root)
	if err != nil {
		return 2, err
	}
	logArgs := pos
	if len(logArgs) == 0 {
		if _, err := gitx.Run(root, "rev-parse", "--verify", "-q", "@{upstream}"); err == nil {
			logArgs = []string{"@{upstream}..HEAD"}
		} else {
			logArgs = []string{"-n", "1", "HEAD"}
		}
	}
	commits, err := gitx.Log(root, logArgs...)
	if err != nil {
		return 2, err
	}
	var results []checkedCommit
	failed := 0
	for _, c := range commits {
		m := journal.Parse(c.Message)
		r := journal.Check(m, cfg, nil) // CI: the environment says nothing about the author
		cc := checkedCommit{Commit: c.Short, Subject: m.Subject, Agent: r.Agent, Skipped: r.Skipped, Issues: r.Issues}
		if cc.Issues == nil {
			cc.Issues = []journal.Issue{}
		}
		bad := r.Blocking(cfg.Log.Mode)
		if *strict {
			for _, i := range r.Issues {
				if i.Severity == journal.Error {
					bad = true
				}
			}
		}
		if bad {
			failed++
		}
		results = append(results, cc)
	}
	if *format == "json" {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if results == nil {
			results = []checkedCommit{}
		}
		if err := enc.Encode(map[string]any{"mode": cfg.Log.Mode, "checked": len(results), "failed": failed, "commits": results}); err != nil {
			return 2, err
		}
	} else if *format == "md" {
		writeCheckMarkdown(out, results, failed, string(cfg.Log.Mode), *strict)
	} else {
		problems := 0
		for _, r := range results {
			if len(r.Issues) == 0 {
				continue
			}
			problems++
			who := "human"
			if r.Agent {
				who = "agent"
			}
			fmt.Fprintf(out, "%s %s (%s)\n", r.Commit, r.Subject, who)
			for _, i := range r.Issues {
				sev := "warn"
				if i.Severity == journal.Error {
					sev = "error"
				}
				fmt.Fprintf(out, "  %s[%s]: %s\n", sev, i.Code, i.Text)
			}
		}
		fmt.Fprintf(out, "isekai check: %d commits, %d with problems, %d failing (mode=%s%s)\n",
			len(results), problems, failed, cfg.Log.Mode, map[bool]string{true: ", strict", false: ""}[*strict])
		if problems > 0 {
			fmt.Fprintf(out, "Each commit should end with `Why: <reason>`. Example:\n%s\n", journal.Example)
		}
	}
	if failed > 0 {
		return 1, nil
	}
	return 0, nil
}

func cmdHook(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	if len(args) == 0 {
		return 2, errors.New("hook: missing hook name (commit-msg, prepare-commit-msg, claude-guard)")
	}
	switch args[0] {
	case "commit-msg":
		if len(args) < 2 {
			return 2, errors.New("hook commit-msg: missing message file")
		}
		return hookCommitMsg(args[1], stderr)
	case "prepare-commit-msg":
		if len(args) < 2 {
			return 2, errors.New("hook prepare-commit-msg: missing message file")
		}
		src := ""
		if len(args) > 2 {
			src = args[2]
		} else {
			// pre-commit passes only the file; the source comes in its environment.
			src = os.Getenv("PRE_COMMIT_COMMIT_MSG_SOURCE")
		}
		if err := hookPrepare(args[1], src); err != nil {
			// Never block a commit because the template could not be written.
			fmt.Fprintln(stderr, "isekai: prepare-commit-msg:", err)
		}
		return 0, nil
	case "claude-guard":
		return hookClaudeGuard(stdin, stderr)
	}
	return 2, fmt.Errorf("hook: unknown hook %q", args[0])
}

func hookCommitMsg(file string, stderr io.Writer) (int, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return 1, err
	}
	root, err := repoRoot()
	if err != nil {
		return 1, err
	}
	cfg, err := config.Load(root)
	if err != nil {
		// A broken config must not lock everyone out of committing.
		fmt.Fprintln(stderr, "isekai: warning:", err, "— using defaults")
		cfg = config.Default()
	}
	cc := gitx.CommentChar(root)
	if strings.TrimSpace(journal.StripComments(string(data), cc)) == "" {
		return 0, nil // git aborts empty messages itself
	}
	// Comment lines are ignored by the parser exactly as git ignores them, so
	// the hook and `git log` agree even for -m messages, where git keeps them.
	m := journal.ParseComment(journal.CutScissors(string(data), cc), cc)
	r := journal.Check(m, cfg, journal.OSEnv)
	if rep := journal.Report(r, cfg.Log.Mode, m.Subject); rep != "" {
		fmt.Fprint(stderr, rep)
	}
	if r.Blocking(cfg.Log.Mode) {
		return 1, nil
	}
	return 0, nil
}

func hookPrepare(file, source string) error {
	// Only when an editor will open: with -m/-F (source "message") git does
	// not strip comment lines, so a template would end up in the commit.
	if source != "" && source != "template" {
		return nil
	}
	root, err := repoRoot()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	cc := gitx.CommentChar(root)
	cfg, _ := config.Load(root)
	tpl := journal.Template(root, cfg, cc)
	s := string(data)
	// Insert before git's own comment block so the trailers sit right under
	// the message the author is about to type.
	idx, off := -1, 0
	for _, line := range strings.SplitAfter(s, "\n") {
		if strings.HasPrefix(line, cc) {
			idx = off
			break
		}
		off += len(line)
	}
	if idx < 0 {
		s = strings.TrimRight(s, "\n") + "\n" + tpl
	} else {
		s = s[:idx] + tpl + s[idx:]
	}
	return os.WriteFile(file, []byte(s), 0o644)
}

func hookClaudeGuard(stdin io.Reader, stderr io.Writer) (int, error) {
	var in struct {
		ToolName  string `json:"tool_name"`
		ToolInput struct {
			Command string `json:"command"`
		} `json:"tool_input"`
	}
	data, err := io.ReadAll(stdin)
	if err != nil || json.Unmarshal(data, &in) != nil {
		return 0, nil // not our business; never break the agent on bad input
	}
	if reason := journal.BypassAttempt(in.ToolInput.Command); reason != "" {
		fmt.Fprintf(stderr, "isekai: blocked %s. This project records why each change was made; "+
			"commit without skipping hooks and end the message with a `Why: <reason>` trailer. Example:\n%s\n", reason, journal.Example)
		return 2, nil // exit 2 = block the tool call and show stderr to the agent
	}
	return 0, nil
}

// agentName is how the current agent identifies itself, or "" for a human.
func agentName() string {
	cfg := config.Default()
	if root, err := repoRoot(); err == nil {
		if c, err := config.Load(root); err == nil {
			cfg = c
		}
	}
	if k, v, ok := cfg.Agents.MatchEnv(os.Getenv); ok {
		return config.AgentName(k, v)
	}
	return ""
}

// writeCheckMarkdown renders check results for a CI job summary.
func writeCheckMarkdown(out io.Writer, results []checkedCommit, failed int, mode string, strict bool) {
	problems := 0
	for _, r := range results {
		if len(r.Issues) > 0 {
			problems++
		}
	}
	icon := "✅"
	if failed > 0 {
		icon = "❌"
	} else if problems > 0 {
		icon = "⚠️"
	}
	fmt.Fprintf(out, "### %s isekai check\n\n", icon)
	fmt.Fprintf(out, "%d commits checked, %d with problems, %d failing (mode `%s`%s).\n\n", len(results), problems, failed, mode, map[bool]string{true: ", strict", false: ""}[strict])
	if problems == 0 {
		fmt.Fprintln(out, "Every commit explains why.")
		return
	}
	fmt.Fprintln(out, "| Commit | Subject | Author | Problem |")
	fmt.Fprintln(out, "|---|---|---|---|")
	esc := func(s string) string {
		s = strings.ReplaceAll(s, "|", "\\|")
		s = strings.ReplaceAll(s, "<", "&lt;")
		return strings.ReplaceAll(s, "\n", " ")
	}
	for _, r := range results {
		if len(r.Issues) == 0 {
			continue
		}
		who := "human"
		if r.Agent {
			who = "agent"
		}
		var texts []string
		for _, i := range r.Issues {
			texts = append(texts, "`"+i.Code+"` "+esc(i.Text))
		}
		fmt.Fprintf(out, "| `%s` | %s | %s | %s |\n", r.Commit, esc(r.Subject), who, strings.Join(texts, "<br>"))
	}
	fmt.Fprintf(out, "\nEach commit should end with a `Why:` trailer. Example:\n\n```\n%s\n```\n", journal.Example)
}
