// d2 is compiled in as a library (spec §7): no external binary, layout fixed
// to TALA. Nothing outside this package may import d2.
package diagram

import (
	"context"
	"strings"
	"sync"

	"github.com/d2lang/d2/d2graph"
	"github.com/d2lang/d2/d2layouts/d2talalayout"
	"github.com/d2lang/d2/d2lib"
	"github.com/d2lang/d2/d2renderers/d2animate"
	"github.com/d2lang/d2/d2renderers/d2ascii"
	"github.com/d2lang/d2/d2renderers/d2ascii/charset"
	"github.com/d2lang/d2/d2renderers/d2svg"
	"github.com/d2lang/d2/d2target"
	"github.com/d2lang/d2/lib/log"
	"github.com/d2lang/d2/lib/textmeasure"
)

// Preamble is hudiff's house style, prepended to every source. The glob reaches
// every level and every board; an agent's own border-radius overrides it.
const Preamble = "**.style.border-radius: 8\n"

// animateMS matches the oracle's --animate-interval for steps diagrams.
const animateMS = 1400

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

func renderD2(src, theme string, sketch bool) ([]byte, error) {
	diagram, ro, err := compile(src, theme, sketch)
	if err != nil {
		return nil, err
	}
	if !multiboard(diagram) {
		return d2svg.Render(diagram, ro)
	}
	master, err := diagram.HashID(ro.Salt)
	if err != nil {
		return nil, err
	}
	ro.MasterID = master
	boards, err := d2svg.RenderMultiboard(diagram, ro)
	if err != nil || len(boards) == 0 {
		return nil, err
	}
	return d2animate.Wrap(diagram, boards, *ro, animateMS)
}

func multiboard(d *d2target.Diagram) bool {
	return len(d.Steps)+len(d.Layers)+len(d.Scenarios) > 0
}

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
	_, err := renderD2(src, "light", false)
	if err != nil {
		return trimErr(err)
	}
	return nil
}

// trimErr drops the preamble line number offset from d2's message: the agent
// never saw that line.
func trimErr(err error) error {
	return errString(strings.ReplaceAll(err.Error(), "\t", " "))
}

type errString string

func (e errString) Error() string { return string(e) }
