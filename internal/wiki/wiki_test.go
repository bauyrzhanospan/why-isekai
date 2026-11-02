package wiki

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAddConfirmStale(t *testing.T) {
	root := t.TempDir()
	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	if _, _, err := Add(root, "gotchas", "npm test flakes on ws.spec.ts; rerun once", AddOptions{Author: "claude-opus-5-5", Agent: true, Commit: "a1b2c3d", Now: old}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Add(root, "gotchas", "legacy/admin is the old admin; do not touch", AddOptions{Now: now}); err != nil {
		t.Fatal(err)
	}
	es, _, err := Read(root, "gotchas")
	if err != nil || len(es) != 2 {
		t.Fatal(es, err)
	}
	if es[0].Date != "2026-01-01" || es[0].Commit != "a1b2c3d" || es[0].Author != "claude-opus-5-5" || es[1].Author != "human" || es[1].Commit != "-" {
		t.Fatalf("%+v", es)
	}
	st, _ := Stale(root, DefaultStaleDays, now)
	if len(st) != 1 || st[0].N != 1 {
		t.Fatalf("stale = %+v", st)
	}
	if _, err := Confirm(root, "gotchas", 1, now); err != nil {
		t.Fatal(err)
	}
	if st, _ := Stale(root, DefaultStaleDays, now); len(st) != 0 {
		t.Fatalf("still stale: %+v", st)
	}
	data, _ := os.ReadFile(filepath.Join(root, Dir, "gotchas.md"))
	if !strings.HasPrefix(string(data), "# Gotchas") || !strings.Contains(string(data), "[2026-11-03 a1b2c3d claude-opus-5-5]") {
		t.Fatalf("page:\n%s", data)
	}
}

func TestAgentGuards(t *testing.T) {
	root := t.TempDir()
	e, w, err := Add(root, "priorities", "ship the brief before secrets", AddOptions{Author: "codex", Agent: true})
	if err != nil || !strings.HasSuffix(e.Text, "(proposed)") || len(w) == 0 {
		t.Fatal(e, w, err)
	}
	e, w, err = Add(root, "people", "Erlan approves API changes", AddOptions{Author: "codex", Agent: true})
	if err != nil || !strings.HasSuffix(e.Text, "(draft)") || len(w) == 0 {
		t.Fatal(e, w, err)
	}
	if _, _, err := Add(root, "gotchas", "token is ghp_abcdefghijklmnopqrstuvwxyz0123456789", AddOptions{}); err == nil {
		t.Fatal("secret accepted")
	}
	if _, _, err := Add(root, "../x", "y", AddOptions{}); err == nil {
		t.Fatal("path traversal accepted")
	}
}
