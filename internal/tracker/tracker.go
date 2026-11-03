// Package tracker mirrors the journal into an issue tracker (roadmap task 9)
// and reads human answers back. Git trailers stay the source of truth: the
// tracker is a view plus a channel for humans to answer agents. Every
// tracker is an Adapter; GitHub Issues (through the gh CLI) is the first.
package tracker

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bauyrzhanospan/why-isekai/internal/config"
	"github.com/bauyrzhanospan/why-isekai/internal/gitx"
	"github.com/bauyrzhanospan/why-isekai/internal/journal"
)

// Issue is the tracker-neutral view of an issue.
type Issue struct {
	Number   int
	Title    string
	Body     string
	Open     bool
	URL      string
	Comments []Comment
}

// Comment on an issue.
type Comment struct {
	Author  string
	Body    string
	Created string
	// Maintainer is true when the author can write to the repository. On a
	// public repository anyone can comment, and agents read the answer.
	Maintainer bool
}

// Adapter is what a tracker must provide.
type Adapter interface {
	Name() string
	EnsureLabels(labels ...string) error
	List(label string) ([]Issue, error) // open and closed
	Create(title, body string, labels []string) (Issue, error)
	Close(number int, comment string) error
}

// Marker hidden in issue bodies so re-runs never duplicate.
var markerRe = regexp.MustCompile(`<!-- isekai:next id=([0-9a-f]{12}) -->`)

func marker(id string) string { return "<!-- isekai:next id=" + id + " -->" }

// doneComment starts the comment isekai leaves when a Done closes an issue.
const doneComment = "Done in commit "

// closedByDone reports whether isekai closed the issue because of a Done
// trailer (rather than a person closing it as "won't do").
func closedByDone(is Issue) bool {
	n := len(is.Comments)
	return n > 0 && strings.HasPrefix(is.Comments[n-1].Body, doneComment)
}

// Action is one change sync made (or would make with dry run).
type Action struct {
	Kind   string `json:"kind"` // create | close
	Number int    `json:"number,omitempty"`
	Title  string `json:"title"`
}

// Question is a decision issue: an agent asked, a human may have answered.
type Question struct {
	Number   int    `json:"number"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Open     bool   `json:"open"`
	Answer   string `json:"answer,omitempty"`
	AnsBy    string `json:"answered_by,omitempty"`
	Answered string `json:"answered_at,omitempty"`
}

// Cache is written to .git/isekai/issues.json; brief and stats read it, so
// they never touch the network.
type Cache struct {
	SyncedAt       string     `json:"synced_at"`
	Tracker        string     `json:"tracker"`
	ClosedByHumans []string   `json:"closed_by_humans"` // item ids whose issue a human closed
	Waiting        []Question `json:"waiting"`          // open decisions without an answer
	Answered       []Question `json:"answered"`         // decisions with an answer
}

// Options for Sync.
type Options struct {
	DryRun bool
	Now    time.Time
}

// Sync opens issues for open Next items, closes issues whose item was
// closed by Done, and refreshes the cache. It returns what it changed.
func Sync(root string, cfg config.Config, a Adapter, o Options) ([]Action, Cache, error) {
	ic := cfg.Issues
	var acts []Action
	if !o.DryRun {
		if err := a.EnsureLabels(ic.NextLabel, ic.DecisionLabel); err != nil {
			return nil, Cache{}, err
		}
	}
	b, err := journal.BuildBrief(root, cfg, 1, o.Now)
	if err != nil {
		return nil, Cache{}, err
	}
	issues, err := a.List(ic.NextLabel)
	if err != nil {
		return nil, Cache{}, err
	}
	byID := map[string]Issue{}
	for _, is := range issues {
		if m := markerRe.FindStringSubmatch(is.Body); m != nil {
			byID[m[1]] = is
		}
	}
	cache := Cache{SyncedAt: o.Now.Format(time.RFC3339), Tracker: a.Name(), ClosedByHumans: []string{}, Waiting: []Question{}, Answered: []Question{}}

	openIDs := map[string]bool{}
	// Oldest first, so issue numbers follow the order the work was written.
	for k := len(b.Open) - 1; k >= 0; k-- {
		it := b.Open[k]
		id := journal.ItemID(it.Text)
		openIDs[id] = true
		is, ok := byID[id]
		if ok && is.Open {
			continue
		}
		if ok && !closedByDone(is) {
			cache.ClosedByHumans = append(cache.ClosedByHumans, id)
			continue
		}
		// Not tracked yet, or closed earlier by a Done and now written as a
		// Next again: that is new open work, so it gets a new issue.
		text := strings.TrimSpace(it.Text)
		if ic.OpenFor == "marked" {
			if !strings.HasPrefix(strings.ToLower(text), "[issue]") {
				continue
			}
			text = strings.TrimSpace(text[len("[issue]"):])
		}
		title := text
		if r := []rune(title); len(r) > 120 {
			title = string(r[:119]) + "…"
		}
		body := fmt.Sprintf("Open work left by commit `%s` on %s.\n\nClose it with a commit trailer `Done: %s`, or close this issue if it will not be done.\n\n%s",
			it.Commit, it.Date, it.Text, marker(id))
		acts = append(acts, Action{Kind: "create", Title: title})
		created := Issue{Title: title, Body: body, Open: true}
		if !o.DryRun {
			var err error
			created, err = a.Create(title, body, []string{ic.NextLabel})
			if err != nil {
				return acts, cache, err
			}
			created.Open = true
			acts[len(acts)-1].Number = created.Number
		}
		// The same text may be open in several commits: one issue for all.
		byID[id] = created
	}
	closed := map[int]bool{}
	for _, ci := range b.ClosedItems {
		id := journal.ItemID(ci.Text)
		if openIDs[id] {
			continue // reopened by a later Next with the same text
		}
		is, ok := byID[id]
		if !ok || !is.Open || is.Number == 0 || closed[is.Number] {
			continue
		}
		closed[is.Number] = true
		acts = append(acts, Action{Kind: "close", Number: is.Number, Title: is.Title})
		if !o.DryRun {
			if err := a.Close(is.Number, doneComment+ci.Commit+"."); err != nil {
				return acts, cache, err
			}
		}
	}

	decisions, err := a.List(ic.DecisionLabel)
	if err != nil {
		return acts, cache, err
	}
	for _, d := range decisions {
		q := Question{Number: d.Number, Title: d.Title, URL: d.URL, Open: d.Open}
		// The answer is the latest comment by someone who can write to the
		// repository; comments by anyone else are ignored.
		for k := len(d.Comments) - 1; k >= 0; k-- {
			c := d.Comments[k]
			if !c.Maintainer || strings.TrimSpace(c.Body) == "" {
				continue
			}
			q.Answer, q.AnsBy, q.Answered = strings.TrimSpace(c.Body), c.Author, c.Created
			break
		}
		if r := []rune(q.Answer); len(r) > 500 {
			q.Answer = string(r[:499]) + "…"
		}
		switch {
		case q.Answer != "":
			cache.Answered = append(cache.Answered, q)
		case q.Open:
			cache.Waiting = append(cache.Waiting, q)
		}
	}
	if !o.DryRun {
		if err := WriteCache(root, cache); err != nil {
			return acts, cache, err
		}
	}
	return acts, cache, nil
}

// Ask opens a decision issue: an agent asks humans a question.
func Ask(cfg config.Config, a Adapter, question, context string) (Issue, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return Issue{}, errors.New("empty question")
	}
	if err := a.EnsureLabels(cfg.Issues.DecisionLabel); err != nil {
		return Issue{}, err
	}
	title := question
	if r := []rune(title); len(r) > 120 {
		title = string(r[:119]) + "…"
	}
	body := question + "\n\n" + strings.TrimSpace(context) + "\n\nAnswer in a comment; the next agent session sees the latest comment in `isekai brief`. Close the issue when it is settled."
	return a.Create(title, body, []string{cfg.Issues.DecisionLabel})
}

func cachePath(root string) (string, error) {
	gd, err := gitx.Run(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	return filepath.Join(gd, "isekai", "issues.json"), nil
}

// WriteCache stores the cache in the git directory (never committed).
func WriteCache(root string, c Cache) error {
	p, err := cachePath(root)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(data, '\n'), 0o644)
}

// ReadCache returns the cache, or ok=false when sync never ran.
func ReadCache(root string) (Cache, bool) {
	var c Cache
	p, err := cachePath(root)
	if err != nil {
		return c, false
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) || err != nil {
		return c, false
	}
	if json.Unmarshal(data, &c) != nil {
		return c, false
	}
	return c, true
}

// ApplyCache folds the last sync into a brief: items whose issue a human
// closed are no longer open, and agents' questions with answers are shown.
// It reads only the local cache, never the network.
func ApplyCache(root string, b *journal.Brief) {
	c, ok := ReadCache(root)
	if !ok {
		return
	}
	closed := map[string]bool{}
	for _, id := range c.ClosedByHumans {
		closed[id] = true
	}
	kept := b.Open[:0]
	for _, o := range b.Open {
		if !closed[journal.ItemID(o.Text)] {
			kept = append(kept, o)
		}
	}
	b.Open = kept
	for _, q := range c.Waiting {
		b.Waiting = append(b.Waiting, journal.HumanQuestion{Number: q.Number, Title: q.Title, URL: q.URL})
	}
	for _, q := range c.Answered {
		if q.Open {
			b.Answers = append(b.Answers, journal.HumanQuestion{Number: q.Number, Title: q.Title, URL: q.URL, Answer: q.Answer, AnsweredBy: q.AnsBy})
		}
	}
}

// For returns the adapter configured for the repository.
func For(root string, cfg config.Config) (Adapter, error) {
	switch cfg.Issues.Tracker {
	case "", "github":
		return &GitHub{Dir: root, Repo: cfg.Issues.Repo}, nil
	default:
		return nil, fmt.Errorf("tracker %q is not supported yet (supported: github; GitLab, Linear, Jira, ClickUp, Trello and YouTrack are planned)", cfg.Issues.Tracker)
	}
}
