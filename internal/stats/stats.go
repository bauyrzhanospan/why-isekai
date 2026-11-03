// Package stats computes the dashboard numbers (`isekai stats`), the data
// contract documented in docs/skins.md (schema version 1). Everything comes
// from git history, the repository's isekai files and a local counter; no
// network, no guesses: a number we cannot know is null.
package stats

import (
	"encoding/json"
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bauyrzhanospan/why-isekai/internal/config"
	"github.com/bauyrzhanospan/why-isekai/internal/gitx"
	"github.com/bauyrzhanospan/why-isekai/internal/journal"
	"github.com/bauyrzhanospan/why-isekai/internal/secrets"
	"github.com/bauyrzhanospan/why-isekai/internal/tracker"
	"github.com/bauyrzhanospan/why-isekai/internal/wiki"
)

// SchemaVersion of the JSON contract.
const SchemaVersion = 1

type Repo struct {
	Name        string `json:"name"`
	Branch      string `json:"branch"`
	GeneratedAt string `json:"generated_at"`
}

type Level struct {
	Pct    int    `json:"pct"` // progress to the next level
	Level  int    `json:"level"`
	XP     int    `json:"xp"`
	NextXP int    `json:"next_xp"`
	PrevXP int    `json:"prev_xp"`
	Rules  string `json:"rules"`
}

type Agent struct {
	ID      string `json:"id"`
	Commits int    `json:"commits"`
	Pct     int    `json:"pct"` // share of all commits
}

type Journal struct {
	Commits          int      `json:"commits"`
	WithWhy          int      `json:"with_why"`
	WhyPct           int      `json:"why_pct"`
	Passing          int      `json:"passing"`
	PassingPct       int      `json:"passing_pct"`
	DeadEnds         int      `json:"dead_ends"`
	OpenCount        int      `json:"open_count"`
	Closed           int      `json:"closed_count"`
	WaitingForHumans int      `json:"waiting_for_humans"`
	OldestOpenDays   int      `json:"oldest_open_days"`
	AvgCloseDays     *float64 `json:"avg_close_days"`
}

type OpenItem struct {
	Text   string `json:"text"`
	Commit string `json:"commit"`
	Date   string `json:"date"`
	Days   int    `json:"days"`
	Stale  bool   `json:"stale"`
}

type DeadEnd struct {
	Commit string `json:"commit"`
	Text   string `json:"text"`
}

type Week struct {
	Week    string `json:"week"`
	Commits int    `json:"commits"`
	Pct     int    `json:"pct"` // relative to the busiest week shown
}

type Growth struct {
	Weeks      []Week `json:"weeks"`
	Files      int    `json:"files"`
	LinesAdded int    `json:"lines_added"`
	ActiveDays int    `json:"active_days"`
}

type Radar struct {
	Values []int      `json:"radar"`
	Names  []string   `json:"names"`
	List   []StatLine `json:"list"`
}

// StatLine is one attribute with its plain meaning.
type StatLine struct {
	Abbr    string `json:"abbr"`
	Name    string `json:"name"`
	Meaning string `json:"meaning"`
	Value   string `json:"value"`
	Pct     int    `json:"pct"`
}

type Gear struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	What     string `json:"what"`
	Equipped bool   `json:"equipped"`
	Rarity   string `json:"rarity"`
	Effect   string `json:"effect"`
	How      string `json:"how"`
}

type Skill struct {
	ID      string `json:"id"`
	Key     string `json:"key"`
	Name    string `json:"name"`
	What    string `json:"what"`
	Learned bool   `json:"learned"`
}

type Achievement struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Unlocked bool   `json:"unlocked"`
	Progress string `json:"progress"`
	Hint     string `json:"hint"`
}

type WikiPage struct {
	Page    string `json:"page"`
	Entries int    `json:"entries"`
	Stale   int    `json:"stale"`
}

type Secrets struct {
	Refs int `json:"refs"`
}

type Tokens struct {
	Estimate     *int   `json:"estimate"`
	BriefsServed int    `json:"briefs_served"`
	Method       string `json:"method"`
}

// Stats is the whole contract.
type Stats struct {
	Version int     `json:"version"`
	Repo    Repo    `json:"repo"`
	Level   Level   `json:"level"`
	Agents  []Agent `json:"agents"`
	Humans  struct {
		Commits int `json:"commits"`
	} `json:"humans"`
	Journal      Journal       `json:"journal"`
	Open         []OpenItem    `json:"open"`
	DeadEnds     []DeadEnd     `json:"dead_ends"`
	Growth       Growth        `json:"growth"`
	Stats        Radar         `json:"stats"`
	Gear         []Gear        `json:"gear"`
	Skills       []Skill       `json:"skills"`
	Achievements []Achievement `json:"achievements"`
	Wiki         []WikiPage    `json:"wiki"`
	Secrets      Secrets       `json:"secrets"`
	TokensSaved  Tokens        `json:"tokens_saved"`
	NextGear     *Gear         `json:"next_gear"` // first integration not set up yet
}

// XP rules, shown to the user next to the level.
const xpRules = "10 per commit, +5 per Why, +15 per dead end recorded, +20 per Next closed"

var levelSteps = []int{0, 100, 250, 500, 800, 1200, 1700, 2300, 3000}

// LevelFor turns XP into a level.
func LevelFor(xp int) Level {
	l := Level{XP: xp, Rules: xpRules}
	for i := len(levelSteps) - 1; i >= 0; i-- {
		if xp >= levelSteps[i] {
			l.Level = i + 1
			l.PrevXP = levelSteps[i]
			if i+1 < len(levelSteps) {
				l.NextXP = levelSteps[i+1]
			} else {
				l.NextXP = levelSteps[i] + 800
			}
			l.Pct = pct(xp-l.PrevXP, l.NextXP-l.PrevXP)
			break
		}
	}
	return l
}

func pct(a, b int) int {
	if b == 0 {
		return 0
	}
	return int(math.Round(float64(a) * 100 / float64(b)))
}

func capPct(v, full int) int {
	if v >= full {
		return 100
	}
	return v * 100 / full
}

// Build computes the stats for the repository at root.
func Build(root string, cfg config.Config, now time.Time) (Stats, error) {
	s := Stats{Version: SchemaVersion, Agents: []Agent{}, Open: []OpenItem{}, DeadEnds: []DeadEnd{}, Wiki: []WikiPage{}}
	s.Repo.Name = filepath.Base(root)
	if b, err := gitx.Run(root, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		s.Repo.Branch = b
	}
	s.Repo.GeneratedAt = now.Format(time.RFC3339)

	scan := cfg.Log.ScanCommits
	commits, err := gitx.Log(root, "-n", strconv.Itoa(scan))
	if err != nil {
		return s, err
	}
	agentCounts := map[string]int{}
	var agentOrder []string
	days := map[string]bool{}
	weekCounts := map[string]int{}
	j := &s.Journal
	for _, c := range commits {
		m := journal.Parse(c.Message)
		if strings.HasPrefix(m.Subject, "Merge ") {
			continue
		}
		j.Commits++
		if len(m.Values(journal.KeyWhy)) > 0 {
			j.WithWhy++
		}
		r := journal.Check(m, cfg, nil)
		ok := true
		for _, i := range r.Issues {
			if i.Severity == journal.Error {
				ok = false
			}
		}
		if ok {
			j.Passing++
		}
		for _, t := range m.Values(journal.KeyTried) {
			j.DeadEnds++
			if len(s.DeadEnds) < 20 {
				s.DeadEnds = append(s.DeadEnds, DeadEnd{Commit: c.Short, Text: t})
			}
		}
		if a := journal.AgentOf(m, cfg); a != "" {
			if agentCounts[a] == 0 {
				agentOrder = append(agentOrder, a)
			}
			agentCounts[a]++
		} else {
			s.Humans.Commits++
		}
		if t, err := time.Parse(time.RFC3339, c.Time); err == nil {
			days[t.Format("2006-01-02")] = true
			y, w := t.ISOWeek()
			weekCounts[strconv.Itoa(y)+"-W"+twoDigits(w)]++
		}
	}
	for _, a := range agentOrder {
		s.Agents = append(s.Agents, Agent{ID: a, Commits: agentCounts[a], Pct: pct(agentCounts[a], j.Commits)})
	}
	sortAgents(s.Agents)
	j.WhyPct = pct(j.WithWhy, j.Commits)
	j.PassingPct = pct(j.Passing, j.Commits)

	b, err := journal.BuildBrief(root, cfg, 1, now)
	if err != nil {
		return s, err
	}
	tracker.ApplyCache(root, &b)
	j.OpenCount = len(b.Open)
	j.Closed = b.Closed
	if b.Closed > 0 {
		avg := math.Round(b.CloseDays/float64(b.Closed)*10) / 10
		j.AvgCloseDays = &avg
	}
	for _, o := range b.Open {
		s.Open = append(s.Open, OpenItem{Text: o.Text, Commit: o.Commit, Date: o.Date, Days: o.AgeDays, Stale: o.Stale})
		if o.AgeDays > j.OldestOpenDays {
			j.OldestOpenDays = o.AgeDays
		}
	}
	j.WaitingForHumans = waitingForHumans(root)

	// Growth: the last 8 ISO weeks, oldest first.
	for i := 7; i >= 0; i-- {
		y, w := now.AddDate(0, 0, -7*i).ISOWeek()
		k := strconv.Itoa(y) + "-W" + twoDigits(w)
		s.Growth.Weeks = append(s.Growth.Weeks, Week{Week: k, Commits: weekCounts[k]})
	}
	maxW := 0
	for _, w := range s.Growth.Weeks {
		if w.Commits > maxW {
			maxW = w.Commits
		}
	}
	for i := range s.Growth.Weeks {
		s.Growth.Weeks[i].Pct = pct(s.Growth.Weeks[i].Commits, maxW)
	}
	s.Growth.ActiveDays = len(days)
	s.Growth.Files, s.Growth.LinesAdded = numstat(root, scan)

	wikiEntries := 0
	if pages, err := wiki.Summary(root, wiki.DefaultStaleDays, now); err == nil {
		for _, p := range pages {
			s.Wiki = append(s.Wiki, WikiPage{Page: p.Page, Entries: p.Entries, Stale: p.Stale})
			wikiEntries += p.Entries
		}
	}
	if refs, err := secrets.AgentVisibleRefs(root, "agent"); err == nil {
		s.Secrets.Refs = len(refs)
	}

	s.Stats = Radar{
		Names:  []string{"WIS", "DIS", "MEM", "QST", "LOR", "SEC"},
		Values: []int{j.WhyPct, j.PassingPct, capPct(j.DeadEnds, 20), capPct(j.OpenCount, 20), capPct(wikiEntries, 10), capPct(s.Secrets.Refs, 5)},
	}
	frac := func(a, b int) string { return strconv.Itoa(a) + "/" + strconv.Itoa(b) }
	s.Stats.List = []StatLine{
		{"WIS", "Wisdom", "commits that explain why", frac(j.WithWhy, j.Commits), s.Stats.Values[0]},
		{"DIS", "Discipline", "commits that pass the check", frac(j.Passing, j.Commits), s.Stats.Values[1]},
		{"MEM", "Memory", "dead ends recorded", strconv.Itoa(j.DeadEnds), s.Stats.Values[2]},
		{"QST", "Quests", "open work items", strconv.Itoa(j.OpenCount), s.Stats.Values[3]},
		{"LOR", "Lore", "wiki entries", strconv.Itoa(wikiEntries), s.Stats.Values[4]},
		{"SEC", "Secrecy", "secrets used by reference", strconv.Itoa(s.Secrets.Refs), s.Stats.Values[5]},
	}
	s.Level = LevelFor(10*j.Commits + 5*j.WithWhy + 15*j.DeadEnds + 20*j.Closed)

	eq := equipment(root, cfg)
	s.Gear = gear(eq)
	for i := range s.Gear {
		if !s.Gear[i].Equipped && s.Gear[i].How != "coming soon" {
			g := s.Gear[i]
			s.NextGear = &g
			break
		}
	}
	s.Skills = skills(eq, j)
	s.Achievements = achievements(j, wikiEntries)
	s.TokensSaved = Tokens{BriefsServed: BriefsServed(root), Method: "docs/benchmark.md"}
	return s, nil
}

func twoDigits(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func sortAgents(a []Agent) {
	for i := 1; i < len(a); i++ {
		for k := i; k > 0 && (a[k].Commits > a[k-1].Commits || (a[k].Commits == a[k-1].Commits && a[k].ID < a[k-1].ID)); k-- {
			a[k], a[k-1] = a[k-1], a[k]
		}
	}
}

func numstat(root string, n int) (files, added int) {
	out, err := gitx.RunRaw(root, nil, "log", "-n", strconv.Itoa(n), "--no-merges", "--numstat", "--format=")
	if err != nil {
		return 0, 0
	}
	seen := map[string]bool{}
	for _, l := range strings.Split(out, "\n") {
		f := strings.SplitN(l, "\t", 3)
		if len(f) != 3 {
			continue
		}
		if a, err := strconv.Atoi(f[0]); err == nil {
			added += a
		}
		seen[f[2]] = true
	}
	return len(seen), added
}

// equipment reports which integrations are set up in this repository.
func equipment(root string, cfg config.Config) map[string]bool {
	read := func(p string) string {
		data, _ := os.ReadFile(filepath.Join(root, p))
		return string(data)
	}
	eq := map[string]bool{}
	if hd, err := gitx.HooksDir(root); err == nil {
		data, _ := os.ReadFile(filepath.Join(hd, "commit-msg"))
		eq["hooks"] = strings.Contains(string(data), "isekai")
	}
	eq["block"] = cfg.Log.Mode == config.ModeBlock
	claude := read(".claude/settings.json")
	eq["brief"] = strings.Contains(claude, "isekai brief")
	eq["guard"] = strings.Contains(claude, "claude-guard")
	_, err := os.Stat(filepath.Join(root, secrets.ConfigFile))
	eq["secrets"] = err == nil
	eq["mcp"] = strings.Contains(read(".mcp.json"), "isekai")
	eq["tracker"] = false // task 9
	eq["notify"] = false  // task 21
	return eq
}

func gear(eq map[string]bool) []Gear {
	g := []Gear{
		{"blade", "Brief Blade", "history at session start", eq["brief"], "legendary", "the agent reads why, dead ends and open work before its first move", "isekai init --claude"},
		{"helm", "Helm of Hooks", "git hooks check every commit", eq["hooks"], "epic", "no commit lands without a real reason", "isekai init"},
		{"mail", "Mail of Block", "block mode", eq["block"], "rare", "agent commits without a Why are turned back with an example", "isekai init --mode block"},
		{"shield", "Shield of No-Verify", "bypass guard", eq["guard"], "rare", "git commit --no-verify is blocked for agents", "isekai init --claude"},
		{"amulet", "Amulet of Secrets", "secrets by reference", eq["secrets"], "epic", "agents use passwords without ever seeing them", "isekai secrets init"},
		{"boots", "Boots of MCP", "MCP server", eq["mcp"], "rare", "any MCP client can call brief and check", "isekai init --mcp"},
		{"ring", "Ring of Issues", "task tracker sync", eq["tracker"], "epic", "open work appears as issues; answer agents there", "isekai issues sync"},
		{"raven", "Raven Messenger", "Telegram, Slack, email", eq["notify"], "rare", "agents reach you where you are; reply to answer", "coming soon"},
	}
	return g
}

func skills(eq map[string]bool, j *Journal) []Skill {
	return []Skill{
		{"recall", "Q", "Recall", "Reads why, dead ends and open work at the start of a session.", true},
		{"never", "W", "Never Again", "Remembers rejected approaches so no agent tries them twice.", j.DeadEnds > 0},
		{"quest", "E", "Quest Log", "Leaves work for the next session (Next) and closes it (Done).", j.OpenCount+j.Closed > 0},
		{"oath", "R", "Oath of Why", "No commit lands without a real reason.", eq["hooks"]},
		{"summon", "", "Summon Tracker", "Works with GitHub Issues and other trackers.", eq["tracker"]},
		{"raven", "", "Raven Message", "Asks you questions in chat.", eq["notify"]},
	}
}

func achievements(j *Journal, wikiEntries int) []Achievement {
	prog := func(v, of int) string { return strconv.Itoa(v) + "/" + strconv.Itoa(of) }
	min := func(a, b int) int {
		if a < b {
			return a
		}
		return b
	}
	return []Achievement{
		{"first-why", "First Why", j.WithWhy > 0, prog(min(j.WithWhy, 1), 1), "Write your first reason"},
		{"cartographer", "Cartographer", j.DeadEnds >= 5, prog(j.DeadEnds, 5), "Record five rejected approaches"},
		{"quest-closer", "Quest Closer", j.Closed >= 10, prog(j.Closed, 10), "Close ten Next items with Done"},
		{"lorekeeper", "Lorekeeper", wikiEntries >= 10, prog(wikiEntries, 10), "Add ten wiki entries"},
		{"clean-hundred", "Hundred Reasons", j.WithWhy >= 100, prog(j.WithWhy, 100), "One hundred commits that explain why"},
	}
}

// waitingForHumans reads the issue-sync cache (task 9); 0 when absent.
func waitingForHumans(root string) int {
	gd, err := gitx.Run(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return 0
	}
	data, err := os.ReadFile(filepath.Join(gd, "isekai", "issues.json"))
	if err != nil {
		return 0
	}
	var c struct {
		Waiting []json.RawMessage `json:"waiting"`
	}
	json.Unmarshal(data, &c)
	return len(c.Waiting)
}

// statsFile is the local, never-committed counter file.
func statsFile(root string) (string, error) {
	gd, err := gitx.Run(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	return filepath.Join(gd, "isekai", "stats.json"), nil
}

type local struct {
	BriefsServed int `json:"briefs_served"`
}

// BriefsServed returns how many times isekai brief ran in this clone.
func BriefsServed(root string) int {
	p, err := statsFile(root)
	if err != nil {
		return 0
	}
	var l local
	data, err := os.ReadFile(p)
	if err == nil {
		json.Unmarshal(data, &l)
	}
	return l.BriefsServed
}

// CountBrief increments the local counter. Failures are ignored: counting
// must never break the brief.
func CountBrief(root string) {
	p, err := statsFile(root)
	if err != nil {
		return
	}
	var l local
	if data, err := os.ReadFile(p); err == nil {
		json.Unmarshal(data, &l)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return
	}
	l.BriefsServed++
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	out, _ := json.Marshal(l)
	os.WriteFile(p, out, 0o644)
}
