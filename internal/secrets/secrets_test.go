package secrets

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
