package diagram

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/kackerx/hudiff/internal/config"
	"github.com/kackerx/hudiff/internal/diff"
)

const autoMapCaption = "File → the symbols this change adds (green) / removes (red) / edits (amber) → " +
	"who references them outside the changed files. A red edge means the symbol is gone but something " +
	"still calls it. Generated from `hudiff context`, not drawn by an agent."

// Config comes from config.json. d2 is the only engine (2026-09-22: mermaid
// dropped), so sketch is all that is left.
type Config struct {
	Sketch bool
}

func DefaultConfig() Config { return Config{Sketch: config.Load().Diagram.Sketch} }

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

func List(dir string, d diff.Diff, cfg Config) []Entry {
	out := []Entry{}
	if anyHunks(d) {
		out = append(out, Entry{ID: "auto-map", Title: "Change map (automatic)", Kind: "d2",
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
			title = fmt.Sprintf("Diagram %d", i+1)
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

// Source returns the diagram's source and engine, resolving `which` the way the
// oracle did: "auto-map", an index, or an id.
func Source(dir, repo string, d diff.Diff, which string, cfg Config) (src, kind string, err error) {
	if which == "auto-map" {
		return autoMapCached(dir, repo, d), "d2", nil
	}
	dgs := loadRawDiagrams(dir)
	var dg map[string]any
	if i, e := strconv.Atoi(which); e == nil && i >= 0 && i < len(dgs) {
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
		return "", "", fmt.Errorf("no such diagram: %s", which)
	}
	return strOr(dg, "src", ""), strOr(dg, "kind", "d2"), nil
}

// Render returns SVG bytes, always succeeding: a failure comes back as an error
// card so the page has something to show.
func Render(dir, repo string, d diff.Diff, which, theme string, cfg Config) []byte {
	src, _, err := Source(dir, repo, d, which, cfg)
	if err != nil {
		return errorSVG(err.Error())
	}
	cache := filepath.Join(dir, "diagrams", cacheKey(src, theme, cfg.Sketch)+".svg")
	if b, err := os.ReadFile(cache); err == nil {
		return b
	}
	svg, err := renderD2(src, theme, cfg.Sketch)
	if err != nil {
		return errorSVG(err.Error())
	}
	_ = os.MkdirAll(filepath.Dir(cache), 0o755)
	_ = os.WriteFile(cache, svg, 0o644)
	return svg
}

// cacheKey covers everything that changes the pixels: the source, the theme,
// sketch mode and the style preamble.
func cacheKey(src, theme string, sketch bool) string {
	sum := sha1.Sum([]byte(theme + "\x00" + strconv.FormatBool(sketch) + "\x00" + Preamble + "\x00" + src))
	return hex.EncodeToString(sum[:])[:16]
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
		`<text x="16" y="22" font-family="sans-serif" font-size="13" fill="#82071e">diagram failed to render</text>%s</svg>`,
		60+18*n, body.String()))
}
