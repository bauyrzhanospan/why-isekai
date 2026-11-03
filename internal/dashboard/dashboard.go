// Package dashboard renders `isekai dashboard`: one self-contained HTML page
// with the stats data, the built-in looks and any skins in .isekai/ui/skins.
// The page loads nothing from the network (enforced by its CSP).
package dashboard

import (
	"embed"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/bauyrzhanospan/why-isekai/internal/stats"
)

//go:embed assets/* looks/*/*
var files embed.FS

// SkinsDir holds project skins (docs/skins.md).
const SkinsDir = ".isekai/ui/skins"

// Look is one renderable skin.
type Look struct {
	ID          string
	Name        string
	Description string
	HTML        string
	CSS         string
	BuiltIn     bool
}

var builtins = []struct{ id, name, desc string }{
	{"isekai", "Isekai", "Your agent as an anime hero: status window, gear, skills, quests."},
	{"corporate", "Corporate", "Plain dashboard for teams and reports."},
}

// Looks returns the built-in looks plus the project's skins.
func Looks(root string) ([]Look, []error) {
	var out []Look
	for _, b := range builtins {
		h, _ := files.ReadFile("looks/" + b.id + "/index.html")
		c, _ := files.ReadFile("looks/" + b.id + "/skin.css")
		out = append(out, Look{ID: b.id, Name: b.name, Description: b.desc, HTML: string(h), CSS: string(c), BuiltIn: true})
	}
	var errs []error
	entries, err := os.ReadDir(filepath.Join(root, SkinsDir))
	if err != nil {
		return out, nil
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		l, err := LoadSkin(filepath.Join(root, SkinsDir, e.Name()))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		out = append(out, l)
	}
	return out, errs
}

// LoadSkin reads one skin folder.
func LoadSkin(dir string) (Look, error) {
	id := filepath.Base(dir)
	var meta struct {
		Name        string `toml:"name"`
		Description string `toml:"description"`
	}
	if data, err := os.ReadFile(filepath.Join(dir, "skin.toml")); err == nil {
		if _, err := toml.Decode(string(data), &meta); err != nil {
			return Look{}, fmt.Errorf("skin %s: skin.toml: %w", id, err)
		}
	}
	h, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		return Look{}, fmt.Errorf("skin %s: %w", id, err)
	}
	c, _ := os.ReadFile(filepath.Join(dir, "skin.css"))
	if bad := unsafeMarkup(string(h) + string(c)); bad != "" {
		return Look{}, fmt.Errorf("skin %s: %s is not allowed in skins (see docs/skins.md); run isekai ui check %s", id, bad, id)
	}
	name := meta.Name
	if name == "" {
		name = id
	}
	return Look{ID: id, Name: name, Description: meta.Description, HTML: string(h), CSS: string(c)}, nil
}

var unsafeRes = []struct{ what, re string }{
	{"<script>", `(?i)<\s*script`},
	{"an on* event attribute", `(?i)\son[a-z]+\s*=`},
	{"a javascript: URL", `(?i)javascript:`},
	{"<iframe>, <object> or <embed>", `(?i)<\s*(iframe|object|embed)`},
	{"a network URL", `(?i)(https?:)?//[a-z0-9.-]+\.[a-z]{2,}`},
	{"@import", `(?i)@import`},
}

// unsafeMarkup returns what makes a skin unsafe, or "".
func unsafeMarkup(s string) string {
	for _, u := range unsafeRes {
		if regexp.MustCompile(u.re).MatchString(s) {
			return u.what
		}
	}
	return ""
}

// chrome styles the runtime's own UI (the Change look dialog) in any look.
const chrome = `
.isekai-looks { max-width: 640px; width: calc(100% - 32px); border: 1px solid #888; border-radius: 14px; padding: 20px; background: #fff; color: #17181C; font: 15px/1.45 Inter, system-ui, sans-serif; }
.isekai-looks::backdrop { background: rgba(0,0,0,0.55); }
.isekai-looks h2 { margin: 0 0 4px; font-size: 20px; }
.isekai-looks p { margin: 0 0 12px; color: #5B5E66; font-size: 14px; }
.isekai-looks-list { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 8px; margin-bottom: 16px; }
.isekai-looks-list button { font: inherit; text-align: left; display: flex; flex-direction: column; gap: 2px; padding: 12px; min-height: 44px; border-radius: 10px; border: 1px solid #D6D6D0; background: #fff; color: #17181C; cursor: pointer; }
.isekai-looks-list button[aria-pressed="true"] { border: 2px solid #3B47D6; }
.isekai-looks-list span { font-size: 13px; color: #5B5E66; }
.isekai-looks label { display: flex; flex-direction: column; gap: 6px; font-weight: 600; font-size: 14px; }
.isekai-looks textarea { font: inherit; font-weight: 400; padding: 10px; border-radius: 10px; border: 1px solid #D6D6D0; resize: vertical; }
.isekai-looks pre { white-space: pre-wrap; background: #F1F1EC; border-radius: 10px; padding: 12px; font: 12.5px/1.5 ui-monospace, Menlo, monospace; margin: 12px 0; }
.isekai-looks-actions { display: flex; gap: 8px; flex-wrap: wrap; }
.isekai-looks-actions button { font: inherit; font-weight: 600; min-height: 44px; padding: 0 16px; border-radius: 10px; border: 1px solid #D6D6D0; background: #fff; cursor: pointer; }
.isekai-looks-actions button:first-child { background: #3B47D6; border-color: #3B47D6; color: #fff; }
[hidden] { display: none !important; }
`

// Render writes the page. defaultLook is used unless the viewer picked one
// before in this browser; force ignores the viewer's earlier choice.
func Render(w io.Writer, s stats.Stats, looks []Look, defaultLook string, force bool) error {
	data, err := json.Marshal(s) // escapes <, > and & so it cannot close the script tag
	if err != nil {
		return err
	}
	runtime, _ := files.ReadFile("assets/runtime.js")
	hero, _ := files.ReadFile("assets/hero.svg")
	var b strings.Builder
	p := func(f string, a ...any) { fmt.Fprintf(&b, f, a...) }
	forceAttr := ""
	if force {
		forceAttr = " data-force-look"
	}
	p("<!doctype html>\n<html lang=\"en\" data-default-look=\"%s\"%s>\n<head>\n<meta charset=\"utf-8\">\n", html.EscapeString(defaultLook), forceAttr)
	p("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n")
	p("<meta http-equiv=\"Content-Security-Policy\" content=\"default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; img-src data:; font-src data:\">\n")
	p("<title>isekai · %s</title>\n<style>%s</style>\n</head>\n<body>\n<div id=\"app\"></div>\n", html.EscapeString(s.Repo.Name), chrome)
	p("<script type=\"application/json\" id=\"isekai-data\">%s</script>\n", data)
	p("<template id=\"isekai-hero-art\">%s</template>\n", hero)
	for _, l := range looks {
		p("<template data-look=\"%s\" data-name=\"%s\" data-desc=\"%s\"><style>%s</style>\n%s</template>\n",
			html.EscapeString(l.ID), html.EscapeString(l.Name), html.EscapeString(l.Description),
			strings.ReplaceAll(l.CSS, "</style", "<\\/style"), l.HTML)
	}
	p("<script>%s</script>\n</body>\n</html>\n", runtime)
	_, err = io.WriteString(w, b.String())
	return err
}
