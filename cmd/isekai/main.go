// Command isekai is the reference implementation of the why-isekai spec:
// project memory for AI agents, kept in git.
package main

import (
	"fmt"
	"io"
	"os"
)

// Set by the release build (-ldflags "-X main.version=...").
var version = "dev"

const usage = `isekai — project memory for AI agents, kept in git (why-isekai spec 0.1)

Every agent session is an isekai: it lands in an unknown world, not knowing
what was done before. isekai records why things changed, what was tried and
what is left, in plain git commit trailers.

Usage:
  isekai init [--mode warn|block] [--claude] [--mcp]
                                               install git hooks, config, AGENTS.md line
  isekai brief [-n N] [--format text|md|json]  summary for the start of a session
  isekai stats [--format text|json]            numbers for humans: agents, open work, growth, level
  isekai dashboard [-o FILE] [--look L] [--open]
                                               one offline HTML page with those numbers
  isekai check [<range>] [--strict] [--format text|json]
                                               validate commits (for CI)
  isekai run [--scope S] [--as NAME] -- <cmd>  run cmd with secret:// env vars resolved,
                                               values masked in its output
  isekai secrets list|set|init                 manage secret references (values never shown to agents)
  isekai wiki add|confirm|stale|list           project wiki in .isekai/wiki
  isekai mcp [--repo DIR]                      MCP server over stdio (brief, check, wiki, secrets)
  isekai hook <name> ...                       entry points called by git / agent hooks
  isekai version

Commit trailers (last paragraph of the message):
  Why:   the reason — problem, constraint or goal (required for agent commits)
  Tried: rejected approach; why
  Next:  work left for the next session (repeatable)
  Done:  exact text of an earlier Next this commit finishes (repeatable)
  Agent: model id of the AI agent that wrote the commit

Spec and docs: https://github.com/bauyrzhanospan/why-isekai
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stdout, usage)
		return 0
	}
	cmd, rest := args[0], args[1:]
	var err error
	code := 0
	switch cmd {
	case "init":
		err = cmdInit(rest, stdout)
	case "brief":
		err = cmdBrief(rest, stdout)
	case "stats":
		err = cmdStats(rest, stdout)
	case "dashboard":
		err = cmdDashboard(rest, stdout)
	case "check":
		code, err = cmdCheck(rest, stdout)
	case "hook":
		code, err = cmdHook(rest, stdin, stdout, stderr)
	case "run":
		code, err = cmdRun(rest, stdin, stdout, stderr)
	case "secrets":
		err = cmdSecrets(rest, stdin, stdout)
	case "wiki":
		code, err = cmdWiki(rest, stdout, stderr)
	case "mcp":
		code, err = cmdMCP(rest, stdin, stdout, stderr)
	case "version", "--version", "-v":
		fmt.Fprintln(stdout, "isekai", version)
	case "help", "--help", "-h":
		fmt.Fprint(stdout, usage)
	default:
		fmt.Fprintf(stderr, "isekai: unknown command %q\n\n%s", cmd, usage)
		return 2
	}
	if err != nil {
		fmt.Fprintln(stderr, "isekai:", err)
		if code == 0 {
			code = 1
		}
	}
	return code
}
