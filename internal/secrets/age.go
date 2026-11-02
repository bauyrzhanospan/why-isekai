package secrets

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"filippo.io/age"
)

// AgeStore keeps a JSON object {"scope/name": base64(age ciphertext)}.
// Names stay readable (references are not secret, spec §2.1.3) so listing
// needs no key and adding a secret needs only the public recipients.
type AgeStore struct {
	Root       string
	File       string
	Recipients string
}

func (s *AgeStore) path(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(s.Root, p)
}

func (s *AgeStore) load() (map[string]string, error) {
	data, err := os.ReadFile(s.path(s.File))
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	if len(bytes.TrimSpace(data)) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", s.File, err)
	}
	return m, nil
}

func (s *AgeStore) save(m map[string]string) error {
	out, err := json.MarshalIndent(m, "", "  ") // map keys are sorted: stable diffs
	if err != nil {
		return err
	}
	p := s.path(s.File)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, append(out, '\n'), 0o644)
}

// Names implements Store.
func (s *AgeStore) Names() ([]string, error) {
	m, err := s.load()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	return names, nil
}

// Get implements Store.
func (s *AgeStore) Get(key string) (string, error) {
	m, err := s.load()
	if err != nil {
		return "", err
	}
	ct, ok := m[key]
	if !ok {
		return "", fmt.Errorf("%w: %s%s is not in %s (add it: isekai secrets set %s%s)", ErrNotFound, RefPrefix, key, s.File, RefPrefix, key)
	}
	ids, src, err := Identities()
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(ct)
	if err != nil {
		return "", fmt.Errorf("%s: value of %s is not base64", s.File, key)
	}
	r, err := age.Decrypt(bytes.NewReader(raw), ids...)
	if err != nil {
		return "", fmt.Errorf("%w: key from %s cannot decrypt %s%s (is your public key in %s? then run: isekai secrets rekey)", ErrNoKey, src, RefPrefix, key, s.Recipients)
	}
	v, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(v), nil
}

// Set encrypts value to the project recipients. No private key needed.
func (s *AgeStore) Set(key, value string) error {
	recips, err := s.recipients()
	if err != nil {
		return err
	}
	ct, err := encrypt(value, recips)
	if err != nil {
		return err
	}
	m, err := s.load()
	if err != nil {
		return err
	}
	m[key] = ct
	return s.save(m)
}

// Remove deletes a key.
func (s *AgeStore) Remove(key string) error {
	m, err := s.load()
	if err != nil {
		return err
	}
	if _, ok := m[key]; !ok {
		return fmt.Errorf("%w: %s%s", ErrNotFound, RefPrefix, key)
	}
	delete(m, key)
	return s.save(m)
}

// Rekey re-encrypts every value to the current recipients (after adding or
// removing a teammate or CI key). Needs a key that can decrypt everything.
func (s *AgeStore) Rekey() (int, error) {
	recips, err := s.recipients()
	if err != nil {
		return 0, err
	}
	m, err := s.load()
	if err != nil {
		return 0, err
	}
	for k := range m {
		v, err := s.Get(k)
		if err != nil {
			return 0, err
		}
		if m[k], err = encrypt(v, recips); err != nil {
			return 0, err
		}
	}
	return len(m), s.save(m)
}

func encrypt(value string, recips []age.Recipient) (string, error) {
	var buf bytes.Buffer
	w, err := age.Encrypt(&buf, recips...)
	if err != nil {
		return "", err
	}
	if _, err := io.WriteString(w, value); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func (s *AgeStore) recipients() ([]age.Recipient, error) {
	f, err := os.Open(s.path(s.Recipients))
	if err != nil {
		return nil, fmt.Errorf("no recipients file %s (run: isekai secrets init)", s.Recipients)
	}
	defer f.Close()
	var out []age.Recipient
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		r, err := age.ParseX25519Recipient(strings.Fields(l)[0])
		if err != nil {
			return nil, fmt.Errorf("%s: %v", s.Recipients, err)
		}
		out = append(out, r)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s has no age1… public keys (run: isekai secrets init)", s.Recipients)
	}
	return out, sc.Err()
}

// AddRecipient appends a public key to the recipients file if missing.
func (s *AgeStore) AddRecipient(pub, comment string) (bool, error) {
	p := s.path(s.Recipients)
	data, _ := os.ReadFile(p)
	if strings.Contains(string(data), pub) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return false, err
	}
	txt := string(data)
	if txt == "" {
		txt = "# age public keys that can decrypt .isekai secrets. One per line. After editing run: isekai secrets rekey\n"
	} else if !strings.HasSuffix(txt, "\n") {
		txt += "\n"
	}
	txt += "# " + comment + "\n" + pub + "\n"
	return true, os.WriteFile(p, []byte(txt), 0o644)
}

// KeyFile is where `isekai secrets init` stores a generated private key.
func KeyFile() string {
	if p := os.Getenv("ISEKAI_AGE_KEY_FILE"); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(dir, "isekai", "age.key")
}

// Identities finds private keys, in order: ISEKAI_AGE_KEY (key text),
// ISEKAI_AGE_KEY_FILE, SOPS_AGE_KEY, SOPS_AGE_KEY_FILE, the isekai key file,
// the sops default key file. Keys never live in the repository (§2.2.3).
func Identities() ([]age.Identity, string, error) {
	try := func(text, src string) ([]age.Identity, string, error) {
		ids, err := age.ParseIdentities(strings.NewReader(text))
		if err != nil {
			return nil, src, fmt.Errorf("%w: %s: %v", ErrNoKey, src, err)
		}
		return ids, src, nil
	}
	if v := os.Getenv("ISEKAI_AGE_KEY"); v != "" {
		return try(v, "$ISEKAI_AGE_KEY")
	}
	if v := os.Getenv("SOPS_AGE_KEY"); v != "" {
		return try(v, "$SOPS_AGE_KEY")
	}
	files := []string{os.Getenv("ISEKAI_AGE_KEY_FILE"), os.Getenv("SOPS_AGE_KEY_FILE"), KeyFile()}
	if dir, err := os.UserConfigDir(); err == nil {
		files = append(files, filepath.Join(dir, "sops", "age", "keys.txt"))
	}
	for _, f := range files {
		if f == "" {
			continue
		}
		if data, err := os.ReadFile(f); err == nil {
			return try(string(data), f)
		}
	}
	return nil, "", fmt.Errorf("%w: set ISEKAI_AGE_KEY or ISEKAI_AGE_KEY_FILE, or create a key with: isekai secrets init (looked in %s)", ErrNoKey, KeyFile())
}

// GenerateKey creates a private key at KeyFile() unless one exists, and
// returns its public key.
func GenerateKey() (pub string, created bool, err error) {
	p := KeyFile()
	if data, err := os.ReadFile(p); err == nil {
		ids, err := age.ParseX25519Identity(firstKeyLine(string(data)))
		if err != nil {
			return "", false, fmt.Errorf("%s: %v", p, err)
		}
		return ids.Recipient().String(), false, nil
	}
	id, err := age.GenerateX25519Identity()
	if err != nil {
		return "", false, err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return "", false, err
	}
	content := "# isekai age key — keep private, back it up, never commit it\n# public key: " + id.Recipient().String() + "\n" + id.String() + "\n"
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		return "", false, err
	}
	return id.Recipient().String(), true, nil
}

func firstKeyLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "AGE-SECRET-KEY-") {
			return l
		}
	}
	return ""
}
