package secrets

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// IdleFlush: a held-back tail that might be the start of a secret is written
// out after this much silence, so prompts like "Continue? [y/n" appear.
const IdleFlush = 50 * time.Millisecond

// MinMaskLen: shorter values are not masked (they would match everywhere);
// `isekai run` warns about them instead.
const MinMaskLen = 4

type needle struct {
	b    []byte
	repl []byte
}

// Masker builds a writer that replaces secret values in a stream with their
// references (spec §2.4.3). It catches the raw value, its base64 (standard,
// URL-safe, unpadded) and hex encodings, CRLF variants and, for multi-line
// values, each sufficiently long line on its own.
//
// Masking is a seatbelt: a process that transforms the value in other ways
// (reverses it, prints it byte by byte with delays in between, …) can still
// leak it. The security boundary is the key, not the mask.
type Masker struct {
	needles []needle
	maxLen  int
	byFirst [256][]int // needle indexes by first byte, for the prefix scan
}

// NewMasker builds a masker for values keyed by reference string.
func NewMasker(values map[string]string) *Masker {
	m := &Masker{}
	seen := map[string]bool{}
	add := func(v, ref string) {
		if len(v) < MinMaskLen || seen[v] {
			return
		}
		seen[v] = true
		m.needles = append(m.needles, needle{[]byte(v), []byte("[" + ref + "]")})
	}
	for ref, v := range values {
		variants := []string{v, strings.TrimRight(v, "\r\n")}
		if strings.Contains(v, "\n") {
			variants = append(variants, strings.ReplaceAll(v, "\n", "\r\n"))
		}
		for _, x := range variants {
			add(x, ref)
			for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.URLEncoding, base64.RawStdEncoding, base64.RawURLEncoding} {
				add(enc.EncodeToString([]byte(x)), ref)
			}
			add(hex.EncodeToString([]byte(x)), ref)
			add(strings.ToUpper(hex.EncodeToString([]byte(x))), ref)
			if j, err := json.Marshal(x); err == nil {
				add(string(j[1:len(j)-1]), ref) // JSON-escaped, e.g. inside a config dump
			}
			add(url.QueryEscape(x), ref)
			add(url.PathEscape(x), ref)
		}
		if strings.Contains(v, "\n") {
			for _, line := range strings.Split(v, "\n") {
				line = strings.TrimRight(line, "\r")
				if len(strings.TrimSpace(line)) >= 8 {
					add(line, ref)
				}
			}
		}
	}
	// Longest first so a full value wins over one of its lines.
	sort.Slice(m.needles, func(i, j int) bool { return len(m.needles[i].b) > len(m.needles[j].b) })
	for i, n := range m.needles {
		if len(n.b) > m.maxLen {
			m.maxLen = len(n.b)
		}
		m.byFirst[n.b[0]] = append(m.byFirst[n.b[0]], i)
	}
	return m
}

// Writer returns a WriteCloser masking into w. Close flushes held bytes.
func (m *Masker) Writer(w io.Writer) io.WriteCloser {
	return &maskWriter{m: m, w: w}
}

// String masks a complete string.
func (m *Masker) String(s string) string {
	var b bytes.Buffer
	mw := m.Writer(&b)
	io.WriteString(mw, s)
	mw.Close()
	return b.String()
}

type maskWriter struct {
	mu    sync.Mutex
	m     *Masker
	w     io.Writer
	buf   []byte
	timer *time.Timer
	err   error
}

func (mw *maskWriter) Write(p []byte) (int, error) {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	if mw.err != nil {
		return 0, mw.err
	}
	mw.buf = append(mw.buf, p...)
	if err := mw.drain(false); err != nil {
		return 0, err
	}
	if len(mw.buf) > 0 {
		if mw.timer == nil {
			mw.timer = time.AfterFunc(IdleFlush, mw.idle)
		} else {
			mw.timer.Reset(IdleFlush)
		}
	}
	return len(p), nil
}

// idle flushes a held tail after a pause in output. At worst this shows the
// first bytes of a value that never got completed.
func (mw *maskWriter) idle() {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	if len(mw.buf) > 0 && mw.err == nil {
		mw.err = mw.drain(true)
	}
}

func (mw *maskWriter) Close() error {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	if mw.timer != nil {
		mw.timer.Stop()
	}
	if mw.err != nil {
		return mw.err
	}
	return mw.drain(true)
}

// drain emits everything that can no longer be part of a secret: matches are
// replaced, and the only bytes held back are a tail that is a prefix of some
// needle. So output is never delayed unless it might be a secret.
func (mw *maskWriter) drain(final bool) error {
	var out []byte
	for len(mw.buf) > 0 {
		// earliest match
		at, which := -1, -1
		for i, n := range mw.m.needles {
			if k := bytes.Index(mw.buf, n.b); k >= 0 && (at < 0 || k < at) {
				at, which = k, i
			}
		}
		if at >= 0 {
			out = append(out, mw.buf[:at]...)
			out = append(out, mw.m.needles[which].repl...)
			mw.buf = mw.buf[at+len(mw.m.needles[which].b):]
			continue
		}
		cut := len(mw.buf)
		if !final {
			start := len(mw.buf) - mw.m.maxLen + 1
			if start < 0 {
				start = 0
			}
			for p := start; p < len(mw.buf); p++ {
				if mw.isPrefix(mw.buf[p:]) {
					cut = p
					break
				}
			}
		}
		out = append(out, mw.buf[:cut]...)
		mw.buf = mw.buf[cut:]
		break
	}
	if len(out) == 0 {
		return nil
	}
	_, err := mw.w.Write(out)
	return err
}

func (mw *maskWriter) isPrefix(tail []byte) bool {
	for _, i := range mw.m.byFirst[tail[0]] {
		n := mw.m.needles[i].b
		if len(tail) < len(n) && bytes.HasPrefix(n, tail) {
			return true
		}
	}
	return false
}
