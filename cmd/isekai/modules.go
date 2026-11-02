package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bauyrzhanospan/why-isekai/internal/gitx"
	"github.com/bauyrzhanospan/why-isekai/internal/secrets"
	"github.com/bauyrzhanospan/why-isekai/internal/wiki"
)

func cmdRun(args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	fs := newFlags("run")
	scope := fs.String("scope", "", "expose every allowed secret of this scope as an env var")
	as := fs.String("as", "", "agent name for access rules (default: detected)")
	if err := fs.Parse(args); err != nil {
		return 2, fmt.Errorf("run: %v (usage: isekai run [--scope S] -- <command> [args…])", err)
	}
	root, err := repoRoot()
	if err != nil {
		return 2, err
	}
	name := agentName()
	if *as != "" {
		name = *as
	}
	return secrets.Run(fs.Args(), secrets.RunOptions{
		Root: root, Scope: *scope, AgentName: name, Env: os.Environ(),
		Stdin: stdin, Stdout: stdout, Stderr: stderr, Warn: stderr,
	})
}

const secretsUsage = `usage:
  isekai secrets init                 create a personal age key (outside the repo) and .isekai/secrets.toml
  isekai secrets list                 references you may use (never values)
  isekai secrets set <ref>            read the value from stdin and store it encrypted
  isekai secrets rm <ref>             delete a secret
  isekai secrets add-recipient <age1…> [comment]   let another key (teammate, CI) decrypt
  isekai secrets rekey                re-encrypt everything to the current recipients
Use secrets: put VAR=secret://<scope>/<name> in the environment and run
  isekai run -- <command>
Values are never printed; command output has them replaced by their reference.`

func cmdSecrets(args []string, stdin io.Reader, out io.Writer) error {
	if len(args) == 0 {
		return errors.New(secretsUsage)
	}
	root, err := repoRoot()
	if err != nil {
		return err
	}
	sub, rest := args[0], args[1:]
	if sub == "init" {
		return secretsInit(root, out)
	}
	c, ok, err := secrets.LoadConfig(root)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("this project has no %s yet; run: isekai secrets init", secrets.ConfigFile)
	}
	st, err := secrets.Open(root, c)
	if err != nil {
		return err
	}
	ageStore, isAge := st.(*secrets.AgeStore)
	needAge := func() error {
		if !isAge {
			return fmt.Errorf("%s is managed by sops; use: sops edit %s", c.Store.File, c.Store.File)
		}
		return nil
	}
	switch sub {
	case "list", "ls":
		names, err := st.Names()
		if err != nil {
			return err
		}
		who := agentName()
		hidden := 0
		for _, n := range names {
			r, err := secrets.ParseRef(n)
			if err != nil {
				continue
			}
			switch {
			case who != "" && c.Allowed(r, who):
				fmt.Fprintln(out, r.String())
			case who != "":
				hidden++
			case !c.Allowed(r, ""):
				fmt.Fprintln(out, r.String(), "(humans: denied)")
			case !c.Allowed(r, "agent"):
				fmt.Fprintln(out, r.String(), "(agents: denied)")
			default:
				fmt.Fprintln(out, r.String())
			}
		}
		if hidden > 0 {
			fmt.Fprintf(out, "(%d more not available to agent %s)\n", hidden, who)
		}
		if len(names) == 0 {
			fmt.Fprintln(out, "no secrets yet; add one: echo 'value' | isekai secrets set secret://dev/name")
		}
		return nil
	case "set":
		if err := needAge(); err != nil {
			return err
		}
		if len(rest) != 1 {
			return errors.New("usage: isekai secrets set <secret://scope/name>  (value on stdin, e.g. printf %s \"$VALUE\" | isekai secrets set …)")
		}
		r, err := secrets.ParseRef(rest[0])
		if err != nil {
			return err
		}
		if f, ok := stdin.(*os.File); ok {
			if st, err := f.Stat(); err == nil && st.Mode()&os.ModeCharDevice != 0 {
				fmt.Fprintf(os.Stderr, "value for %s (input not hidden; end with Ctrl-D): ", r)
			}
		}
		data, err := io.ReadAll(stdin)
		if err != nil {
			return err
		}
		v := strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
		if v == "" {
			return errors.New("empty value on stdin")
		}
		if err := ageStore.Set(r.Key(), v); err != nil {
			return err
		}
		fmt.Fprintf(out, "stored %s in %s (commit it; values are encrypted)\n", r, c.Store.File)
		return nil
	case "rm", "remove":
		if err := needAge(); err != nil {
			return err
		}
		if len(rest) != 1 {
			return errors.New("usage: isekai secrets rm <secret://scope/name>")
		}
		r, err := secrets.ParseRef(rest[0])
		if err != nil {
			return err
		}
		if err := ageStore.Remove(r.Key()); err != nil {
			return err
		}
		fmt.Fprintf(out, "removed %s\n", r)
		return nil
	case "add-recipient":
		if err := needAge(); err != nil {
			return err
		}
		if len(rest) < 1 {
			return errors.New("usage: isekai secrets add-recipient <age1…> [comment]")
		}
		comment := strings.Join(rest[1:], " ")
		if comment == "" {
			comment = "added " + time.Now().Format("2006-01-02")
		}
		added, err := ageStore.AddRecipient(rest[0], comment)
		if err != nil {
			return err
		}
		if !added {
			fmt.Fprintln(out, "already a recipient")
			return nil
		}
		fmt.Fprintln(out, "recipient added; now run: isekai secrets rekey")
		return nil
	case "rekey":
		if err := needAge(); err != nil {
			return err
		}
		n, err := ageStore.Rekey()
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "re-encrypted %d secrets\n", n)
		return nil
	}
	return fmt.Errorf("unknown secrets command %q\n%s", sub, secretsUsage)
}

func secretsInit(root string, out io.Writer) error {
	c, ok, err := secrets.LoadConfig(root)
	if err != nil {
		return err
	}
	if !ok {
		if err := os.MkdirAll(root+"/.isekai", 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(root+"/"+secrets.ConfigFile, []byte(secrets.ConfigTemplate), 0o644); err != nil {
			return err
		}
		fmt.Fprintln(out, "created  ", secrets.ConfigFile)
		c, _, err = secrets.LoadConfig(root)
		if err != nil {
			return err
		}
	}
	if c.Store.Type != "age" {
		fmt.Fprintf(out, "store type is %s; nothing else to set up\n", c.Store.Type)
		return nil
	}
	pub, created, err := secrets.GenerateKey()
	if err != nil {
		return err
	}
	if created {
		fmt.Fprintln(out, "created  ", secrets.KeyFile(), "(your private key: back it up, never commit it)")
	} else {
		fmt.Fprintln(out, "using    ", secrets.KeyFile())
	}
	st := &secrets.AgeStore{Root: root, File: c.Store.File, Recipients: c.Store.Recipients}
	user, _ := gitx.Run(root, "config", "user.email")
	if user == "" {
		user = "me"
	}
	if added, err := st.AddRecipient(pub, user); err != nil {
		return err
	} else if added {
		fmt.Fprintln(out, "added    ", pub, "to", c.Store.Recipients)
	}
	fmt.Fprintln(out, "Next: echo 'value' | isekai secrets set secret://dev/db_password")
	fmt.Fprintln(out, "Then: DB_PASSWORD=secret://dev/db_password isekai run -- <command>")
	fmt.Fprintln(out, "Note: access rules are a seatbelt, not a security boundary — the boundary is which keys exist where the agent runs.")
	return nil
}

const wikiUsage = `usage:
  isekai wiki add <page> "<text>" [--commit SHA] [--author NAME]
  isekai wiki list [<page>]            pages with counts, or a page's numbered entries
  isekai wiki confirm <page> <n>       entry n is still true: refresh its date
  isekai wiki stale [--days N]         entries not confirmed for N days (default 90)
Standard pages: gotchas, glossary, priorities (humans decide), people (work facts only).`

func cmdWiki(args []string, out, errw io.Writer) (int, error) {
	if len(args) == 0 {
		return 2, errors.New(wikiUsage)
	}
	root, err := repoRoot()
	if err != nil {
		return 2, err
	}
	now := time.Now()
	sub, rest := args[0], args[1:]
	switch sub {
	case "add":
		fs := newFlags("wiki add")
		commit := fs.String("commit", "", "related commit")
		author := fs.String("author", "", "author label")
		pos, err := parseInterspersed(fs, rest)
		if err != nil || len(pos) < 2 {
			return 2, errors.New(wikiUsage)
		}
		name := agentName()
		a := *author
		if a == "" {
			a = name
		}
		if a == "" {
			a = "human"
		}
		e, warns, err := wiki.Add(root, pos[0], strings.Join(pos[1:], " "), wiki.AddOptions{Author: a, Agent: name != "", Commit: *commit, Now: now})
		if err != nil {
			return 1, err
		}
		for _, w := range warns {
			fmt.Fprintln(errw, "isekai: note:", w)
		}
		fmt.Fprintf(out, "added %s #%d: %s\n", e.Page, e.N, e.Text)
		return 0, nil
	case "list", "ls":
		if len(rest) == 0 {
			sum, err := wiki.Summary(root, wiki.DefaultStaleDays, now)
			if err != nil {
				fmt.Fprintln(out, "no wiki yet; start one: isekai wiki add gotchas \"<problem> → <fix>\"")
				return 0, nil
			}
			for _, s := range sum {
				fmt.Fprintf(out, "%-12s %d entries, %d possibly stale\n", s.Page, s.Entries, s.Stale)
			}
			return 0, nil
		}
		es, _, err := wiki.Read(root, rest[0])
		if err != nil {
			return 1, err
		}
		for _, e := range es {
			mark := ""
			if wiki.IsStale(e, wiki.DefaultStaleDays, now) {
				mark = " (possibly stale)"
			}
			fmt.Fprintf(out, "%d. %s [%s %s %s]%s\n", e.N, e.Text, e.Date, e.Commit, e.Author, mark)
		}
		return 0, nil
	case "confirm":
		if len(rest) != 2 {
			return 2, errors.New("usage: isekai wiki confirm <page> <n>")
		}
		n, err := strconv.Atoi(rest[1])
		if err != nil {
			return 2, fmt.Errorf("entry number %q is not a number", rest[1])
		}
		e, err := wiki.Confirm(root, rest[0], n, now)
		if err != nil {
			return 1, err
		}
		fmt.Fprintf(out, "confirmed %s #%d (%s)\n", e.Page, e.N, e.Date)
		return 0, nil
	case "stale":
		fs := newFlags("wiki stale")
		days := fs.Int("days", wiki.DefaultStaleDays, "age in days")
		if err := fs.Parse(rest); err != nil {
			return 2, errors.New(wikiUsage)
		}
		es, err := wiki.Stale(root, *days, now)
		if err != nil {
			return 1, err
		}
		for _, e := range es {
			fmt.Fprintf(out, "%s #%d [%s] %s\n", e.Page, e.N, e.Date, e.Text)
		}
		if len(es) == 0 {
			fmt.Fprintf(out, "nothing older than %d days\n", *days)
		} else {
			fmt.Fprintln(out, "Still true? isekai wiki confirm <page> <n>. Wrong? delete the line and say why in the commit's Why.")
		}
		return 0, nil
	}
	return 2, fmt.Errorf("unknown wiki command %q\n%s", sub, wikiUsage)
}
