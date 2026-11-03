package dashboard

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// MaxSkinBytes caps a skin folder, assets included.
const MaxSkinBytes = 2 << 20

var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)

// ValidName reports whether s can name a skin.
func ValidName(s string) bool { return nameRe.MatchString(s) && s != "isekai" && s != "corporate" }

// NewSkin copies a built-in look into .isekai/ui/skins/<name>/.
func NewSkin(root, name, from string) (string, error) {
	if !ValidName(name) {
		return "", fmt.Errorf("skin name %q: use lowercase letters, digits and dashes (not isekai or corporate)", name)
	}
	if from != "isekai" && from != "corporate" {
		return "", fmt.Errorf("--from must be isekai or corporate")
	}
	dir := filepath.Join(root, SkinsDir, name)
	if _, err := os.Stat(dir); err == nil {
		return "", fmt.Errorf("%s already exists", filepath.ToSlash(filepath.Join(SkinsDir, name)))
	}
	if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
		return "", err
	}
	h, _ := files.ReadFile("looks/" + from + "/index.html")
	c, _ := files.ReadFile("looks/" + from + "/skin.css")
	meta := fmt.Sprintf("name = %q\ndescription = \"\"   # one line: what this look is\nauthor = \"\"\nversion = 1\nbase = %q\n", name, from)
	for p, data := range map[string][]byte{"index.html": h, "skin.css": c, "skin.toml": []byte(meta), "assets/.keep": nil} {
		if err := os.WriteFile(filepath.Join(dir, p), data, 0o644); err != nil {
			return "", err
		}
	}
	return dir, nil
}

// Problem found by CheckSkin.
type Problem struct {
	Error bool
	Text  string
}

var requiredSlots = []struct{ re, what string }{
	{`data-bind="journal\.waiting_for_humans"`, `questions waiting for humans (data-bind="journal.waiting_for_humans")`},
	{`data-each="open"`, `the open work list (<template data-each="open">)`},
	{`journal\.why_pct`, `the share of commits that explain why (journal.why_pct)`},
	{`<isekai-look-button`, `the Change look button (<isekai-look-button>)`},
}

var bindAttrRe = regexp.MustCompile(`data-(?:bind|each|if|name-from|bind-state)="!?([^"]*)"|data-bind-(?:var|attr)="([^"]*)"`)

// CheckSkin validates a skin against docs/skins.md. data is the current
// `isekai stats` JSON, used to catch bindings to fields that do not exist.
func CheckSkin(dir string, data []byte) ([]Problem, error) {
	var ps []Problem
	errf := func(f string, a ...any) { ps = append(ps, Problem{true, fmt.Sprintf(f, a...)}) }
	warnf := func(f string, a ...any) { ps = append(ps, Problem{false, fmt.Sprintf(f, a...)}) }

	total := int64(0)
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			errf("%s is a symlink; skins hold plain files only", rel(dir, p))
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		if strings.HasPrefix(rel(dir, p), "assets/") && !allowedAsset(p) && filepath.Base(p) != ".keep" {
			errf("%s: assets may be woff2, woff, ttf, otf, svg, png, jpg, webp or gif", rel(dir, p))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if total > MaxSkinBytes {
		errf("skin is %d KB; the limit is %d KB", total/1024, MaxSkinBytes/1024)
	}
	h, err := readRegular(filepath.Join(dir, "index.html"))
	if errors.Is(err, fs.ErrNotExist) {
		errf("index.html is missing")
		return ps, nil
	} else if err != nil {
		return nil, err
	}
	c, _ := readRegular(filepath.Join(dir, "skin.css"))
	var svgs []string
	for _, svg := range listAssets(dir, ".svg") {
		b, _ := readRegular(svg)
		svgs = append(svgs, string(b))
	}
	if bad := unsafeSkin(string(h), string(c), svgs); bad != "" {
		errf("%s is not allowed (no scripts, no event attributes, no forms or redirects, no network)", bad)
	}
	for _, s := range requiredSlots {
		if !regexp.MustCompile(s.re).MatchString(string(h)) {
			errf("required slot missing: %s", s.what)
		}
	}
	// Bindings must point at real fields.
	var doc any
	if len(data) > 0 && json.Unmarshal(data, &doc) == nil {
		seen := map[string]bool{}
		for _, m := range bindAttrRe.FindAllStringSubmatch(string(h), -1) {
			var paths []string
			if m[1] != "" {
				paths = []string{m[1]}
			} else {
				for _, kv := range strings.Split(m[2], ";") {
					if i := strings.Index(kv, ":"); i >= 0 {
						paths = append(paths, strings.TrimSpace(kv[i+1:]))
					}
				}
			}
			for _, p := range paths {
				if p == "" || strings.HasPrefix(p, ".") || seen[p] {
					continue
				}
				seen[p] = true
				if !pathExists(doc, p) {
					warnf("binding %q matches no field in isekai stats --format json (typo?)", p)
				}
			}
		}
	}
	warnf("contrast and the 390 px layout cannot be checked here: look at the page (isekai dashboard --look %s --open) at 1280 and 390 px", filepath.Base(dir))
	return ps, nil
}

func pathExists(doc any, path string) bool {
	v := doc
	for _, part := range strings.Split(path, ".") {
		switch x := v.(type) {
		case map[string]any:
			n, ok := x[part]
			if !ok {
				return false
			}
			v = n
		case []any:
			var i int
			if _, err := fmt.Sscanf(part, "%d", &i); err != nil || i < 0 || i >= len(x) {
				return len(x) == 0 // an empty list cannot be checked
			}
			v = x[i]
		default:
			return false
		}
	}
	return true
}

func rel(base, p string) string {
	r, err := filepath.Rel(base, p)
	if err != nil {
		return p
	}
	return filepath.ToSlash(r)
}

var assetExts = map[string]string{".woff2": "font/woff2", ".woff": "font/woff", ".ttf": "font/ttf", ".otf": "font/otf", ".svg": "image/svg+xml", ".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".webp": "image/webp", ".gif": "image/gif"}

func allowedAsset(p string) bool { _, ok := assetExts[strings.ToLower(filepath.Ext(p))]; return ok }

func listAssets(dir, ext string) []string {
	var out []string
	filepath.WalkDir(filepath.Join(dir, "assets"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() && (ext == "" || strings.EqualFold(filepath.Ext(p), ext)) {
			out = append(out, p)
		}
		return nil
	})
	sort.Strings(out)
	return out
}

// inlineAssets replaces references to assets/<file> with data: URIs, so
// the dashboard stays one file with no network access.
func inlineAssets(dir, s string) string {
	for _, p := range listAssets(dir, "") {
		if !allowedAsset(p) {
			continue
		}
		data, err := readRegular(p)
		if err != nil {
			continue
		}
		typ := assetExts[strings.ToLower(filepath.Ext(p))]
		if typ == "" {
			typ = mime.TypeByExtension(filepath.Ext(p))
		}
		uri := "data:" + typ + ";base64," + base64.StdEncoding.EncodeToString(data)
		r := rel(dir, p)
		for _, ref := range []string{"./" + r, r} {
			s = strings.ReplaceAll(s, `"`+ref+`"`, `"`+uri+`"`)
			s = strings.ReplaceAll(s, `'`+ref+`'`, `'`+uri+`'`)
			s = strings.ReplaceAll(s, `(`+ref+`)`, `(`+uri+`)`)
		}
	}
	return s
}

// SetLook writes [ui] look = "<name>" into .isekai/config.toml, keeping the
// rest of the file (comments included) as it is. The result is decoded
// before it is written, so a config isekai cannot edit safely is left alone.
func SetLook(configPath, name string) error {
	if name != "isekai" && name != "corporate" && !ValidName(name) {
		return fmt.Errorf("look %q: not a valid skin name", name)
	}
	data, err := os.ReadFile(configPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	var before map[string]any
	if _, err := toml.Decode(string(data), &before); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(configPath), err)
	}
	line := `look = "` + name + `"`
	lines := strings.Split(string(data), "\n")
	ui, end := -1, len(lines)
	for i, l := range lines {
		h := tableRe.FindStringSubmatch(l)
		if h == nil {
			continue
		}
		if ui >= 0 {
			end = i
			break
		}
		if h[1] == "ui" {
			ui = i
		}
	}
	switch {
	case ui < 0:
		s := strings.TrimRight(string(data), "\n")
		if s != "" {
			s += "\n\n"
		}
		lines = strings.Split(s+"[ui]\n"+line+"\n", "\n")
	default:
		done := false
		for i := ui + 1; i < end; i++ {
			if lookRe.MatchString(lines[i]) {
				lines[i] = lookRe.ReplaceAllString(lines[i], "${1}"+line+"${2}")
				done = true
				break
			}
		}
		if !done {
			lines = append(lines[:ui+1], append([]string{line}, lines[ui+1:]...)...)
		}
	}
	out := strings.Join(lines, "\n")
	var after struct {
		UI struct {
			Look string `toml:"look"`
		} `toml:"ui"`
	}
	if _, err := toml.Decode(out, &after); err != nil || after.UI.Look != name {
		return fmt.Errorf("%s: could not set [ui] look safely; set look = %q under [ui] by hand", filepath.Base(configPath), name)
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(configPath, []byte(out), 0o644)
}

// A table header such as [ui], [ ui ] or [ui]  # comment.
var tableRe = regexp.MustCompile(`^\s*\[\s*([A-Za-z0-9_.-]+)\s*\]\s*(?:#.*)?$`)

// A look = "..." line, keeping indentation and a trailing comment.
var lookRe = regexp.MustCompile(`^(\s*)look\s*=\s*(?:"[^"]*"|'[^']*')(\s*(?:#.*)?)$`)
