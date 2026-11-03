package tracker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// GitHub talks to GitHub Issues through the gh CLI, which already handles
// authentication (gh auth login, GH_TOKEN in CI). isekai holds no token.
type GitHub struct {
	Dir  string // repository directory; gh infers owner/repo from its remote
	Repo string // optional owner/repo override
}

// Name implements Adapter.
func (g *GitHub) Name() string { return "github" }

func (g *GitHub) gh(args ...string) ([]byte, error) {
	bin, err := exec.LookPath("gh")
	if err != nil {
		return nil, errors.New("the GitHub CLI (gh) is not installed: https://cli.github.com — then run gh auth login")
	}
	if g.Repo != "" {
		args = append(args, "--repo", g.Repo)
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = g.Dir
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("gh %s: %s", strings.Join(args[:2], " "), msg)
	}
	return out.Bytes(), nil
}

// EnsureLabels implements Adapter.
func (g *GitHub) EnsureLabels(labels ...string) error {
	colors := map[int]string{0: "7DF3FF", 1: "F0B33C"}
	for i, l := range labels {
		if l == "" {
			continue
		}
		if _, err := g.gh("label", "create", l, "--force", "--color", colors[i%2], "--description", "managed by why-isekai"); err != nil {
			return err
		}
	}
	return nil
}

type ghIssue struct {
	Number   int    `json:"number"`
	Title    string `json:"title"`
	Body     string `json:"body"`
	State    string `json:"state"`
	URL      string `json:"url"`
	Comments []struct {
		Author struct {
			Login string `json:"login"`
		} `json:"author"`
		Body      string `json:"body"`
		CreatedAt string `json:"createdAt"`
	} `json:"comments"`
}

// List implements Adapter.
func (g *GitHub) List(label string) ([]Issue, error) {
	out, err := g.gh("issue", "list", "--label", label, "--state", "all", "--limit", "500", "--json", "number,title,body,state,url,comments")
	if err != nil {
		return nil, err
	}
	var raw []ghIssue
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("gh issue list: unexpected output: %w", err)
	}
	var is []Issue
	for _, r := range raw {
		i := Issue{Number: r.Number, Title: r.Title, Body: r.Body, Open: strings.EqualFold(r.State, "open"), URL: r.URL}
		for _, c := range r.Comments {
			i.Comments = append(i.Comments, Comment{Author: c.Author.Login, Body: c.Body, Created: c.CreatedAt})
		}
		is = append(is, i)
	}
	return is, nil
}

var issueNumRe = regexp.MustCompile(`/issues/([0-9]+)\s*$`)

// Create implements Adapter.
func (g *GitHub) Create(title, body string, labels []string) (Issue, error) {
	args := []string{"issue", "create", "--title", title, "--body", body}
	for _, l := range labels {
		args = append(args, "--label", l)
	}
	out, err := g.gh(args...)
	if err != nil {
		return Issue{}, err
	}
	url := strings.TrimSpace(string(out))
	i := Issue{Title: title, Body: body, Open: true, URL: url}
	if m := issueNumRe.FindStringSubmatch(url); m != nil {
		i.Number, _ = strconv.Atoi(m[1])
	}
	return i, nil
}

// Close implements Adapter.
func (g *GitHub) Close(number int, comment string) error {
	_, err := g.gh("issue", "close", strconv.Itoa(number), "--comment", comment)
	return err
}
