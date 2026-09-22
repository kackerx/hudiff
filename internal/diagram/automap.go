package diagram

import (
	"fmt"
	"strings"

	"github.com/kackerx/hudiff/internal/diff"
	"github.com/kackerx/hudiff/internal/repoctx"
)

var statusLabel = map[string]string{"A": "added", "D": "deleted", "R": "renamed", "M": "modified"}
var symClass = map[string]string{"+": "added", "-": "removed", "~": "changed"}

const autoMapHeader = `direction: right`

func q(s string) string { return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"` }

func base(p string) string { return p[strings.LastIndex(p, "/")+1:] }

type plainFile struct{ id, label, path string }

// AutoMapSrc renders the change map: file → symbols it touches → outside
// references. Generated from repoctx, never by an agent.
func AutoMapSrc(repo string, d diff.Diff) string {
	items := repoctx.Collect(repo, d, "", 4)
	changed := map[string]bool{}
	for _, it := range items {
		changed[it.Path] = true
	}
	L := strings.Split(autoMapHeader, "\n")
	var edges []string
	var plain []plainFile
	var extOrder []string
	extSeen := map[string]bool{}

	for _, it := range items {
		fid := q(it.Path)
		st, ok := statusLabel[it.Status]
		if !ok {
			st = it.Status
		}
		label := fmt.Sprintf("%s\\n%s  +%d -%d", base(it.Path), st, it.Add, it.Del)
		if len(it.Syms) == 0 {
			plain = append(plain, plainFile{fid, label, it.Path})
			continue
		}
		L = append(L, fid+": {", "  label: "+q(label), "  class: file", "  tooltip: "+q(it.Path))
		for _, sym := range capSyms(it.Syms) {
			lab := sym.Kind + " " + sym.Name
			if sym.Total != 0 && sym.Noisy {
				lab += fmt.Sprintf("  ·%d refs", sym.Total)
			}
			L = append(L, fmt.Sprintf("  %s: { label: %s; class: %s }", q(sym.Name), q(lab), symClass[sym.Kind]))
		}
		L = append(L, "}")

		for _, sym := range capSyms(it.Syms) {
			if sym.Noisy || sym.Total == 0 {
				continue
			}
			var files []string
			for _, f := range sym.RefFiles {
				if f != it.Path {
					files = append(files, f)
				}
			}
			for _, f := range capFiles(files) {
				if !changed[f] && !extSeen[f] {
					extSeen[f] = true
					extOrder = append(extOrder, f)
				}
				edges = append(edges, refEdge(fid, sym.Name, f, sym.Kind == "-"))
			}
			if len(files) > 4 {
				more := fmt.Sprintf("…%d more files reference %s", len(files)-4, sym.Name)
				mid := q(fmt.Sprintf("more:%s:%s", it.Path, sym.Name))
				L = append(L, fmt.Sprintf("%s: { label: %s; class: ext }", mid, q(more)))
				edges = append(edges, fmt.Sprintf("%s.%s -> %s { class: ref }", fid, q(sym.Name), mid))
			}
		}
	}
	for _, f := range extOrder {
		L = append(L, fmt.Sprintf("%s: { label: %s; class: ext; tooltip: %s }", q(f), q(base(f)), q(f)))
	}
	L = append(L, plainNodes(plain)...)
	L = append(L, edges...)
	return strings.Join(L, "\n") + "\n"
}

func refEdge(fid, sym, target string, danger bool) string {
	e := fmt.Sprintf("%s.%s -> %s", fid, q(sym), q(target))
	cls := "ref"
	if danger {
		e += ": " + q("still referenced!")
		cls = "danger"
	}
	return e + " { class: " + cls + " }"
}

func plainNodes(plain []plainFile) []string {
	if len(plain) == 0 {
		return nil
	}
	if len(plain) > 8 {
		lab := fmt.Sprintf("%d other files (no symbol-level change)", len(plain))
		return []string{fmt.Sprintf("others: { label: %s; class: file }", q(lab))}
	}
	out := make([]string, 0, len(plain))
	for _, p := range plain {
		out = append(out, fmt.Sprintf("%s: { label: %s; class: file; tooltip: %s }", p.id, q(p.label), q(p.path)))
	}
	return out
}

func capSyms(s []repoctx.Sym) []repoctx.Sym {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

func capFiles(f []string) []string {
	if len(f) > 4 {
		return f[:4]
	}
	return f
}
