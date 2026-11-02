// Package wiki implements Module 3 of the spec: short dated entries in
// .isekai/wiki/*.md (gotchas, glossary, priorities, people).
package wiki

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Dir is the wiki directory relative to the repository root.
const Dir = ".isekai/wiki"

// DefaultStaleDays: entries not confirmed for longer are possibly stale (§3.3.1).
const DefaultStaleDays = 90

// Standard pages and their headers.
var Standard = map[string]string{
	"gotchas":    "# Gotchas\n\nLocal traps: problem → fix. One entry per line, ending with [date commit author].\n",
	"glossary":   "# Glossary\n\nProject jargon, names and in-jokes. One entry per line, ending with [date commit author].\n",
	"priorities": "# Priorities\n\nWhat matters now, most important first. Set by humans; agents only add entries marked (proposed).\n",
	"people":     "# People\n\nWork facts only: role, area of ownership, who to ask. No judgements about people. Agent entries are drafts until a human confirms them.\n",
}

// Entry is one list item with a trailing [date commit author] label.
type Entry struct {
	Page   string `json:"page"`
	N      int    `json:"n"` // 1-based number among the page's entries
	Line   int    `json:"line"`
	Text   string `json:"text"`
	Date   string `json:"date"`
	Commit string `json:"commit"`
	Author string `json:"author"`
}

var entryRe = regexp.MustCompile(`^\s*[-*]\s+(.*?)\s*\[(\d{4}-\d{2}-\d{2})\s+(\S+)\s+([^\]]+)\]\s*$`)

var nameRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func pagePath(root, page string) (string, error) {
	page = strings.TrimSuffix(page, ".md")
	if !nameRe.MatchString(page) {
		return "", fmt.Errorf("page name %q: use letters, digits, _ and - (e.g. gotchas)", page)
	}
	return filepath.Join(root, Dir, page+".md"), nil
}

// Read returns the entries of one page.
func Read(root, page string) ([]Entry, []string, error) {
	p, err := pagePath(root, page)
	if err != nil {
		return nil, nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, nil, err
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	var es []Entry
	for i, l := range lines {
		if m := entryRe.FindStringSubmatch(l); m != nil {
			es = append(es, Entry{Page: strings.TrimSuffix(page, ".md"), N: len(es) + 1, Line: i, Text: m[1], Date: m[2], Commit: m[3], Author: strings.TrimSpace(m[4])})
		}
	}
	return es, lines, nil
}

// Pages lists page names in the wiki directory.
func Pages(root string) ([]string, error) {
	ents, err := os.ReadDir(filepath.Join(root, Dir))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			out = append(out, strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	sort.Strings(out)
	return out, nil
}

// PageSummary counts entries in a page.
type PageSummary struct {
	Page    string
	Entries int
	Stale   int
}

// IsStale reports whether e was last confirmed more than days ago.
func IsStale(e Entry, days int, now time.Time) bool {
	t, err := time.Parse("2006-01-02", e.Date)
	if err != nil {
		return false
	}
	return now.Sub(t) > time.Duration(days)*24*time.Hour
}

// Summary returns per-page counts for the brief.
func Summary(root string, days int, now time.Time) ([]PageSummary, error) {
	pages, err := Pages(root)
	if err != nil {
		return nil, err
	}
	var out []PageSummary
	for _, p := range pages {
		es, _, err := Read(root, p)
		if err != nil {
			continue
		}
		s := PageSummary{Page: p, Entries: len(es)}
		for _, e := range es {
			if IsStale(e, days, now) {
				s.Stale++
			}
		}
		out = append(out, s)
	}
	return out, nil
}

// Top returns the first n entry texts of a page (for priorities in the brief).
func Top(root, page string, n int) ([]string, error) {
	es, _, err := Read(root, page)
	if err != nil {
		return nil, err
	}
	var out []string
	for i, e := range es {
		if i == n {
			break
		}
		out = append(out, e.Text)
	}
	return out, nil
}

// Stale lists entries across all pages not confirmed for more than days.
func Stale(root string, days int, now time.Time) ([]Entry, error) {
	pages, err := Pages(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []Entry
	for _, p := range pages {
		es, _, err := Read(root, p)
		if err != nil {
			continue
		}
		for _, e := range es {
			if IsStale(e, days, now) {
				out = append(out, e)
			}
		}
	}
	return out, nil
}

// secretish catches obvious credentials pasted into an entry (§3.6.1).
var secretish = regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----|\bAKIA[0-9A-Z]{16}\b|\bgh[pousr]_[A-Za-z0-9]{30,}|\bsk-[A-Za-z0-9_-]{20,}|\bxox[abpr]-[A-Za-z0-9-]{10,}|AGE-SECRET-KEY-1[0-9A-Z]{50,}|\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.`)

// AddOptions for Add.
type AddOptions struct {
	Author string // "human" or agent id
	Agent  bool
	Commit string // short hash or "-"
	Now    time.Time
}

// Add appends an entry and returns it plus any warnings.
func Add(root, page, text string, o AddOptions) (Entry, []string, error) {
	var warns []string
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return Entry{}, nil, errors.New("empty entry text")
	}
	if strings.ContainsAny(text, "[]") && entryRe.MatchString("- "+text) {
		return Entry{}, nil, errors.New("entry text already ends with a [date commit author] label; give the text only")
	}
	if secretish.MatchString(text) {
		return Entry{}, nil, errors.New("entry looks like it contains a secret value; reference it as secret://<scope>/<name> instead (spec §3.6)")
	}
	page = strings.TrimSuffix(page, ".md")
	p, err := pagePath(root, page)
	if err != nil {
		return Entry{}, nil, err
	}
	if o.Agent {
		switch page {
		case "priorities":
			if lt := strings.ToLower(text); !strings.Contains(lt, "(proposed)") && !strings.Contains(lt, "(предложено)") {
				text += " (proposed)"
			}
			warns = append(warns, "priorities are set by humans; your entry is marked (proposed). Do not reorder or delete others' entries (spec §3.4).")
		case "people":
			if lt := strings.ToLower(text); !strings.Contains(lt, "(draft)") && !strings.Contains(lt, "(черновик)") {
				text += " (draft)"
			}
			warns = append(warns, "people.md holds work facts only (role, ownership, who to ask) — no judgements about people. Your entry is a draft until a human confirms it (spec §3.5).")
		}
	}
	author := strings.TrimSpace(o.Author)
	if author == "" {
		author = "human"
	}
	commit := o.Commit
	if commit == "" {
		commit = "-"
	}
	if o.Now.IsZero() {
		o.Now = time.Now()
	}
	date := o.Now.Format("2006-01-02")
	line := fmt.Sprintf("- %s [%s %s %s]\n", text, date, commit, author)

	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return Entry{}, nil, err
		}
		hdr, ok := Standard[page]
		if !ok {
			hdr = "# " + page + "\n"
		}
		data = []byte(hdr + "\n")
	} else if err != nil {
		return Entry{}, nil, err
	}
	s := string(data)
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	if err := os.WriteFile(p, []byte(s+line), 0o644); err != nil {
		return Entry{}, nil, err
	}
	es, _, _ := Read(root, page)
	e := Entry{Page: page, Text: text, Date: date, Commit: commit, Author: author, N: len(es)}
	return e, warns, nil
}

// Confirm sets the date of entry n on page to now (§3.2.3).
func Confirm(root, page string, n int, now time.Time) (Entry, error) {
	es, lines, err := Read(root, page)
	if err != nil {
		return Entry{}, err
	}
	if n < 1 || n > len(es) {
		return Entry{}, fmt.Errorf("%s has %d entries; no entry %d (see: isekai wiki list %s)", page, len(es), n, page)
	}
	e := es[n-1]
	old := fmt.Sprintf("[%s %s ", e.Date, e.Commit)
	e.Date = now.Format("2006-01-02")
	l := lines[e.Line]
	i := strings.LastIndex(l, old)
	lines[e.Line] = l[:i] + fmt.Sprintf("[%s %s ", e.Date, e.Commit) + l[i+len(old):]
	p, _ := pagePath(root, page)
	return e, os.WriteFile(p, []byte(strings.Join(lines, "\n")), 0o644)
}
