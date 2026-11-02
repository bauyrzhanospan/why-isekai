// Package gitx runs the git binary. IsekAI deliberately shells out to git
// instead of reimplementing it, so behaviour matches what users see.
package gitx

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Run executes git with args in dir and returns trimmed stdout.
func Run(dir string, args ...string) (string, error) {
	out, err := RunRaw(dir, nil, args...)
	return strings.TrimRight(out, "\n"), err
}

// RunRaw executes git with optional stdin and returns stdout untrimmed.
func RunRaw(dir string, stdin []byte, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return stdout.String(), fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.String(), nil
}

// Root returns the top level of the work tree containing dir.
func Root(dir string) (string, error) {
	return Run(dir, "rev-parse", "--show-toplevel")
}

// HooksDir returns the directory git runs hooks from (honours core.hooksPath).
func HooksDir(dir string) (string, error) {
	return Run(dir, "rev-parse", "--path-format=absolute", "--git-path", "hooks")
}

// CommentChar returns core.commentString or core.commentChar, defaulting to
// "#" (also for "auto", where git picks "#" unless the message uses it).
func CommentChar(dir string) string {
	if c, err := Run(dir, "config", "--get", "core.commentString"); err == nil && c != "" && c != "auto" {
		return c
	}
	c, err := Run(dir, "config", "--get", "core.commentChar")
	if err != nil || c == "" || c == "auto" {
		return "#"
	}
	return c
}

// Commit is one entry read by Log.
type Commit struct {
	Hash    string
	Short   string
	Time    string // ISO 8601 committer date
	Author  string
	Message string // full raw message
}

const (
	fieldSep  = "\x1f"
	recordSep = "\x1e"
)

// Log reads commits. args are passed to `git log` before the format flag,
// e.g. "-n", "20" or a revision range. An empty repository yields no commits.
func Log(dir string, args ...string) ([]Commit, error) {
	if _, err := Run(dir, "rev-parse", "--verify", "-q", "HEAD"); err != nil {
		return nil, nil
	}
	a := append([]string{"log", "--format=%H" + fieldSep + "%h" + fieldSep + "%cI" + fieldSep + "%an" + fieldSep + "%B" + recordSep}, args...)
	out, err := RunRaw(dir, nil, a...)
	if err != nil {
		return nil, err
	}
	var cs []Commit
	for _, rec := range strings.Split(out, recordSep) {
		rec = strings.TrimLeft(rec, "\n")
		if rec == "" {
			continue
		}
		f := strings.SplitN(rec, fieldSep, 5)
		if len(f) != 5 {
			continue
		}
		cs = append(cs, Commit{Hash: f[0], Short: f[1], Time: f[2], Author: f[3], Message: f[4]})
	}
	return cs, nil
}
