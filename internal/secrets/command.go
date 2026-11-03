package secrets

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// CommandStore reads secrets through an existing secrets manager's CLI
// (spec §2.2.1). Templates are argv lists, never shell strings, and scope
// and name are validated to [A-Za-z0-9_-] before substitution, so a
// reference cannot inject arguments.
type CommandStore struct {
	Root    string
	Tool    string
	Scopes  []string
	GetCmd  []string
	ListCmd []string
	Hint    string // how to install the tool
}

type preset struct {
	get, list []string
	hint      string
}

// Presets for well-known managers. They follow each tool's documented CLI;
// override get/list in .isekai/secrets.toml if your setup differs.
var presets = map[string]preset{
	"dotenvx": {
		get:  []string{"dotenvx", "get", "{NAME}", "-f", ".env.{scope}"},
		list: []string{"dotenvx", "get", "-f", ".env.{scope}"},
		hint: "npm i -g @dotenvx/dotenvx (https://dotenvx.com)",
	},
	"1password": {
		get:  []string{"op", "read", "op://{scope}/{name}/password"},
		list: []string{"op", "item", "list", "--vault", "{scope}", "--format", "json"},
		hint: "https://developer.1password.com/docs/cli/get-started/ (then op signin)",
	},
	"vault": {
		get:  []string{"vault", "kv", "get", "-field=value", "secret/{scope}/{name}"},
		list: []string{"vault", "kv", "list", "-format=json", "secret/{scope}"},
		hint: "https://developer.hashicorp.com/vault/install (then vault login)",
	},
	"infisical": {
		get:  []string{"infisical", "secrets", "get", "{NAME}", "--env={scope}", "--plain", "--silent"},
		list: []string{"infisical", "export", "--env={scope}", "--format=json"},
		hint: "https://infisical.com/docs/cli/overview (then infisical login)",
	},
}

// NewCommandStore builds a store from a preset or from explicit templates.
func NewCommandStore(root string, c Config) (Store, error) {
	t := c.Store.Type
	p, known := presets[t]
	if !known && t != "command" {
		return nil, fmt.Errorf("%s: store type %q is not supported (supported: age, sops, dotenvx, 1password, vault, infisical, command)", ConfigFile, t)
	}
	s := &CommandStore{Root: root, Tool: t, Scopes: c.Store.Scopes, GetCmd: p.get, ListCmd: p.list, Hint: p.hint}
	if len(c.Store.Get) > 0 {
		s.GetCmd = c.Store.Get
	}
	if len(c.Store.List) > 0 {
		s.ListCmd = c.Store.List
	}
	if len(s.GetCmd) == 0 {
		return nil, fmt.Errorf("%s: type \"command\" needs [store] get = [\"tool\", \"arg\", \"{scope}\", \"{name}\"]", ConfigFile)
	}
	return s, nil
}

func expand(tmpl []string, scope, name string) []string {
	out := make([]string, len(tmpl))
	for i, a := range tmpl {
		a = strings.ReplaceAll(a, "{scope}", scope)
		a = strings.ReplaceAll(a, "{name}", name)
		a = strings.ReplaceAll(a, "{NAME}", EnvName(name))
		out[i] = a
	}
	return out
}

func (s *CommandStore) run(argv []string) ([]byte, error) {
	bin, err := exec.LookPath(argv[0])
	if err != nil {
		hint := s.Hint
		if hint == "" {
			hint = "install it and make sure it is on PATH"
		}
		return nil, fmt.Errorf("%w: %s is not on PATH (%s)", ErrStoreMissing, argv[0], hint)
	}
	cmd := exec.Command(bin, argv[1:]...)
	cmd.Dir = s.Root
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		// Only the tool's first error line; never stdout, which may hold a value.
		first, _, _ := strings.Cut(strings.TrimSpace(errb.String()), "\n")
		low := strings.ToLower(first)
		switch {
		case strings.Contains(low, "not found") || strings.Contains(low, "no value") || strings.Contains(low, "does not exist") || strings.Contains(low, "isn't an item"):
			return nil, fmt.Errorf("%w: %s says: %s", ErrNotFound, s.Tool, first)
		case strings.Contains(low, "permission") || strings.Contains(low, "forbidden") || strings.Contains(low, "denied"):
			return nil, fmt.Errorf("%w by %s: %s", ErrDenied, s.Tool, first)
		default:
			return nil, fmt.Errorf("%w: %s failed: %s (are you logged in?)", ErrNoKey, s.Tool, first)
		}
	}
	return out.Bytes(), nil
}

// Get implements Store.
func (s *CommandStore) Get(key string) (string, error) {
	r, err := ParseRef(key)
	if err != nil {
		return "", err
	}
	out, err := s.run(expand(s.GetCmd, r.Scope, r.Name))
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}

// Names implements Store: lists names per configured scope.
func (s *CommandStore) Names() ([]string, error) {
	if len(s.ListCmd) == 0 || len(s.Scopes) == 0 {
		return nil, nil // the tool cannot list, or scopes are not configured
	}
	var names []string
	for _, scope := range s.Scopes {
		if !partRe.MatchString(scope) {
			return nil, fmt.Errorf("%s: bad scope %q", ConfigFile, scope)
		}
		out, err := s.run(expand(s.ListCmd, scope, ""))
		if err != nil {
			return nil, err
		}
		for _, n := range parseNames(out) {
			n = strings.ToLower(strings.ReplaceAll(n, " ", "_"))
			if partRe.MatchString(n) {
				names = append(names, scope+"/"+n)
			}
		}
	}
	sort.Strings(names)
	return names, nil
}

// parseNames accepts the shapes these CLIs print: a JSON array of strings,
// an array of objects with title/key/name, or an object whose keys are
// names; otherwise one name per line or KEY=value lines.
func parseNames(out []byte) []string {
	var arr []any
	if json.Unmarshal(out, &arr) == nil {
		var ns []string
		for _, v := range arr {
			switch x := v.(type) {
			case string:
				ns = append(ns, strings.TrimSuffix(x, "/"))
			case map[string]any:
				for _, k := range []string{"title", "key", "name", "secretKey"} {
					if s, ok := x[k].(string); ok {
						ns = append(ns, s)
						break
					}
				}
			}
		}
		return ns
	}
	var obj map[string]any
	if json.Unmarshal(out, &obj) == nil {
		var ns []string
		for k := range obj {
			ns = append(ns, k)
		}
		return ns
	}
	var ns []string
	for _, l := range strings.Split(string(out), "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		k, _, _ := strings.Cut(l, "=")
		ns = append(ns, strings.TrimSpace(k))
	}
	return ns
}
