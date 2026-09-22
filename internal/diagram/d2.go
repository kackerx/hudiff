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

// The ruler is expensive to build and safe to share.
var ruler = sync.OnceValues(textmeasure.NewRuler)

func renderD2(src, theme string, sketch bool) ([]byte, error) {
	r, err := ruler()
	if err != nil {
		return nil, err
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
	diagram, _, err := d2lib.Compile(log.WithDefault(context.Background()), Preamble+src, co, ro)
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
