package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAgentEnv(t *testing.T) {
	a := Default().Agents
	cases := []struct {
		env  map[string]string
		name string
		ok   bool
	}{
		{map[string]string{"AI_AGENT": "claude-code_2-1_agent", "CLAUDECODE": "1"}, "claude-code_2-1_agent", true},
		{map[string]string{"CURSOR_AGENT": "1"}, "cursor", true},
		{map[string]string{"CODEX_THREAD_ID": "abc123"}, "codex", true},
		{map[string]string{"ISEKAI_AGENT": "my-bot"}, "my-bot", true},
		{map[string]string{"AGENT": "amp"}, "amp", true},
		{map[string]string{"AGENT": "jenkins"}, "", false},
		{map[string]string{"CLAUDECODE": "0"}, "", false},
		{map[string]string{"GEMINI_CLI": "false"}, "", false},
		{nil, "", false},
	}
	for _, c := range cases {
		k, v, ok := a.MatchEnv(func(s string) string { return c.env[s] })
		if ok != c.ok || (ok && AgentName(k, v) != c.name) {
			t.Errorf("%v: got %q/%q ok=%v name=%q", c.env, k, v, ok, AgentName(k, v))
		}
	}
}

func TestLoadTemplateRoundTrip(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, Dir), 0o755)
	os.WriteFile(filepath.Join(root, File), []byte(Template(ModeBlock)), 0o644)
	t.Setenv("ISEKAI_MODE", "")
	c, err := Load(root)
	if err != nil || c.Log.Mode != ModeBlock || len(c.Agents.Env) != len(DefaultAgentEnv) || c.Log.MinWhyLen != 15 {
		t.Fatalf("%+v %v", c, err)
	}
	t.Setenv("ISEKAI_MODE", "warn")
	if c, _ := Load(root); c.Log.Mode != ModeWarn {
		t.Fatal("ISEKAI_MODE override")
	}
	t.Setenv("ISEKAI_MODE", "nope")
	if _, err := Load(root); err == nil {
		t.Fatal("bad mode accepted")
	}
}
