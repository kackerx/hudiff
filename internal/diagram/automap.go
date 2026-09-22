package diagram

import (
	"fmt"
	"path"
	"strings"

	"github.com/kackerx/hudiff/internal/diff"
	"github.com/kackerx/hudiff/internal/repoctx"
)

var statusLabel = map[string]string{"A": "added", "D": "deleted", "R": "renamed", "M": "modified"}
var symClass = map[string]string{"+": "added", "-": "removed", "~": "changed"}

const autoMapHeader = `direction: right`

func q(s string) string { return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"` }

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
	var nodes []node
	linked := map[string]bool{}
	var extOrder []string
	extSeen := map[string]bool{}

	for _, it := range items {
		fid := q(it.Path)
		st, ok := statusLabel[it.Status]
		if !ok {
			st = it.Status
		}
		label := fmt.Sprintf("%s\\n%s  +%d -%d", path.Base(it.Path), st, it.Add, it.Del)
		if len(it.Syms) == 0 {
			plain = append(plain, plainFile{fid, label, it.Path})
			continue
		}
		blk := []string{fid + ": {", "  label: " + q(label), "  class: file", "  tooltip: " + q(it.Path)}
		for _, sym := range capSyms(it.Syms) {
			lab := sym.Kind + " " + sym.Name
			if sym.Total != 0 && sym.Noisy {
				lab += fmt.Sprintf("  ·%d refs", sym.Total)
			}
			blk = append(blk, fmt.Sprintf("  %s: { label: %s; class: %s }", q(sym.Name), q(lab), symClass[sym.Kind]))
		}
		nodes = append(nodes, node{fid, append(blk, "}")})

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
				linked[fid], linked[q(f)] = true, true
				edges = append(edges, refEdge(fid, sym.Name, f, sym.Kind == "-"))
			}
			if len(files) > 4 {
				more := fmt.Sprintf("…%d more files reference %s", len(files)-4, sym.Name)
				mid := q(fmt.Sprintf("more:%s:%s", it.Path, sym.Name))
				nodes = append(nodes, node{mid, []string{fmt.Sprintf("%s: { label: %s; class: ext }", mid, q(more))}})
				linked[fid], linked[mid] = true, true
				edges = append(edges, fmt.Sprintf("%s.%s -> %s { class: ref }", fid, q(sym.Name), mid))
			}
		}
	}
	for _, f := range extOrder {
		nodes = append(nodes, node{q(f), []string{fmt.Sprintf("%s: { label: %s; class: ext; tooltip: %s }", q(f), q(path.Base(f)), q(f))}})
	}
	for _, p := range plainNodes(plain) {
		nodes = append(nodes, p)
	}
	L = append(L, layout(nodes, linked)...)
	L = append(L, edges...)
	return strings.Join(L, "\n") + "\n"
}

type node struct {
	id   string
	text []string
}

// layout keeps the connected part as it is — TALA routes edges well — and puts
// the nodes nothing points at into a grid: on their own they stack into one tall
// column, which is what made a four-file change map 202 wide and 419 high.
func layout(nodes []node, linked map[string]bool) []string {
	var out, loose []string
	for _, n := range nodes {
		if linked[n.id] {
			out = append(out, n.text...)
			continue
		}
		loose = append(loose, n.text...)
	}
	if len(loose) == 0 {
		return out
	}
	if len(nodes) == len(loose) && len(out) == 0 && len(nodes) < 2 {
		return append(out, loose...)
	}
	out = append(out, "loose: {", "  label: \"\"", "  grid-columns: 3", "  grid-gap: 26",
		"  style: { stroke-width: 0; fill: transparent }")
	for _, l := range loose {
		out = append(out, "  "+l)
	}
	return append(out, "}")
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

func plainNodes(plain []plainFile) []node {
	if len(plain) == 0 {
		return nil
	}
	if len(plain) > 8 {
		lab := fmt.Sprintf("%d other files (no symbol-level change)", len(plain))
		return []node{{"others", []string{fmt.Sprintf("others: { label: %s; class: file }", q(lab))}}}
	}
	out := make([]node, 0, len(plain))
	for _, p := range plain {
		out = append(out, node{p.id, []string{fmt.Sprintf("%s: { label: %s; class: file; tooltip: %s }", p.id, q(p.label), q(p.path))}})
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
