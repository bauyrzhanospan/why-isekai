package dashboard

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestUnsafeHTMLRejects(t *testing.T) {
	for _, bad := range []string{
		`<svg/onload=alert(1)>`,
		`<img src="data:,x"onerror="alert(1)">`,
		`<a href="jav&#x61;script:alert(2)">x</a>`,
		`<a href="http://10.0.0.1/">x</a>`,
		`<a href="//evil.example/">x</a>`,
		`<meta http-equiv="refresh" content="5;url=http://10.0.0.1/">`,
		`<form action="#"><button>go</button></form>`,
		`<button formaction="http://10.0.0.1/">go</button>`,
		`<div style="background:url(http://10.0.0.1/x.png)">`,
		`<style>@import "x.css";</style>`,
		`<style>b{background:url( 'https://e.example/p.png' )}</style>`,
		`<SCRIPT>alert(1)</SCRIPT>`,
		`<iframe srcdoc="x">`,
		`<svg><a xlink:href="javascript:alert(1)"><text>x</text></a></svg>`,
		`<svg><animate attributeName="href" values="javascript:alert(1)"/></svg>`,
		`<base href="#">`,
		`<link rel="stylesheet" href="assets/a.css">`,
	} {
		if unsafeHTML(bad) == "" {
			t.Errorf("accepted %s", bad)
		}
	}
}

func TestUnsafeHTMLAllowsBuiltins(t *testing.T) {
	for _, id := range []string{"isekai", "corporate"} {
		h, _ := files.ReadFile("looks/" + id + "/index.html")
		c, _ := files.ReadFile("looks/" + id + "/skin.css")
		if bad := unsafeSkin(string(h), string(c), nil); bad != "" {
			t.Errorf("%s: %s", id, bad)
		}
	}
	ok := `<a href="#quests">q</a><img src="assets/p.png" alt=""><svg xmlns="http://www.w3.org/2000/svg"><use href="#i"/></svg><style>@font-face{src:url(assets/h.woff2)}</style>`
	if bad := unsafeHTML(ok); bad != "" {
		t.Error(bad)
	}
}

// A skin cannot pull a file in through a symlink.
func TestLoadSkinSkipsSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks")
	}
	dir := filepath.Join(t.TempDir(), "s")
	os.MkdirAll(filepath.Join(dir, "assets"), 0o755)
	secret := filepath.Join(t.TempDir(), "id_rsa")
	os.WriteFile(secret, []byte("PRIVATE-KEY-MATERIAL"), 0o600)
	os.Symlink(secret, filepath.Join(dir, "assets", "x.png"))
	os.WriteFile(filepath.Join(dir, "index.html"), []byte(`<img src="assets/x.png" alt="">`), 0o644)
	l, err := LoadSkin(dir)
	if err == nil && strings.Contains(l.HTML, "UFJJVkFURS1LRVk") { // base64 of the secret
		t.Fatal("symlinked file inlined")
	}
	os.Remove(filepath.Join(dir, "index.html"))
	os.Symlink(secret, filepath.Join(dir, "index.html"))
	if _, err := LoadSkin(dir); err == nil {
		t.Fatal("symlinked index.html accepted")
	}
}
