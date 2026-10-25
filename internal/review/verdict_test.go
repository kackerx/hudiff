package review

import (
	"strings"
	"testing"

	"github.com/kackerx/hudiff/internal/diff"
)

func TestVerdict(t *testing.T) {
	for _, c := range []struct {
		v  any
		ok bool
	}{
		{nil, true},
		{map[string]any{"decision": "fix-then-merge", "reason": "r1 first"}, true},
		{map[string]any{"decision": "ship-it", "reason": "x"}, false},
		{map[string]any{"decision": "merge"}, false},
		{"merge", false},
	} {
		rv := Report{"summary": "s", "items": []any{}}
		if c.v != nil {
			rv["verdict"] = c.v
		}
		if got := len(Validate(rv, diff.Diff{})) == 0; got != c.ok {
			t.Errorf("verdict %v: valid=%v, want %v", c.v, got, c.ok)
		}
	}
	old := Report{"summary": "s", "items": []any{}, "verdict": map[string]any{"decision": "do-not-merge", "reason": "a"}}
	m := Merge(old, Report{"items": []any{}, "verdict": map[string]any{"decision": "merge", "reason": "fixed"}}, false)
	if !strings.Contains(Markdown(m), "**Ready to merge**: fixed") {
		t.Errorf("markdown lost the newer verdict:\n%s", Markdown(m))
	}
}
