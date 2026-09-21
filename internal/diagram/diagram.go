// Package diagram lists and renders the session's diagrams. d2 is rendered
// server side with a hash cache; mermaid is left to the browser.
package diagram

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/kackerx/hudiff/internal/diff"
)

const autoMapCaption = "文件 → 本次新增(绿)/删除(红)/改动(黄)的符号 → 改动文件之外的引用方。" +
	"红线 = 符号被删了但别处还在引用。由 `hudiff ctx` 的数据自动生成, 不是 agent 画的。"

var multiboard = regexp.MustCompile(`(?m)^\s*(steps|layers|scenarios)\s*:`)

var d2Theme = map[string]string{"light": "0", "dark": "200"}

var d2Bin = sync.OnceValue(func() string {
	p, err := exec.LookPath("d2")
	if err != nil {
		return ""
	}
	return p
})

func HasD2() bool { return d2Bin() != "" }

type Entry struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Kind    string   `json:"kind"`
	Auto    bool     `json:"auto"`
	Hunks   []string `json:"hunks"`
	Caption string   `json:"caption"`
	Src     *string  `json:"src,omitempty"`
	Index   *int     `json:"index,omitempty"`
}

type rawNotes struct {
	Diagrams []map[string]any `json:"diagrams"`
}

func loadRawDiagrams(dir string) []map[string]any {
	var n rawNotes
	b, err := os.ReadFile(filepath.Join(dir, "notes.json"))
	if err != nil {
		return nil
	}
	_ = json.Unmarshal(b, &n)
	return n.Diagrams
}

func str(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

func strOr(m map[string]any, k, def string) string {
	if v, ok := m[k]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}

func strList(m map[string]any, k string) []string {
	out := []string{}
	if arr, ok := m[k].([]any); ok {
		for _, v := range arr {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

func List(dir string, d diff.Diff) []Entry {
	out := []Entry{}
	if HasD2() && anyHunks(d) {
		out = append(out, Entry{ID: "auto-map", Title: "改动地图(自动)", Kind: "d2",
			Auto: true, Hunks: []string{}, Caption: autoMapCaption})
	}
	for i, dg := range loadRawDiagrams(dir) {
		if dg == nil {
			continue
		}
		id := str(dg, "id")
		if id == "" {
			id = "d" + strconv.Itoa(i)
		}
		title := str(dg, "title")
		if title == "" {
			title = fmt.Sprintf("图 %d", i+1)
		}
		src, idx := strOr(dg, "src", ""), i
		out = append(out, Entry{ID: id, Title: title, Kind: strOr(dg, "kind", "d2"), Auto: false,
			Hunks: strList(dg, "hunks"), Caption: str(dg, "caption"), Src: &src, Index: &idx})
	}
	return out
}

func anyHunks(d diff.Diff) bool {
	for _, f := range d.Files {
		if len(f.Hunks) > 0 {
			return true
		}
	}
	return false
}

// Render returns SVG bytes for `which`, always succeeding: failures come back
// as an error card so the page has something to show.
func Render(dir, repo string, d diff.Diff, which, theme string) []byte {
	var src string
	if which == "auto-map" {
		src = autoMapCached(dir, repo, d)
	} else {
		dgs := loadRawDiagrams(dir)
		var dg map[string]any
		if i, err := strconv.Atoi(which); err == nil && i >= 0 && i < len(dgs) {
			dg = dgs[i]
		} else {
			for _, x := range dgs {
				if x != nil && str(x, "id") == which {
					dg = x
					break
				}
			}
		}
		if dg == nil {
			return errorSVG("没有这张图: " + which)
		}
		if strOr(dg, "kind", "d2") != "d2" {
			return errorSVG("这张图不是 d2, 由浏览器渲染")
		}
		src = strOr(dg, "src", "")
	}
	sum := sha1.Sum([]byte(theme + "\x00" + src))
	cache := filepath.Join(dir, "diagrams", hex.EncodeToString(sum[:])[:16]+".svg")
	if b, err := os.ReadFile(cache); err == nil {
		return b
	}
	svg, err := renderD2(src, theme)
	if err != nil {
		return errorSVG(err.Error())
	}
	_ = os.MkdirAll(filepath.Dir(cache), 0o755)
	_ = os.WriteFile(cache, svg, 0o644)
	return svg
}

type autoSrc struct {
	mtime int64
	src   string
}

var autoCache sync.Map // session dir -> autoSrc

// autoMapCached keeps the generated source per diff.json mtime: building it
// shells out to git grep for every changed symbol.
func autoMapCached(dir, repo string, d diff.Diff) string {
	var mt int64
	if st, err := os.Stat(filepath.Join(dir, "diff.json")); err == nil {
		mt = st.ModTime().UnixNano()
	}
	if hit, ok := autoCache.Load(dir); ok {
		if c := hit.(autoSrc); c.mtime == mt {
			return c.src
		}
	}
	src := AutoMapSrc(repo, d)
	autoCache.Store(dir, autoSrc{mt, src})
	return src
}

func renderD2(src, theme string) ([]byte, error) {
	if !HasD2() {
		return nil, fmt.Errorf("d2 未安装: brew install d2")
	}
	th, ok := d2Theme[theme]
	if !ok {
		th = "0"
	}
	args := []string{"--theme", th, "--pad", "16", "--layout", "dagre", "--scale", "1"}
	if multiboard.MatchString(src) {
		return renderMultiboard(args, src)
	}
	return runD2(append(args, "-", "-"), src)
}

// Multi-board sources (steps/layers/scenarios) cannot stream to stdout, so
// they go through a temp file and become a frame animation.
func renderMultiboard(args []string, src string) ([]byte, error) {
	td, err := os.MkdirTemp("", "hudiff-d2")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(td)
	out := filepath.Join(td, "out.svg")
	if _, err := runD2(append(args, "--animate-interval", "1400", "-", out), src); err != nil {
		return nil, err
	}
	return os.ReadFile(out)
}

func runD2(args []string, src string) ([]byte, error) {
	cmd := exec.Command(d2Bin(), args...)
	cmd.Stdin = strings.NewReader(src)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func errorSVG(msg string) []byte {
	msg = strings.ReplaceAll(msg, "&", "&amp;")
	msg = strings.ReplaceAll(msg, "<", "&lt;")
	r := []rune(msg)
	if len(r) > 900 {
		r = r[:900]
	}
	var body strings.Builder
	n := 0
	for i := 0; i < len(r); i += 90 {
		body.WriteString(fmt.Sprintf(`<text x="16" y="%d" font-family="monospace" font-size="12" fill="#cf222e">%s</text>`,
			40+n*18, string(r[i:min(i+90, len(r))])))
		n++
	}
	return []byte(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="860" height="%d">`+
		`<rect width="100%%" height="100%%" fill="#fff5f5" stroke="#f0a0a0"/>`+
		`<text x="16" y="22" font-family="sans-serif" font-size="13" fill="#82071e">图渲染失败</text>%s</svg>`,
		60+18*n, body.String()))
}
