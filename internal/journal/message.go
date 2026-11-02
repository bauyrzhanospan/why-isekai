// Package journal implements Module 1 of the spec: commit trailers
// Why / Tried / Next / Done / Agent, their validation and the brief.
package journal

import (
	"regexp"
	"strings"
)

// Keys defined by the spec, in canonical spelling.
const (
	KeyWhy   = "Why"
	KeyTried = "Tried"
	KeyNext  = "Next"
	KeyDone  = "Done"
	KeyAgent = "Agent"
)

// Trailer is one `Key: value` line from the trailer block.
type Trailer struct {
	Key   string
	Value string
}

// Message is a parsed commit message.
type Message struct {
	Subject  string
	Body     string // everything between subject and trailer block
	Trailers []Trailer
	// Conventional Commits parts of the subject; Type is "" if the subject
	// is not in that form.
	Type        string
	Description string
}

// Values returns all values for key (case-insensitive), in order.
func (m Message) Values(key string) []string {
	var out []string
	for _, t := range m.Trailers {
		if strings.EqualFold(t.Key, key) {
			out = append(out, t.Value)
		}
	}
	return out
}

var (
	trailerRe = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9-]*)[ \t]*:[ \t]*(.*)$`)
	ccRe      = regexp.MustCompile(`^([A-Za-z]+)(\([^)]*\))?!?:[ \t]*(.*)$`)
)

// StripComments removes lines starting with commentChar and everything after
// a scissors line, as `git commit --cleanup=strip|scissors` would. The
// commit-msg hook sees the message before git cleans it up.
func StripComments(msg string, commentChar string) string {
	if commentChar == "" {
		commentChar = "#"
	}
	scissors := commentChar + " ------------------------ >8 ------------------------"
	var out []string
	for _, line := range strings.Split(strings.ReplaceAll(msg, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, scissors) {
			break
		}
		if strings.HasPrefix(line, commentChar) {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// CutScissors drops the scissors line and everything after it
// (git commit --verbose puts the diff there).
func CutScissors(msg, commentChar string) string {
	if commentChar == "" {
		commentChar = "#"
	}
	scissors := commentChar + " ------------------------ >8 ------------------------"
	if i := strings.Index(msg, "\n"+scissors); i >= 0 {
		return msg[:i+1]
	}
	if strings.HasPrefix(msg, scissors) {
		return ""
	}
	return msg
}

// Parse is ParseComment with "#" as the comment character.
func Parse(msg string) Message { return ParseComment(msg, "#") }

// git treats a block with at least 25% trailer lines as trailers when one of
// them starts with a git-generated prefix (trailer.c, git_generated_prefixes).
var gitGeneratedPrefixes = []string{"Signed-off-by: ", "(cherry picked from commit "}

// ParseComment splits a commit message into subject, body and trailers,
// following git's trailer rules (git interpret-trailers --parse):
//   - the trailer block is the last paragraph, never the subject paragraph;
//   - comment lines (starting with commentChar) and trailing comment/blank
//     lines are ignored, so the hook (which sees comments) and git log agree;
//   - the block counts as trailers when all its lines are trailers, or when at
//     least 25% are and one starts with a git-generated prefix
//     (Signed-off-by, "(cherry picked from commit").
func ParseComment(msg, commentChar string) Message {
	if commentChar == "" {
		commentChar = "#"
	}
	msg = strings.ReplaceAll(msg, "\r\n", "\n")
	isComment := func(l string) bool { return strings.HasPrefix(l, commentChar) }
	blank := func(l string) bool { return strings.TrimSpace(l) == "" }
	lines := strings.Split(msg, "\n")
	// Drop trailing blank and comment lines (git's ignored log message tail).
	end := len(lines)
	for end > 0 && (blank(lines[end-1]) || isComment(lines[end-1])) {
		end--
	}
	lines = lines[:end]
	start := 0
	for start < len(lines) && (blank(lines[start]) || isComment(lines[start])) {
		start++
	}
	lines = lines[start:]
	var m Message
	if len(lines) == 0 {
		return m
	}
	i := 0
	var subj []string
	for ; i < len(lines) && !blank(lines[i]); i++ {
		if !isComment(lines[i]) {
			subj = append(subj, strings.TrimSpace(lines[i]))
		}
	}
	m.Subject = strings.Join(subj, " ")
	if mm := ccRe.FindStringSubmatch(m.Subject); mm != nil {
		m.Type = strings.ToLower(mm[1])
		m.Description = strings.TrimSpace(mm[3])
	} else {
		m.Description = m.Subject
	}
	rest := lines[i:]

	// Last paragraph of the rest.
	bstart := len(rest)
	for bstart > 0 && !blank(rest[bstart-1]) {
		bstart--
	}
	block := rest[bstart:]
	var trailers []Trailer
	trailerLines, otherLines, recognized := 0, 0, false
	lastWasTrailer := false
	for _, line := range block {
		if isComment(line) {
			continue
		}
		if (line[0] == ' ' || line[0] == '\t') && lastWasTrailer {
			trailers[len(trailers)-1].Value += " " + strings.TrimSpace(line)
			continue
		}
		for _, p := range gitGeneratedPrefixes {
			if strings.HasPrefix(line, p) {
				recognized = true
			}
		}
		if mm := trailerRe.FindStringSubmatch(line); mm != nil {
			trailers = append(trailers, Trailer{Key: mm[1], Value: strings.TrimSpace(mm[2])})
			trailerLines++
			lastWasTrailer = true
			continue
		}
		lastWasTrailer = false
		if strings.HasPrefix(line, "(cherry picked from commit ") {
			trailerLines++
			continue
		}
		otherLines++
	}
	isBlock := len(block) > 0 && trailerLines > 0 &&
		(otherLines == 0 || (recognized && trailerLines*3 >= otherLines))
	if isBlock {
		m.Trailers = trailers
		rest = rest[:bstart]
	}
	var body []string
	for _, l := range rest {
		if !isComment(l) {
			body = append(body, l)
		}
	}
	m.Body = strings.TrimSpace(strings.Join(body, "\n"))
	return m
}
