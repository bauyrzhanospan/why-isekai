package secrets

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// SopsStore reads a SOPS-encrypted file through the sops binary. The file is
// a map of scopes to maps of names: {prod: {db_password: …}}.
type SopsStore struct {
	Root  string
	File  string
	cache map[string]string
}

func (s *SopsStore) decrypt() (map[string]string, error) {
	if s.cache != nil {
		return s.cache, nil
	}
	bin, err := exec.LookPath("sops")
	if err != nil {
		return nil, fmt.Errorf("%w: sops is not on PATH (install: brew install sops | https://github.com/getsops/sops/releases) — or switch [store] type to \"age\", which is built in", ErrStoreMissing)
	}
	p := s.File
	if !filepath.IsAbs(p) {
		p = filepath.Join(s.Root, p)
	}
	cmd := exec.Command(bin, "--decrypt", "--output-type", "json", p)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if strings.Contains(strings.ToLower(msg), "no such file") {
			return nil, fmt.Errorf("%w: %s does not exist", ErrNotFound, s.File)
		}
		// sops errors do not include plaintext; still keep only the first line.
		first, _, _ := strings.Cut(msg, "\n")
		return nil, fmt.Errorf("%w: sops could not decrypt %s: %s", ErrNoKey, s.File, first)
	}
	var raw map[string]any
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
		return nil, errors.New("sops output is not a JSON object")
	}
	m := map[string]string{}
	for scope, v := range raw {
		if scope == "sops" {
			continue
		}
		inner, ok := v.(map[string]any)
		if !ok {
			continue
		}
		for name, val := range inner {
			switch x := val.(type) {
			case string:
				m[scope+"/"+name] = x
			default:
				b, _ := json.Marshal(x)
				m[scope+"/"+name] = string(b)
			}
		}
	}
	s.cache = m
	return m, nil
}

// Names implements Store. sops needs a key even to list.
func (s *SopsStore) Names() ([]string, error) {
	m, err := s.decrypt()
	if err != nil {
		return nil, err
	}
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

// Get implements Store.
func (s *SopsStore) Get(key string) (string, error) {
	m, err := s.decrypt()
	if err != nil {
		return "", err
	}
	v, ok := m[key]
	if !ok {
		return "", fmt.Errorf("%w: %s%s is not in %s (add it with: sops edit %s)", ErrNotFound, RefPrefix, key, s.File, s.File)
	}
	return v, nil
}
