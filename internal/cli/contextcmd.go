package cli

import (
	"fmt"
	"strings"

	"github.com/kackerx/hudiff/internal/diff"
	"github.com/kackerx/hudiff/internal/repoctx"
	"github.com/kackerx/hudiff/internal/session"
)

func (c *ContextCmd) Run(g *Globals) error {
	sid, dir, err := session.Current(g.Session)
	if err != nil {
		return usage("%v", err)
	}
	sess, err := session.LoadSession(dir)
	if err != nil {
		return usage("%v", err)
	}
	d := diff.Load(dir)
	changed := map[string]bool{}
	for _, f := range d.Files {
		changed[f.Path] = true
	}
	fmt.Printf("# hudiff session %s: repo-level context  (%s)\n", sid, sess.Repo)
	fmt.Println("# Read this before writing notes: callers tell you the blast radius, recent history is often the reason for the change, tests tell you whether it is covered.")
	for _, it := range repoctx.Collect(sess.Repo, d, c.File, c.Refs) {
		printCtxItem(it, changed)
	}
	return nil
}

func printCtxItem(it repoctx.Item, changed map[string]bool) {
	fmt.Printf("\n=== %s  [%s]  +%d -%d\n", it.Path, it.Status, it.Add, it.Del)
	if len(it.Hist) > 0 {
		fmt.Println("recent history:")
		for _, h := range it.Hist {
			fmt.Println("  " + h)
		}
	}
	switch {
	case it.Fam == "":
		fmt.Println("symbols: (no symbol rules for this file type, skipped)")
	case len(it.Syms) > 0:
		var parts []string
		for _, s := range it.Syms {
			parts = append(parts, fmt.Sprintf("%s%s(#%d)", s.Kind, s.Name, s.Hunk))
		}
		fmt.Println("symbols: " + strings.Join(parts, "  "))
	}
	if len(it.Syms) > 0 {
		fmt.Println("references (outside the changed files):")
		for _, s := range it.Syms {
			printSym(s)
		}
	}
	printTests(it, changed)
}

func printSym(s repoctx.Sym) {
	short := s.Name[strings.LastIndex(s.Name, ".")+1:]
	if s.Noisy {
		fmt.Printf("  %s: %d hits, too common a name to list (git grep -nw %s)\n", s.Name, s.Total, short)
		return
	}
	if s.Total == 0 {
		tag := "  (nothing else in the repo references it)"
		switch s.Kind {
		case "-":
			tag = "  (removed, nothing else referenced it, clean)"
		case "+":
			tag = "  (new symbol, no callers yet)"
		}
		fmt.Printf("  %s: 0%s\n", s.Name, tag)
		return
	}
	var extra []string
	if s.InChanged > 0 {
		extra = append(extra, fmt.Sprintf("%d of them in other files this change touches", s.InChanged))
	}
	if s.InTests > 0 {
		extra = append(extra, fmt.Sprintf("%d in tests", s.InTests))
	}
	if s.Kind == "-" {
		extra = append(extra, "⚠ the symbol was removed but is still referenced")
	}
	tail := ""
	if len(extra) > 0 {
		tail = "  (" + strings.Join(extra, "; ") + ")"
	}
	fmt.Printf("  %s: %d hits%s\n", s.Name, s.Total, tail)
	for _, r := range s.Refs {
		fmt.Printf("      %s:%s  %s\n", r.File, r.Line, r.Text)
	}
	if s.Total > len(s.Refs) {
		fmt.Printf("      … %d more\n", s.Total-len(s.Refs))
	}
}

func printTests(it repoctx.Item, changed map[string]bool) {
	if len(it.Tests) == 0 {
		if !repoctx.IsTest(it.Path) && it.Status != "D" {
			fmt.Println("related tests: none found  ⚠ this change has no test coverage (or the tests are named unusually)")
		}
		return
	}
	inDiff := 0
	for _, t := range it.Tests {
		if changed[t] {
			inDiff++
		}
	}
	shown, more := it.Tests, ""
	if len(shown) > 8 {
		shown, more = shown[:8], " …"
	}
	tail := "   (none of them changed here)"
	if inDiff > 0 {
		tail = fmt.Sprintf("   (%d of them changed here)", inDiff)
	}
	fmt.Println("related tests: " + strings.Join(shown, ", ") + more + tail)
}
