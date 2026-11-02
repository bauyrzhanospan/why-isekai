package journal

import (
	"fmt"
	"strings"
	"time"

	"github.com/bauyrzhanospan/why-isekai/internal/config"
	"github.com/bauyrzhanospan/why-isekai/internal/gitx"
)

// Template returns the commented block prepare-commit-msg inserts: answering
// a concrete prompt is easier than filling an empty field, so the author
// sees the staged files, the keys and the items still open.
func Template(root string, cfg config.Config, cc string) string {
	var b strings.Builder
	p := func(f string, a ...any) { b.WriteString(cc + " " + fmt.Sprintf(f, a...) + "\n") }
	b.WriteString("\n")
	p("isekai: end the message with trailers (remove the leading %q; commented lines are dropped).", cc)
	p("Why is required for agent commits except style:/docs:/ci:. One line per value.")
	b.WriteString(cc + "Why: <the problem, constraint or goal — not what changed>\n")
	b.WriteString(cc + "Tried: <approach you rejected>; <why>\n")
	b.WriteString(cc + "Next: <work left for the next session>\n")
	b.WriteString(cc + "Agent: <model id, if you are an AI agent>\n")
	if br, err := BuildBrief(root, cfg, 1, time.Now()); err == nil && len(br.Open) > 0 {
		p("Open items — finish one? add  Done: <exact text>")
		for i, o := range br.Open {
			if i == 10 {
				p("  … %d more (isekai brief)", len(br.Open)-10)
				break
			}
			p("  %s  [%s]", o.Text, o.Commit)
		}
	}
	if files, err := gitx.Run(root, "diff", "--cached", "--name-status"); err == nil && files != "" {
		p("Staged:")
		lines := strings.Split(files, "\n")
		for i, l := range lines {
			if i == 30 {
				p("  … %d more", len(lines)-30)
				break
			}
			p("  %s", strings.ReplaceAll(l, "\t", " "))
		}
	}
	b.WriteString(cc + "\n")
	return b.String()
}
