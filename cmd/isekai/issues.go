package main

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/bauyrzhanospan/why-isekai/internal/config"
	"github.com/bauyrzhanospan/why-isekai/internal/gitx"
	"github.com/bauyrzhanospan/why-isekai/internal/tracker"
)

const issuesUsage = `usage:
  isekai issues sync [--dry-run] [--if-enabled]   open issues for Next items, close them on Done, read answers back
  isekai issues ask "<question>"                  ask the humans a question (a "decision" issue)
  isekai issues status                            what the last sync saw (no network)
Talks to the tracker only when you run sync or ask; brief and stats read the local cache.
GitHub needs the gh CLI logged in (gh auth login, or GH_TOKEN in CI).`

func cmdIssues(args []string, out io.Writer) (int, error) {
	if len(args) == 0 {
		return 2, errors.New(issuesUsage)
	}
	root, err := repoRoot()
	if err != nil {
		return 2, err
	}
	cfg, err := config.Load(root)
	if err != nil {
		return 2, err
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "sync":
		fs := newFlags("issues sync")
		dry := fs.Bool("dry-run", false, "show what would change")
		ifEnabled := fs.Bool("if-enabled", false, "do nothing unless [issues] enabled = true")
		if err := fs.Parse(rest); err != nil {
			return 2, errors.New(issuesUsage)
		}
		if *ifEnabled && !cfg.Issues.Enabled {
			fmt.Fprintln(out, "issue sync is off ([issues] enabled = false in .isekai/config.toml)")
			return 0, nil
		}
		a, err := tracker.For(root, cfg)
		if err != nil {
			return 1, err
		}
		acts, cache, err := tracker.Sync(root, cfg, a, tracker.Options{DryRun: *dry, Now: time.Now()})
		verb := map[bool]string{true: "would ", false: ""}[*dry]
		for _, x := range acts {
			if x.Number > 0 {
				fmt.Fprintf(out, "%s%s #%d %s\n", verb, x.Kind, x.Number, x.Title)
			} else {
				fmt.Fprintf(out, "%s%s %s\n", verb, x.Kind, x.Title)
			}
		}
		if err != nil {
			return 1, err
		}
		fmt.Fprintf(out, "%d changes; %d questions waiting for humans, %d answered; %d items closed by humans\n",
			len(acts), len(cache.Waiting), len(cache.Answered), len(cache.ClosedByHumans))
		return 0, nil
	case "ask":
		q := strings.TrimSpace(strings.Join(rest, " "))
		if q == "" {
			return 2, errors.New(`usage: isekai issues ask "<question>"`)
		}
		a, err := tracker.For(root, cfg)
		if err != nil {
			return 1, err
		}
		head, _ := gitx.Run(root, "rev-parse", "--short", "HEAD")
		branch, _ := gitx.Run(root, "rev-parse", "--abbrev-ref", "HEAD")
		who := agentName()
		if who == "" {
			who = "a human"
		}
		ctx := fmt.Sprintf("Asked by %s on branch `%s` at `%s`.", who, branch, head)
		is, err := tracker.Ask(cfg, a, q, ctx)
		if err != nil {
			return 1, err
		}
		fmt.Fprintf(out, "asked: %s\nThe answer will appear in isekai brief after the next isekai issues sync.\n", is.URL)
		return 0, nil
	case "status":
		c, ok := tracker.ReadCache(root)
		if !ok {
			fmt.Fprintln(out, "never synced; run: isekai issues sync")
			return 0, nil
		}
		fmt.Fprintf(out, "last sync %s with %s\n", c.SyncedAt, c.Tracker)
		for _, q := range c.Waiting {
			fmt.Fprintf(out, "waiting  #%d %s %s\n", q.Number, q.Title, q.URL)
		}
		for _, q := range c.Answered {
			fmt.Fprintf(out, "answered #%d %s — %s: %q\n", q.Number, q.Title, q.AnsBy, q.Answer)
		}
		fmt.Fprintf(out, "%d items closed by humans\n", len(c.ClosedByHumans))
		return 0, nil
	}
	return 2, fmt.Errorf("unknown issues command %q\n%s", sub, issuesUsage)
}
