package dashboard

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// Skins are HTML written by agents and shared through git, so they are
// treated as untrusted. They are checked with an HTML tokenizer and an
// allow-list, not with regular expressions over the raw text: attribute
// values are entity-decoded first, so jav&#x61;script: and <svg/onload>
// are seen for what they are. The dashboard's CSP additionally allows only
// the runtime script (by hash), so an event attribute that slipped through
// still could not run.

// Elements a skin may not contain: they run code, load or navigate.
var blockedElems = map[string]bool{
	"script": true, "iframe": true, "frame": true, "frameset": true, "object": true, "embed": true,
	"applet": true, "portal": true, "fencedframe": true, "meta": true, "base": true, "link": true,
	"form": true, "noscript": true, "foreignobject": true, "animate": true,
	"set": true, "animatemotion": true, "animatetransform": true, "handler": true, "listener": true,
}

// URL-carrying attributes. Their values may only point inside the page.
var urlAttrs = map[string]bool{
	"href": true, "src": true, "srcset": true, "action": true, "formaction": true, "poster": true,
	"background": true, "ping": true, "cite": true, "data": true, "codebase": true, "manifest": true,
	"xlink:href": true, "lowsrc": true, "dynsrc": true, "longdesc": true, "usemap": true,
}

func localURL(v string) bool {
	v = strings.TrimSpace(strings.ToLower(v))
	switch {
	case v == "", strings.HasPrefix(v, "#"):
		return true
	case strings.HasPrefix(v, "data:image/"): // images cannot run script
		return true
	case strings.HasPrefix(v, "assets/"), strings.HasPrefix(v, "./assets/"):
		return !strings.Contains(v, "..") && !strings.Contains(v, ":")
	}
	return false
}

// unsafeHTML returns the first reason s is not allowed in a skin, or "".
func unsafeHTML(s string) string {
	z := html.NewTokenizer(strings.NewReader(s))
	inStyle := false
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				return ""
			}
			return "unparseable HTML"
		case html.StartTagToken, html.SelfClosingTagToken, html.EndTagToken:
			t := z.Token()
			name := strings.ToLower(t.Data)
			inStyle = name == "style" && tt == html.StartTagToken
			if blockedElems[name] {
				return "<" + name + ">"
			}
			if tt == html.EndTagToken {
				continue
			}
			for _, a := range t.Attr {
				k := strings.ToLower(a.Key)
				if a.Namespace != "" {
					k = strings.ToLower(a.Namespace) + ":" + k
				}
				switch {
				case strings.HasPrefix(k, "on"):
					return fmt.Sprintf("an event attribute (%s)", k)
				case k == "http-equiv" || k == "srcdoc" || k == "is":
					return fmt.Sprintf("the %s attribute", k)
				case urlAttrs[k] || strings.HasSuffix(k, ":href"):
					if !localURL(a.Val) {
						return fmt.Sprintf("a non-local URL in %s (only #fragments, assets/ files and data:image)", k)
					}
				case k == "style":
					if bad := unsafeCSS(a.Val); bad != "" {
						return bad
					}
				}
			}
		case html.TextToken:
			if inStyle {
				if bad := unsafeCSS(string(z.Text())); bad != "" {
					return bad
				}
			}
		}
	}
}

var cssURLRe = regexp.MustCompile(`(?i)url\(\s*(?:"([^"]*)"|'([^']*)'|([^)]*))\s*\)`)

// unsafeCSS returns what makes a stylesheet unsafe, or "".
func unsafeCSS(s string) string {
	low := strings.ToLower(s)
	if strings.Contains(low, "@import") {
		return "@import"
	}
	if strings.Contains(low, "expression(") || strings.Contains(low, "-moz-binding") || strings.Contains(low, "behavior:") {
		return "scriptable CSS"
	}
	if strings.Contains(s, `\`) && strings.Contains(low, "url") {
		return "escaped characters near url()"
	}
	for _, m := range cssURLRe.FindAllStringSubmatch(s, -1) {
		v := m[1] + m[2] + m[3]
		if !localURL(v) && !strings.HasPrefix(strings.ToLower(strings.TrimSpace(v)), "data:font/") && !strings.HasPrefix(strings.ToLower(strings.TrimSpace(v)), "data:application/font") {
			return "a non-local url() (only assets/ files and data: images and fonts)"
		}
	}
	return ""
}

// unsafeSkin checks a skin's HTML, CSS and SVG assets.
func unsafeSkin(htmlSrc, css string, svgs []string) string {
	if bad := unsafeHTML(htmlSrc); bad != "" {
		return bad
	}
	if bad := unsafeCSS(css); bad != "" {
		return bad
	}
	for _, s := range svgs {
		if bad := unsafeHTML(s); bad != "" {
			return "in an SVG asset: " + bad
		}
	}
	return ""
}
