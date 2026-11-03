package dashboard

import (
	"strings"
	"testing"
)

// isekai ui use must reach viewers who opened the dashboard before: only a
// look they picked is remembered, and the pick is tied to the default it beat.
func TestRuntimeRemembersOnlyPickedLooks(t *testing.T) {
	b, err := files.ReadFile("assets/runtime.js")
	if err != nil {
		t.Fatal(err)
	}
	js := strings.ReplaceAll(string(b), "\r\n", "\n") // Windows checkouts
	start := strings.Index(js, "function render(")
	if start < 0 {
		t.Fatal("runtime has no render function")
	}
	end := strings.Index(js[start:], "\n  }\n")
	if end < 0 {
		t.Fatal("cannot find the end of render")
	}
	if strings.Contains(js[start:start+end], "localStorage") {
		t.Error("render stores the look, so a later default never shows")
	}
	if !strings.Contains(js, "'isekai-look:' + forced") {
		t.Error("the stored pick is not tied to the default look")
	}
}
