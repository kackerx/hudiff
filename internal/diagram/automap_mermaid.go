package diagram

import (
	"fmt"
	"strings"

	"github.com/kackerx/hudiff/internal/repoctx"
)

// The four classes SKILL.md tells agents to use; the auto map uses the same
// ones so both dialects read alike.
const mermaidClasses = `classDef added fill:#dafbe1,stroke:#1a7f37,color:#1f2328,rx:8,ry:8
classDef removed fill:#ffebe9,stroke:#cf222e,color:#1f2328,rx:8,ry:8,stroke-dasharray:3
classDef changed fill:#fff8c5,stroke:#9a6700,color:#1f2328,rx:8,ry:8
classDef external fill:#ffffff,stroke:#d0d7de,color:#656d76,rx:8,ry:8,stroke-dasharray:2`

var mermaidClass = map[string]string{"+": "added", "-": "removed", "~": "changed"}

func mq(s string) string {
	return `"` + strings.NewReplacer(`"`, "#quot;", "<", "#lt;", ">", "#gt;").Replace(s) + `"`
}

// autoMapMermaid mirrors autoMapD2 node for node; ids are positional because
// mermaid ids cannot hold arbitrary paths.
func autoMapMermaid(items []repoctx.Item) string {
	changed := map[string]bool{}
	for _, it := range items {
		changed[it.Path] = true
	}
	L := []string{"flowchart LR", mermaidClasses}
	var edges, plain []string
	extID := map[string]string{}

	for fi, it := range items {
		st, ok := statusLabel[it.Status]
		if !ok {
			st = it.Status
		}
		label := fmt.Sprintf("%s<br/>%s +%d -%d", base(it.Path), st, it.Add, it.Del)
		fid := fmt.Sprintf("f%d", fi)
		if len(it.Syms) == 0 {
			plain = append(plain, fmt.Sprintf("  %s[%s]:::external", fid, mq(label)))
			continue
		}
		L = append(L, fmt.Sprintf("  subgraph %s[%s]", fid, mq(label)))
		for si, sym := range capSyms(it.Syms) {
			lab := sym.Kind + " " + sym.Name
			if sym.Total != 0 && sym.Noisy {
				lab += fmt.Sprintf(" ·%d 处引用", sym.Total)
			}
			L = append(L, fmt.Sprintf("    %s_%d[%s]:::%s", fid, si, mq(lab), mermaidClass[sym.Kind]))
		}
		L = append(L, "  end")

		for si, sym := range capSyms(it.Syms) {
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
				id, seen := extID[f]
				if !seen {
					id = fmt.Sprintf("x%d", len(extID))
					extID[f] = id
					if !changed[f] {
						L = append(L, fmt.Sprintf("  %s[%s]:::external", id, mq(base(f))))
					}
				}
				arrow := "-->"
				lab := ""
				if sym.Kind == "-" {
					arrow, lab = "-. \"仍在引用!\" .->", ""
				}
				edges = append(edges, fmt.Sprintf("  %s_%d %s%s %s", fid, si, arrow, lab, id))
			}
			if len(files) > 4 {
				mid := fmt.Sprintf("m%d_%d", fi, si)
				L = append(L, fmt.Sprintf("  %s[%s]:::external", mid,
					mq(fmt.Sprintf("…另 %d 个文件引用 %s", len(files)-4, sym.Name))))
				edges = append(edges, fmt.Sprintf("  %s_%d --> %s", fid, si, mid))
			}
		}
	}
	L = append(L, plainMermaid(plain)...)
	L = append(L, edges...)
	return strings.Join(L, "\n") + "\n"
}

func plainMermaid(plain []string) []string {
	if len(plain) > 8 {
		return []string{fmt.Sprintf("  others[%s]:::external",
			mq(fmt.Sprintf("其它 %d 个文件(无符号级改动)", len(plain))))}
	}
	return plain
}
