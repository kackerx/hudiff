package diagram

import (
	"fmt"
	"strings"

	"github.com/kackerx/hudiff/internal/diff"
	"github.com/kackerx/hudiff/internal/repoctx"
)

var statusLabel = map[string]string{"A": "新增", "D": "删除", "R": "重命名", "M": "修改"}
var symClass = map[string]string{"+": "added", "-": "removed", "~": "changed"}

const autoMapHeader = `direction: right
classes: {
  file: { style: { fill: "#f6f8fa"; stroke: "#8b949e"; border-radius: 8; font-size: 14 } }
  added: { style: { fill: "#dafbe1"; stroke: "#1a7f37"; font-size: 13; border-radius: 4 } }
  removed: { style: { fill: "#ffebe9"; stroke: "#cf222e"; font-size: 13; border-radius: 4; stroke-dash: 3 } }
  changed: { style: { fill: "#fff8c5"; stroke: "#9a6700"; font-size: 13; border-radius: 4 } }
  ext: { style: { fill: "#ffffff"; stroke: "#d0d7de"; font-size: 12; border-radius: 6; stroke-dash: 2 } }
  danger: { style: { stroke: "#cf222e"; stroke-width: 2; font-color: "#cf222e" } }
  ref: { style: { stroke: "#8b949e"; font-size: 11 } }
}`

func q(s string) string { return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"` }

func base(p string) string { return p[strings.LastIndex(p, "/")+1:] }

type plainFile struct{ id, label, path string }

// AutoMapSrc renders the change map: file → symbols it touches → outside
// references. Generated from repoctx, never written by an agent.
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
				lab += fmt.Sprintf("  ·%d 处引用", sym.Total)
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
				more := fmt.Sprintf("…另 %d 个文件引用 %s", len(files)-4, sym.Name)
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
		e += ": " + q("仍在引用!")
		cls = "danger"
	}
	return e + " { class: " + cls + " }"
}

func plainNodes(plain []plainFile) []string {
	if len(plain) == 0 {
		return nil
	}
	if len(plain) > 8 {
		lab := fmt.Sprintf("其它 %d 个文件(无符号级改动)", len(plain))
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
