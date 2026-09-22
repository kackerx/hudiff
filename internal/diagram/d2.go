// d2 is compiled in as a library (spec §7): no external binary, layout fixed
// to TALA. Nothing outside this package may import d2.
package diagram

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/d2lang/d2/d2graph"
	"github.com/d2lang/d2/d2layouts/d2talalayout"
	"github.com/d2lang/d2/d2lib"
	"github.com/d2lang/d2/d2renderers/d2ascii"
	"github.com/d2lang/d2/d2renderers/d2ascii/charset"
	"github.com/d2lang/d2/d2renderers/d2svg"
	"github.com/d2lang/d2/d2target"
	"github.com/d2lang/d2/lib/log"
	"github.com/d2lang/d2/lib/textmeasure"
)

// Preamble is hudiff's house style, prepended to every source — the agent's
// diagrams and the generated change map alike, so both draw from the same
// palette (spec §7). It is part of the SVG cache key.
const Preamble = `**.style.border-radius: 8
classes: {
  file: { style: { fill: "#f6f8fa"; stroke: "#8b949e"; border-radius: 8; font-size: 14 } }
  added: { style: { fill: "#dafbe1"; stroke: "#1a7f37"; font-size: 13; border-radius: 4 } }
  removed: { style: { fill: "#ffebe9"; stroke: "#cf222e"; font-size: 13; border-radius: 4; stroke-dash: 3 } }
  changed: { style: { fill: "#fff8c5"; stroke: "#9a6700"; font-size: 13; border-radius: 4 } }
  ext: { style: { fill: "#ffffff"; stroke: "#d0d7de"; font-size: 12; border-radius: 6; stroke-dash: 2 } }
  danger: { style: { stroke: "#cf222e"; stroke-width: 2; font-color: "#cf222e" } }
  ref: { style: { stroke: "#8b949e"; font-size: 11 } }
}
`

var themeID = map[string]int64{"light": 0, "dark": 200}

// The ruler is expensive to build, so it is shared — but it fills its font-face
// map lazily and is not safe to use from two goroutines at once: the page asks
// for every diagram in parallel and that killed the daemon with
// "fatal error: concurrent map writes".
// ponytail: one lock for all compiles; a ruler pool if cold renders pile up.
var ruler = sync.OnceValues(textmeasure.NewRuler)
var compiling sync.Mutex

func compile(src, theme string, sketch bool) (*d2target.Diagram, *d2svg.RenderOpts, error) {
	r, err := ruler()
	if err != nil {
		return nil, nil, err
	}
	id, ok := themeID[theme]
	if !ok {
		id = themeID["light"]
	}
	pad := int64(16)
	ro := &d2svg.RenderOpts{Pad: &pad, ThemeID: &id, Sketch: &sketch}
	co := &d2lib.CompileOptions{
		Ruler:          r,
		LayoutResolver: func(string) (d2graph.LayoutGraph, error) { return d2talalayout.DefaultLayout, nil },
	}
	compiling.Lock()
	defer compiling.Unlock()
	d, _, err := d2lib.Compile(log.WithDefault(context.Background()), Preamble+src, co, ro)
	return d, ro, err
}

// renderBoards returns one SVG per board: a steps / layers / scenarios diagram
// used to be wrapped by d2animate, which flipped between near-identical frames.
// Static boards stacked in the page are readable; the only animation left is
// what an agent asks for with `style.animated: true` on an edge.
func renderBoards(src, theme string, sketch bool) ([][]byte, []string, error) {
	diagram, ro, err := compile(src, theme, sketch)
	if err != nil {
		return nil, nil, err
	}
	boards, names := boardsOf(diagram)
	out := make([][]byte, 0, len(boards))
	for _, b := range boards {
		svg, err := d2svg.Render(b, ro)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, sizeSVG(svg))
	}
	return out, names, nil
}

// sizeSVG copies the viewBox onto the root element's width / height. d2 leaves
// them off, and a browser showing a viewBox-only SVG in an <img> has no
// intrinsic size to work from, so it stretches it to the container.
var svgOpen = regexp.MustCompile(`<svg ([^>]*?)viewBox="0 0 ([0-9.]+) ([0-9.]+)"`)

func sizeSVG(b []byte) []byte {
	m := svgOpen.FindSubmatchIndex(b)
	if m == nil || bytes.Contains(b[m[0]:m[1]], []byte("width=")) {
		return b
	}
	size := fmt.Sprintf(` width="%s" height="%s"`, b[m[4]:m[5]], b[m[6]:m[7]])
	return append(b[:m[1]:m[1]], append([]byte(size), b[m[1]:]...)...)
}

// boardsOf: the boards to draw, with the label each one gets in the page.
func boardsOf(d *d2target.Diagram) ([]*d2target.Diagram, []string) {
	for _, set := range []struct {
		boards []*d2target.Diagram
		prefix string
	}{{d.Steps, "Step "}, {d.Layers, ""}, {d.Scenarios, ""}} {
		if len(set.boards) == 0 {
			continue
		}
		names := make([]string, len(set.boards))
		for i, b := range set.boards {
			names[i] = set.prefix + b.Name
			if b.Name == "" {
				names[i] = set.prefix + strconv.Itoa(i+1)
			}
		}
		return set.boards, names
	}
	return []*d2target.Diagram{d}, nil
}

// Boards names a diagram's boards, or nil when it is a plain one. Compiling is
// the only way to know, so the answer is kept per source.
func Boards(src string) []string {
	if v, ok := boardCache.Load(src); ok {
		return v.([]string)
	}
	var names []string
	if d, _, err := compile(src, "light", false); err == nil {
		_, names = boardsOf(d)
	}
	boardCache.Store(src, names)
	return names
}

var boardCache sync.Map // diagram source -> board names

// RenderASCII draws the diagram as text, so an agent can read its own diagram
// in the terminal. The d2 CLI switches to ELK for this; we keep TALA rather
// than embed a second layout engine.
func RenderASCII(src string) (string, error) {
	d, ro, err := compile(src, "light", false)
	if err != nil {
		return "", err
	}
	out, err := d2ascii.NewASCIIartist().Render(log.WithDefault(context.Background()), d, &d2ascii.RenderOpts{
		Scale: ro.Scale, Charset: charset.ASCII})
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// CheckD2 compiles a source and returns the compiler's own message, which is
// what `hudiff notes` hands back to the agent.
func CheckD2(src string) error {
	_, _, err := renderBoards(src, "light", false)
	if err != nil {
		return trimErr(err)
	}
	return nil
}

// trimErr rebases d2's line numbers onto the source the agent actually wrote:
// d2 counts from the top of Preamble+src, the agent never saw the preamble.
var preambleLines = strings.Count(Preamble, "\n")

var d2Pos = regexp.MustCompile(`(?m)^(\d+):\d+:`)

func trimErr(err error) error {
	msg := d2Pos.ReplaceAllStringFunc(strings.ReplaceAll(err.Error(), "\t", " "), func(m string) string {
		line, rest, _ := strings.Cut(m, ":")
		n, err := strconv.Atoi(line)
		if err != nil || n <= preambleLines {
			return m
		}
		return strconv.Itoa(n-preambleLines) + ":" + rest
	})
	return errString(msg)
}

type errString string

func (e errString) Error() string { return string(e) }
