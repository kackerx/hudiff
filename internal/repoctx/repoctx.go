// Package repoctx extracts repo-level context for a diff: which symbols the
// change defines/removes and who else references them. Port of collect_ctx();
// spec §5 calls this `context/`, renamed to avoid shadowing stdlib context.
package repoctx

import (
	"os/exec"
	"regexp"
	"strings"

	"github.com/kackerx/hudiff/internal/diff"
)

const noiseCap = 120

var symRes = map[string][]*regexp.Regexp{
	"go": rx(`^func\s+(?:\([^)]*\)\s*)?(\w+)`, `^type\s+(\w+)`, `^(?:var|const)\s+(\w+)`),
	"py": rx(`^\s*(?:async\s+)?def\s+(\w+)`, `^\s*class\s+(\w+)`, `^([A-Z_][A-Z0-9_]{2,})\s*=`),
	"jsts": rx(`^(?:export\s+)?(?:default\s+)?(?:async\s+)?function\s*\*?\s*(\w+)`,
		`^(?:export\s+)?(?:default\s+)?class\s+(\w+)`,
		`^export\s+(?:const|let|var)\s+(\w+)`,
		`^(?:export\s+)?(?:interface|type|enum)\s+(\w+)`,
		`^\s+(?:async\s+)?(\w+)\s*\([^)]*\)\s*\{`),
	"lua": rx(`^(?:local\s+)?function\s+([\w.:]+)`, `^(\w+(?:\.\w+)+)\s*=\s*function`, `^M\.(\w+)\s*=`),
	"rs": rx(`^\s*(?:pub(?:\([^)]*\))?\s+)?(?:async\s+)?fn\s+(\w+)`,
		`^\s*(?:pub(?:\([^)]*\))?\s+)?(?:struct|enum|trait|type)\s+(\w+)`),
	"sh": rx(`^\s*(?:function\s+)?([A-Za-z_]\w*)\s*\(\)\s*\{`, `^([A-Z_][A-Z0-9_]{2,})=`),
	"java": rx(`^\s*(?:public|private|protected|static|\s)*[\w<>\[\],\s]+\s+(\w+)\s*\([^)]*\)\s*(?:throws[^{]*)?\{`,
		`^\s*(?:public\s+)?(?:class|interface|enum|record)\s+(\w+)`),
}

var langFamily = map[string]string{
	"go": "go", "py": "py", "js": "jsts", "mjs": "jsts", "ts": "jsts", "tsx": "jsts",
	"jsx": "jsts", "vue": "jsts", "lua": "lua", "rs": "rs", "sh": "sh", "bash": "sh",
	"zsh": "sh", "java": "java", "kt": "java", "swift": "java", "c": "java", "cpp": "java", "h": "java",
}

func rx(pats ...string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, len(pats))
	for i, p := range pats {
		out[i] = regexp.MustCompile(p)
	}
	return out
}

type Sym struct {
	Name     string
	Kind     string
	Hunk     int
	Total    int
	Noisy    bool
	RefFiles []string
}

type Item struct {
	Path   string
	Status string
	Add    int
	Del    int
	Fam    string
	Syms   []Sym
}

func symFamily(path string) string {
	base := path[strings.LastIndex(path, "/")+1:]
	ext := ""
	if i := strings.LastIndex(base, "."); i >= 0 {
		ext = strings.ToLower(base[i+1:])
	}
	if ext == "" && base != "" && !strings.HasPrefix(base, ".") {
		return "sh"
	}
	return langFamily[ext]
}

// ordered keeps Python dict insertion order, which the auto map depends on.
type ordered struct {
	keys []string
	val  map[string]int
}

func newOrdered() *ordered { return &ordered{val: map[string]int{}} }

func (o *ordered) setDefault(k string, v int) {
	if _, ok := o.val[k]; !ok {
		o.keys = append(o.keys, k)
		o.val[k] = v
	}
}

func Collect(repo string, d diff.Diff, only string, refsCap int) []Item {
	var out []Item
	for _, f := range d.Files {
		if only != "" && f.Path != only {
			continue
		}
		it := Item{Path: f.Path, Status: f.Status, Add: f.Add, Del: f.Del, Fam: symFamily(f.Path)}
		added, removed := scanSymbols(f, it.Fam)
		for _, e := range mergeOrder(added, removed) {
			it.Syms = append(it.Syms, resolveRefs(repo, f.Path, e))
		}
		out = append(out, it)
	}
	return out
}

type entry struct {
	name string
	kind string
	hunk int
}

func scanSymbols(f diff.File, fam string) (*ordered, *ordered) {
	added, removed := newOrdered(), newOrdered()
	if fam == "" {
		return added, removed
	}
	for hi, h := range f.Hunks {
		for _, ln := range h.Lines {
			if ln.T == " " {
				continue
			}
			for _, re := range symRes[fam] {
				m := re.FindStringSubmatch(ln.S)
				if m == nil || len(m[1]) <= 1 || (m[1][0] >= '0' && m[1][0] <= '9') {
					continue
				}
				if ln.T == "+" {
					added.setDefault(m[1], hi)
				} else {
					removed.setDefault(m[1], hi)
				}
				break
			}
		}
	}
	return added, removed
}

func mergeOrder(added, removed *ordered) []entry {
	var out []entry
	for _, k := range added.keys {
		if _, both := removed.val[k]; both {
			out = append(out, entry{k, "~", added.val[k]})
		}
	}
	for _, k := range added.keys {
		if _, both := removed.val[k]; !both {
			out = append(out, entry{k, "+", added.val[k]})
		}
	}
	for _, k := range removed.keys {
		if _, both := added.val[k]; !both {
			out = append(out, entry{k, "-", removed.val[k]})
		}
	}
	if len(out) > 20 {
		out = out[:20]
	}
	return out
}

func resolveRefs(repo, path string, e entry) Sym {
	needle := e.name
	needle = needle[strings.LastIndex(needle, ".")+1:]
	needle = needle[strings.LastIndex(needle, ":")+1:]
	var hits []string
	for _, l := range strings.Split(git(repo, "grep", "-n", "-w", "-I", "--", needle, "."), "\n") {
		if l != "" && !strings.HasPrefix(l, path+":") {
			hits = append(hits, l)
		}
	}
	s := Sym{Name: e.name, Kind: e.kind, Hunk: e.hunk, Total: len(hits), Noisy: len(hits) > noiseCap}
	if s.Noisy {
		return s
	}
	seen := map[string]bool{}
	for _, l := range hits {
		fn, _, _ := strings.Cut(l, ":")
		seen[fn] = true
	}
	s.RefFiles = sortedKeys(seen)
	return s
}

func git(repo string, args ...string) string {
	out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(string(out), "\n")
}
