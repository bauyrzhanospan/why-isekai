package secrets

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParseRef(t *testing.T) {
	r, err := ParseRef("secret://prod/db_password")
	if err != nil || r.Scope != "prod" || r.Name != "db_password" || r.String() != "secret://prod/db_password" {
		t.Fatal(r, err)
	}
	for _, bad := range []string{"secret://prod", "secret://pr od/x", "secret://a/b/c", "secret://a/ü"} {
		if _, err := ParseRef(bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestAccess(t *testing.T) {
	c := Config{Access: map[string][]string{
		"human":            {"*/*"},
		"agent":            {"*/*", "!prod/*"},
		"agent:deploy-bot": {"prod/deploy_key"},
	}}
	ref := func(s string) Ref { r, _ := ParseRef(s); return r }
	cases := []struct {
		ref, who string
		want     bool
	}{
		{"prod/db", "", true},
		{"prod/db", "claude", false},
		{"dev/db", "claude", true},
		{"prod/deploy_key", "deploy-bot", true},
		{"prod/db", "deploy-bot", false},
	}
	for _, tc := range cases {
		if got := c.Allowed(ref(tc.ref), tc.who); got != tc.want {
			t.Errorf("%s by %q = %v", tc.ref, tc.who, got)
		}
	}
}

func TestMasker(t *testing.T) {
	secret := "hunter2-very-secret"
	multi := "-----BEGIN KEY-----\nAAAABBBBCCCCDDDD\nEEEEFFFFGGGG\n-----END KEY-----"
	m := NewMasker(map[string]string{"secret://dev/pw": secret, "secret://dev/key": multi, "secret://dev/tiny": "ab"})
	in := "pw=" + secret + " b64=" + base64.StdEncoding.EncodeToString([]byte(secret)) +
		" url=" + base64.RawURLEncoding.EncodeToString([]byte(secret)) +
		"\nkey:\n" + multi + "\nline: EEEEFFFFGGGG\ncrlf: " + strings.ReplaceAll(multi, "\n", "\r\n") + "\nab stays"
	out := m.String(in)
	for _, leak := range []string{secret, "AAAABBBBCCCCDDDD", "EEEEFFFFGGGG", base64.StdEncoding.EncodeToString([]byte(secret))} {
		if strings.Contains(out, leak) {
			t.Errorf("leaked %q in %q", leak, out)
		}
	}
	if !strings.Contains(out, "[secret://dev/pw]") || !strings.Contains(out, "ab stays") {
		t.Errorf("out = %q", out)
	}
}

// A value split across writes must still be masked.
func TestMaskerStreaming(t *testing.T) {
	secret := "correct-horse-battery"
	m := NewMasker(map[string]string{"secret://x/y": secret})
	var b bytes.Buffer
	w := m.Writer(&b)
	data := "before " + secret + " after " + secret
	for i := 0; i < len(data); i++ {
		w.Write([]byte{data[i]})
	}
	w.Close()
	if strings.Contains(b.String(), secret) || b.String() != "before [secret://x/y] after [secret://x/y]" {
		t.Fatalf("got %q", b.String())
	}
}

func TestAgeStoreRoundTrip(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ISEKAI_AGE_KEY_FILE", filepath.Join(t.TempDir(), "age.key"))
	t.Setenv("ISEKAI_AGE_KEY", "")
	t.Setenv("SOPS_AGE_KEY", "")
	t.Setenv("SOPS_AGE_KEY_FILE", "")
	pub, created, err := GenerateKey()
	if err != nil || !created {
		t.Fatal(err)
	}
	st := &AgeStore{Root: root, File: DefaultAgeFile, Recipients: DefaultRecipientsFile}
	if _, err := st.AddRecipient(pub, "test"); err != nil {
		t.Fatal(err)
	}
	if err := st.Set("dev/db", "s3cr3t-value"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(root, DefaultAgeFile))
	if strings.Contains(string(raw), "s3cr3t") {
		t.Fatal("plaintext in store file")
	}
	if !strings.Contains(string(raw), `"dev/db"`) {
		t.Fatal("name should be readable")
	}
	v, err := st.Get("dev/db")
	if err != nil || v != "s3cr3t-value" {
		t.Fatal(v, err)
	}
	if _, err := st.Get("dev/nope"); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatal(err)
	}
	if n, err := st.Rekey(); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	// Without a key, decryption fails with ErrNoKey and no value leaks.
	t.Setenv("ISEKAI_AGE_KEY_FILE", filepath.Join(t.TempDir(), "missing.key"))
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AppData", t.TempDir())
	if _, err := st.Get("dev/db"); err == nil || !strings.Contains(err.Error(), "no decryption key") {
		t.Fatalf("err = %v", err)
	}
}

func TestMaskerIdleFlushAndEncodings(t *testing.T) {
	m := NewMasker(map[string]string{"secret://x/y": `nopq"rstuvw`})
	var mu sync.Mutex
	var b bytes.Buffer
	w := m.Writer(writerFunc(func(p []byte) (int, error) { mu.Lock(); defer mu.Unlock(); return b.Write(p) }))
	w.Write([]byte("Continue? [y/n"))
	time.Sleep(4 * IdleFlush)
	mu.Lock()
	got := b.String()
	mu.Unlock()
	if got != "Continue? [y/n" {
		t.Fatalf("held tail not flushed: %q", got)
	}
	w.Write([]byte(` cfg={"pw":"nopq\"rstuvw"}`))
	w.Close()
	if strings.Contains(b.String(), "rstuvw") {
		t.Fatalf("json-escaped value leaked: %q", b.String())
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

func TestNamedAgentExclusion(t *testing.T) {
	c := Config{Access: map[string][]string{"agent": {"*/*"}, "agent:bot": {"!dev/x"}}}
	if c.Allowed(Ref{"dev", "x"}, "bot") || !c.Allowed(Ref{"dev", "y"}, "bot") || !c.Allowed(Ref{"dev", "x"}, "other") {
		t.Fatal("named exclusion")
	}
}

func TestCommandStorePresetWithFakeCLI(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake CLI is a shell script")
	}
	bin := t.TempDir()
	// A fake `op`: read prints a value; item list prints JSON titles; unknown items fail.
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = read ]; then case \"$2\" in op://dev/db_password/password) printf 'pa55-op-value\\n';; *) echo \"[ERROR] \\\"$2\\\" isn't an item\" >&2; exit 1;; esac; fi\n" +
		"if [ \"$1\" = item ]; then echo '[{\"title\":\"db_password\"},{\"title\":\"api key\"}]'; fi\n"
	os.WriteFile(filepath.Join(bin, "op"), []byte(script), 0o755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	var c Config
	c.Store.Type = "1password"
	c.Store.Scopes = []string{"dev"}
	st, err := NewCommandStore(t.TempDir(), c)
	if err != nil {
		t.Fatal(err)
	}
	v, err := st.Get("dev/db_password")
	if err != nil || v != "pa55-op-value" {
		t.Fatalf("get: %q %v", v, err)
	}
	if _, err := st.Get("dev/nope"); err == nil || !strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "pa55") {
		t.Fatalf("missing: %v", err)
	}
	names, err := st.Names()
	if err != nil || strings.Join(names, ",") != "dev/api_key,dev/db_password" {
		t.Fatalf("names: %v %v", names, err)
	}
	if _, err := st.Get("dev/x; rm -rf /"); err == nil {
		t.Fatal("bad reference accepted")
	}
	c.Store.Type = "vault"
	st, _ = NewCommandStore(t.TempDir(), c)
	if _, err := st.Get("dev/db"); err == nil || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("missing tool: %v", err)
	}
}
