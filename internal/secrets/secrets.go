// Package secrets implements Module 2 of the spec: agents use secrets by
// reference (secret://scope/name) and never see their values.
//
// IsekAI has no cryptography of its own. The default store encrypts each
// value with age (filippo.io/age, the reference implementation of
// age-encryption.org/v1); the sops store shells out to the sops binary.
package secrets

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// ConfigFile describes where secrets live and who may use them. No values.
const ConfigFile = ".isekai/secrets.toml"

// RefPrefix starts every secret reference.
const RefPrefix = "secret://"

var partRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Ref is a parsed secret reference.
type Ref struct{ Scope, Name string }

func (r Ref) String() string { return RefPrefix + r.Scope + "/" + r.Name }

// Key is "scope/name", the form used in access rules and store files.
func (r Ref) Key() string { return r.Scope + "/" + r.Name }

// ParseRef accepts "secret://scope/name" or "scope/name".
func ParseRef(s string) (Ref, error) {
	t := strings.TrimPrefix(strings.TrimSpace(s), RefPrefix)
	scope, name, ok := strings.Cut(t, "/")
	if !ok || !partRe.MatchString(scope) || !partRe.MatchString(name) {
		return Ref{}, fmt.Errorf("bad secret reference %q: want secret://<scope>/<name>, letters, digits, _ and - only (e.g. secret://dev/db_password)", s)
	}
	return Ref{scope, name}, nil
}

// Config is .isekai/secrets.toml.
type Config struct {
	Store struct {
		Type       string `toml:"type"` // age | sops | dotenvx | 1password | vault | infisical | command
		File       string `toml:"file"`
		Recipients string `toml:"recipients"` // age only
		// For command-based stores: the scopes that exist, and optional
		// overrides of the preset command templates ({scope}, {name}, {NAME}).
		Scopes []string `toml:"scopes"`
		Get    []string `toml:"get"`
		List   []string `toml:"list"`
	} `toml:"store"`
	// Access maps "human", "agent" or "agent:<name>" to glob patterns over
	// scope/name. A pattern starting with "!" excludes.
	Access map[string][]string `toml:"access"`
}

// Defaults for a project that has not configured anything.
const (
	DefaultAgeFile        = ".isekai/secrets.age.json"
	DefaultRecipientsFile = ".isekai/age-recipients.txt"
)

// LoadConfig reads the secrets config; ok is false when the project does not
// use the secrets module.
func LoadConfig(root string) (Config, bool, error) {
	var c Config
	data, err := os.ReadFile(filepath.Join(root, ConfigFile))
	if errors.Is(err, fs.ErrNotExist) {
		return c, false, nil
	}
	if err != nil {
		return c, false, err
	}
	if _, err := toml.Decode(string(data), &c); err != nil {
		return c, false, fmt.Errorf("%s: %w", ConfigFile, err)
	}
	if c.Store.Type == "" {
		c.Store.Type = "age"
	}
	if c.Store.Type == "age" {
		if c.Store.File == "" {
			c.Store.File = DefaultAgeFile
		}
		if c.Store.Recipients == "" {
			c.Store.Recipients = DefaultRecipientsFile
		}
	}
	if c.Store.File == "" && (c.Store.Type == "age" || c.Store.Type == "sops") {
		return c, true, fmt.Errorf("%s: [store] file is required for type %q", ConfigFile, c.Store.Type)
	}
	if c.Access == nil {
		c.Access = map[string][]string{}
	}
	if _, ok := c.Access["human"]; !ok {
		c.Access["human"] = []string{"*/*"}
	}
	if _, ok := c.Access["agent"]; !ok {
		c.Access["agent"] = []string{"*/*", "!prod/*"}
	}
	return c, true, nil
}

// ConfigTemplate is written by `isekai secrets init`.
const ConfigTemplate = `# why-isekai secrets: where secrets live and who may use them. No values here.
# Spec: https://github.com/bauyrzhanospan/why-isekai/blob/main/SPEC.md#module-2-secrets
#
# IMPORTANT: access rules are a seatbelt against mistakes, NOT a security
# boundary. An agent says who it is and can lie. The real boundary is which
# decryption keys exist in the environment the agent runs in.

[store]
type = "age"                                # age (built in) | sops | dotenvx | 1password | vault | infisical | command
file = ".isekai/secrets.age.json"           # names in clear text, each value age-encrypted
recipients = ".isekai/age-recipients.txt"   # public keys that can decrypt (one per line)

[access]
human = ["*/*"]
agent = ["*/*", "!prod/*"]                  # agents get everything except prod
# "agent:deploy-bot" = ["prod/deploy_key"]  # extra grants for a named agent
`

// Allowed reports whether caller may use ref. agentName is "" for a human.
// A named grant ("agent:<name>") wins over generic agent rules; otherwise a
// key is allowed when some pattern matches and no "!" pattern does.
func (c Config) Allowed(ref Ref, agentName string) bool {
	key := ref.Key()
	if agentName != "" {
		named := c.Access["agent:"+agentName]
		if excluded(named, key) {
			return false
		}
		if match(named, key) {
			return true
		}
		return match(c.Access["agent"], key)
	}
	return match(c.Access["human"], key)
}

func excluded(rules []string, key string) bool {
	for _, p := range rules {
		if strings.HasPrefix(p, "!") {
			if ok, _ := path.Match(strings.TrimPrefix(p, "!"), key); ok {
				return true
			}
		}
	}
	return false
}

func match(rules []string, key string) bool {
	allowed := false
	for _, p := range rules {
		neg := strings.HasPrefix(p, "!")
		p = strings.TrimPrefix(p, "!")
		if p == "*" {
			p = "*/*"
		}
		if ok, _ := path.Match(p, key); ok {
			if neg {
				return false
			}
			allowed = true
		}
	}
	return allowed
}

// Store reads (and for age, writes) secret values.
type Store interface {
	// Names lists scope/name keys without decrypting anything, if possible.
	Names() ([]string, error)
	// Get decrypts one value.
	Get(key string) (string, error)
}

// Error kinds required by spec §2.5.1. Messages never contain values.
var (
	ErrNotFound     = errors.New("secret not found")
	ErrDenied       = errors.New("access denied by .isekai/secrets.toml rules")
	ErrStoreMissing = errors.New("secret store tool not installed")
	ErrNoKey        = errors.New("no decryption key available")
)

// Open returns the configured store.
func Open(root string, c Config) (Store, error) {
	switch c.Store.Type {
	case "age":
		return &AgeStore{Root: root, File: c.Store.File, Recipients: c.Store.Recipients}, nil
	case "sops":
		return &SopsStore{Root: root, File: c.Store.File}, nil
	default:
		return NewCommandStore(root, c)
	}
}

// AgentVisibleRefs lists refs the given caller may use, for `isekai brief`.
// It returns nil when the secrets module is not configured.
func AgentVisibleRefs(root, agentName string) ([]string, error) {
	c, ok, err := LoadConfig(root)
	if err != nil || !ok {
		return nil, err
	}
	st, err := Open(root, c)
	if err != nil {
		return nil, err
	}
	names, err := st.Names()
	if err != nil {
		return nil, err
	}
	if agentName == "" {
		agentName = "agent" // the brief is read by agents
	}
	var out []string
	for _, n := range names {
		r, err := ParseRef(n)
		if err != nil {
			continue
		}
		if c.Allowed(r, agentName) {
			out = append(out, r.String())
		}
	}
	sort.Strings(out)
	return out, nil
}
