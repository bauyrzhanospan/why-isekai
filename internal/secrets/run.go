package secrets

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strings"
)

// RunOptions for Run.
type RunOptions struct {
	Root      string
	Scope     string // also expose every allowed secret in this scope as NAME
	AgentName string // "" for a human
	Env       []string
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
	Warn      io.Writer
}

// Resolve returns env with secret:// values replaced, plus the values used
// (keyed by reference) for masking.
func Resolve(o RunOptions) ([]string, map[string]string, error) {
	var refs []struct {
		i   int
		key string
		ref Ref
	}
	for i, kv := range o.Env {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || !strings.HasPrefix(v, RefPrefix) {
			continue
		}
		r, err := ParseRef(v)
		if err != nil {
			return nil, nil, fmt.Errorf("$%s: %w", k, err)
		}
		refs = append(refs, struct {
			i   int
			key string
			ref Ref
		}{i, k, r})
	}
	if len(refs) == 0 && o.Scope == "" {
		return o.Env, map[string]string{}, nil
	}
	c, ok, err := LoadConfig(o.Root)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return nil, nil, fmt.Errorf("%w: this project has no %s (set one up: isekai secrets init)", ErrStoreMissing, ConfigFile)
	}
	st, err := Open(o.Root, c)
	if err != nil {
		return nil, nil, err
	}
	used := map[string]string{}
	env := append([]string{}, o.Env...)
	get := func(r Ref) (string, error) {
		if !c.Allowed(r, o.AgentName) {
			who := "humans"
			if o.AgentName != "" {
				who = "agent " + o.AgentName
			}
			return "", fmt.Errorf("%w: %s is not available to %s (rules in %s; ask a human to run the command or grant it)", ErrDenied, r, who, ConfigFile)
		}
		v, err := st.Get(r.Key())
		if err != nil {
			return "", err
		}
		used[r.String()] = v
		return v, nil
	}
	for _, x := range refs {
		v, err := get(x.ref)
		if err != nil {
			return nil, nil, fmt.Errorf("$%s: %w", x.key, err)
		}
		env[x.i] = x.key + "=" + v
	}
	if o.Scope != "" {
		if !partRe.MatchString(o.Scope) {
			return nil, nil, fmt.Errorf("bad scope %q", o.Scope)
		}
		names, err := st.Names()
		if err != nil {
			return nil, nil, err
		}
		for _, n := range names {
			r, err := ParseRef(n)
			if err != nil || r.Scope != o.Scope || !c.Allowed(r, o.AgentName) {
				continue
			}
			v, err := get(r)
			if err != nil {
				return nil, nil, err
			}
			env = append(env, EnvName(r.Name)+"="+v)
		}
	}
	return env, used, nil
}

// EnvName turns a secret name into an environment variable name.
func EnvName(name string) string {
	return strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}

// Run executes argv with resolved secrets and masked output. It returns the
// child's exit code.
func Run(argv []string, o RunOptions) (int, error) {
	if len(argv) == 0 {
		return 2, errors.New("run: no command (usage: isekai run [--scope S] -- <command> [args…])")
	}
	env, used, err := Resolve(o)
	if err != nil {
		return 1, err
	}
	var short []string
	for ref, v := range used {
		if len(v) < MinMaskLen {
			short = append(short, ref)
		}
	}
	if len(short) > 0 && o.Warn != nil {
		sort.Strings(short)
		fmt.Fprintf(o.Warn, "isekai: warning: %s shorter than %d chars, cannot be masked in output\n", strings.Join(short, ", "), MinMaskLen)
	}
	m := NewMasker(used)
	out := m.Writer(o.Stdout)
	errw := m.Writer(o.Stderr)
	path, err := exec.LookPath(argv[0])
	if err != nil {
		return 127, fmt.Errorf("run: %s: command not found", argv[0])
	}
	cmd := exec.Command(path, argv[1:]...)
	cmd.Env = env
	cmd.Stdin = o.Stdin
	cmd.Stdout = out
	cmd.Stderr = errw
	// Let the child handle Ctrl-C; we just wait for it.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	defer signal.Stop(sig)
	runErr := cmd.Run()
	out.Close()
	errw.Close()
	if runErr != nil {
		var ee *exec.ExitError
		if errors.As(runErr, &ee) {
			code := ee.ExitCode()
			if code < 0 {
				code = 1
			}
			return code, nil
		}
		return 1, fmt.Errorf("run: %s", m.String(runErr.Error()))
	}
	return 0, nil
}
