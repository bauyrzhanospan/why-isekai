package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/bauyrzhanospan/why-isekai/internal/config"
	"github.com/bauyrzhanospan/why-isekai/internal/dashboard"
	"github.com/bauyrzhanospan/why-isekai/internal/stats"
)

const uiUsage = `usage:
  isekai ui list                         looks you can use (built in and .isekai/ui/skins)
  isekai ui new <name> [--from isekai|corporate]   start a skin from a built-in look
  isekai ui check <name>                 validate a skin (docs/skins.md rules)
  isekai ui use <name>                   make it the dashboard look ([ui] look in .isekai/config.toml)
How to design a skin (for agents): docs/skins.md`

func cmdUI(args []string, out io.Writer) (int, error) {
	if len(args) == 0 {
		return 2, errors.New(uiUsage)
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
	case "list", "ls":
		looks, errs := dashboard.Looks(root)
		for _, l := range looks {
			mark := " "
			if l.ID == cfg.UI.Look {
				mark = "*"
			}
			kind := "skin"
			if l.BuiltIn {
				kind = "built in"
			}
			fmt.Fprintf(out, "%s %-14s %-9s %s\n", mark, l.ID, kind, l.Description)
		}
		for _, e := range errs {
			fmt.Fprintln(out, "! ", e)
		}
		return 0, nil
	case "new":
		fs := newFlags("ui new")
		from := fs.String("from", "isekai", "isekai or corporate")
		pos, err := parseInterspersed(fs, rest)
		if err != nil || len(pos) != 1 {
			return 2, errors.New(uiUsage)
		}
		dir, err := dashboard.NewSkin(root, pos[0], *from)
		if err != nil {
			return 1, err
		}
		fmt.Fprintf(out, "created %s from the %s look\nEdit index.html and skin.css (docs/skins.md), then: isekai ui check %s && isekai ui use %s\n", rel(root, dir), *from, pos[0], pos[0])
		return 0, nil
	case "check":
		if len(rest) != 1 {
			return 2, errors.New(uiUsage)
		}
		name := rest[0]
		if !dashboard.ValidName(name) {
			return 2, fmt.Errorf("%q is not a skin name (built-in looks need no check)", name)
		}
		s, err := stats.Build(root, cfg, time.Now())
		if err != nil {
			return 1, err
		}
		data, _ := json.Marshal(s)
		ps, err := dashboard.CheckSkin(filepath.Join(root, dashboard.SkinsDir, name), data)
		if err != nil {
			return 1, err
		}
		errs := 0
		for _, p := range ps {
			kind := "warn "
			if p.Error {
				kind = "error"
				errs++
			}
			fmt.Fprintf(out, "%s %s\n", kind, p.Text)
		}
		if errs > 0 {
			fmt.Fprintf(out, "%s: %d errors; fix them before isekai ui use %s\n", name, errs, name)
			return 1, nil
		}
		// It must also render.
		looks, lerrs := dashboard.Looks(root)
		for _, e := range lerrs {
			return 1, e
		}
		var buf bytes.Buffer
		if err := dashboard.Render(&buf, s, looks, name, true); err != nil {
			return 1, err
		}
		fmt.Fprintf(out, "%s: ok\n", name)
		return 0, nil
	case "use":
		if len(rest) != 1 {
			return 2, errors.New(uiUsage)
		}
		name := rest[0]
		looks, _ := dashboard.Looks(root)
		found := false
		for _, l := range looks {
			found = found || l.ID == name
		}
		if !found {
			return 1, fmt.Errorf("no look %q; see isekai ui list", name)
		}
		if name != "isekai" && name != "corporate" {
			if !dashboard.ValidName(name) {
				return 1, fmt.Errorf("skin %q: rename the folder to lowercase letters, digits and dashes", name)
			}
			s, _ := stats.Build(root, cfg, time.Now())
			data, _ := json.Marshal(s)
			ps, err := dashboard.CheckSkin(filepath.Join(root, dashboard.SkinsDir, name), data)
			if err != nil {
				return 1, err
			}
			for _, p := range ps {
				if p.Error {
					return 1, fmt.Errorf("%s does not pass isekai ui check %s: %s", name, name, p.Text)
				}
			}
		}
		if err := dashboard.SetLook(filepath.Join(root, config.File), name); err != nil {
			return 1, err
		}
		fmt.Fprintf(out, "dashboard look: %s (commit .isekai/ so everyone gets it; switch back with isekai ui use isekai)\n", name)
		return 0, nil
	}
	return 2, fmt.Errorf("unknown ui command %q\n%s", sub, uiUsage)
}

func rel(root, p string) string {
	if r, err := filepath.Rel(root, p); err == nil {
		return filepath.ToSlash(r)
	}
	return p
}
