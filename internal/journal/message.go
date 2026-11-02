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

// Parse splits a (comment-free) commit message into subject, body and
// trailers. The trailer block is the last paragraph, when it is not the
// subject paragraph and every line in it is either `Key: value` or a
// continuation line starting with whitespace. This matches
// `git interpret-trailers --parse` for messages written to the spec.
func Parse(msg string) Message {
	msg = strings.ReplaceAll(msg, "\r\n", "\n")
	lines := strings.Split(strings.TrimRight(msg, " \t\n"), "\n")
	// drop leading blank lines
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	var m Message
	if len(lines) == 0 {
		return m
	}
	// Subject paragraph: lines up to the first blank line, joined.
	i := 0
	var subj []string
	for ; i < len(lines) && strings.TrimSpace(lines[i]) != ""; i++ {
		subj = append(subj, strings.TrimSpace(lines[i]))
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
	start := len(rest)
	for start > 0 && strings.TrimSpace(rest[start-1]) != "" {
		start--
	}
	block := rest[start:]
	var trailers []Trailer
	ok := len(block) > 0
	for _, line := range block {
		if (line[0] == ' ' || line[0] == '\t') && len(trailers) > 0 {
			trailers[len(trailers)-1].Value += " " + strings.TrimSpace(line)
			continue
		}
		mm := trailerRe.FindStringSubmatch(line)
		if mm == nil {
			ok = false
			break
		}
		trailers = append(trailers, Trailer{Key: mm[1], Value: strings.TrimSpace(mm[2])})
	}
	if ok {
		m.Trailers = trailers
		rest = rest[:start]
	}
	m.Body = strings.TrimSpace(strings.Join(rest, "\n"))
	return m
}
