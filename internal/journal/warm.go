package journal

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/bauyrzhanospan/why-isekai/internal/config"
	"github.com/bauyrzhanospan/why-isekai/internal/gitx"
)

// The "warm" part of the brief: what git already knows even when nobody has
// written trailers yet, so the brief is useful from the first day (task 2).
// Everything here is deterministic and inferred items are marked as such.

// CommitLine is a commit shown by hash and subject.
type CommitLine struct {
	Commit  string `json:"commit"`
	Subject string `json:"subject"`
}

// HotSpot is a directory touched by many recent commits.
type HotSpot struct {
	Path    string `json:"path"`
	Commits int    `json:"commits"`
}

// AgentCount is how many commits in the window an agent made.
type AgentCount struct {
	Agent   string `json:"agent"`
	Commits int    `json:"commits"`
}

// Todo is a TODO/FIXME added in a recent commit and still in the code.
type Todo struct {
	Text   string `json:"text"`
	File   string `json:"file"`
	Commit string `json:"commit"`
}

const (
	maxRecent  = 8
	maxHot     = 5
	maxTodos   = 8
	maxTodoLen = 120
)

var (
	todoRe    = regexp.MustCompile(`\b(TODO|FIXME|XXX|HACK)(\([^)]*\))?:`)
	commentRe = regexp.MustCompile(`^\s*(//+|#+|/\*+|\*+|--+|;+|<!--|%+|'|rem\b)\s*`)
	coNameRe  = regexp.MustCompile(`^\s*([^<]+?)\s*(<.*)?$`)
)

// warm fills the inferred sections from the window of recent commits.
func (b *Brief) warm(root string, window []gitx.Commit, cfg config.Config) {
	agents := map[string]int{}
	for _, c := range window {
		m := Parse(c.Message)
		if len(m.Values(KeyWhy)) == 0 && len(b.RecentNoWhy) < maxRecent && !strings.HasPrefix(m.Subject, "Merge ") {
			b.RecentNoWhy = append(b.RecentNoWhy, CommitLine{Commit: c.Short, Subject: m.Subject})
		}
		if strings.HasPrefix(m.Subject, `Revert "`) {
			inner := strings.TrimSuffix(strings.TrimPrefix(m.Subject, `Revert "`), `"`)
			b.DeadEnds = append(b.DeadEnds, DeadEnd{Commit: c.Short, Tried: "reverted: " + inner, Inferred: true})
		}
		if name := agentOf(m, cfg); name != "" {
			agents[name]++
		}
	}
	for a, n := range agents {
		b.Agents = append(b.Agents, AgentCount{Agent: a, Commits: n})
	}
	sort.Slice(b.Agents, func(i, j int) bool {
		if b.Agents[i].Commits != b.Agents[j].Commits {
			return b.Agents[i].Commits > b.Agents[j].Commits
		}
		return b.Agents[i].Agent < b.Agents[j].Agent
	})
	if len(window) == 0 {
		return
	}
	n := strconv.Itoa(len(window))
	b.HotSpots = hotSpots(root, n)
	b.Todos = recentTodos(root, n)
}

// agentOf names the agent that made a commit, from trailers only.
func agentOf(m Message, cfg config.Config) string {
	if v := m.Values(KeyAgent); len(v) > 0 && strings.TrimSpace(v[0]) != "" {
		return strings.TrimSpace(v[0])
	}
	for _, co := range m.Values("Co-Authored-By") {
		lc := strings.ToLower(co)
		for _, s := range cfg.Agents.Coauthors {
			if s != "" && strings.Contains(lc, strings.ToLower(s)) {
				if mm := coNameRe.FindStringSubmatch(co); mm != nil && mm[1] != "" {
					return mm[1]
				}
				return co
			}
		}
	}
	return ""
}

// hotSpots counts, per directory (two levels deep) or top-level file, how many of the last n
// commits touched it.
func hotSpots(root, n string) []HotSpot {
	out, err := gitx.RunRaw(root, nil, "log", "-n", n, "--no-merges", "--format=%x1e", "--name-only")
	if err != nil {
		return nil
	}
	counts := map[string]int{}
	for _, rec := range strings.Split(out, "\x1e") {
		seen := map[string]bool{}
		for _, f := range strings.Split(rec, "\n") {
			f = strings.TrimSpace(f)
			if f == "" {
				continue
			}
			d := dirKey(f)
			if !seen[d] {
				seen[d] = true
				counts[d]++
			}
		}
	}
	var hs []HotSpot
	for p, c := range counts {
		if c > 1 {
			hs = append(hs, HotSpot{Path: p, Commits: c})
		}
	}
	sort.Slice(hs, func(i, j int) bool {
		if hs[i].Commits != hs[j].Commits {
			return hs[i].Commits > hs[j].Commits
		}
		return hs[i].Path < hs[j].Path
	})
	if len(hs) > maxHot {
		hs = hs[:maxHot]
	}
	return hs
}

func dirKey(f string) string {
	parts := strings.Split(filepath.ToSlash(f), "/")
	switch {
	case len(parts) == 1:
		return parts[0] // a top-level file says more than "(root)"
	case len(parts) == 2:
		return parts[0] + "/"
	default:
		return parts[0] + "/" + parts[1] + "/"
	}
}

// recentTodos finds TODO/FIXME lines added in the last n commits that are
// still present in the working tree.
func recentTodos(root, n string) []Todo {
	out, err := gitx.RunRaw(root, nil, "log", "-n", n, "--no-merges", "-p", "--unified=0", "--no-color", "--no-ext-diff", "--format=%x1e%h")
	if err != nil {
		return nil
	}
	var todos []Todo
	seen := map[string]bool{}
	files := map[string]string{} // cache of current file contents
	commit, file := "", ""
	sc := bufio.NewScanner(strings.NewReader(out))
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	lines := 0
	for sc.Scan() {
		lines++
		if lines > 200000 || len(todos) >= maxTodos {
			break
		}
		l := sc.Text()
		switch {
		case strings.HasPrefix(l, "\x1e"):
			commit = strings.TrimPrefix(l, "\x1e")
		case strings.HasPrefix(l, "+++ "):
			file = strings.TrimPrefix(strings.TrimPrefix(l, "+++ "), "b/")
		case strings.HasPrefix(l, "+") && todoRe.MatchString(l):
			raw := strings.TrimSpace(l[1:])
			if file == "" || file == "/dev/null" || seen[raw] {
				continue
			}
			content, ok := files[file]
			if !ok {
				data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
				if err == nil && len(data) < 2<<20 {
					content = string(data)
				}
				files[file] = content
			}
			if !strings.Contains(content, raw) {
				continue // removed since
			}
			seen[raw] = true
			text := strings.TrimSpace(commentRe.ReplaceAllString(raw, ""))
			text = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(text, "*/"), "-->"))
			if r := []rune(text); len(r) > maxTodoLen {
				text = string(r[:maxTodoLen-1]) + "…"
			}
			todos = append(todos, Todo{Text: text, File: file, Commit: commit})
		}
	}
	return todos
}
