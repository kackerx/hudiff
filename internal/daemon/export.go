package daemon

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/kackerx/hudiff/internal/config"
	"github.com/kackerx/hudiff/internal/diagram"
	"github.com/kackerx/hudiff/internal/diff"
	"github.com/kackerx/hudiff/internal/session"
)

var (
	cssLink  = regexp.MustCompile(`<link rel="stylesheet"(?: id="([^"]+)")? href="/([^"]+)"[^>]*>`)
	jsTag    = regexp.MustCompile(`<script src="/([^"]+)"></script>`)
	fontFace = regexp.MustCompile(`@font-face\{[^}]*\}`)
)

// ExportHTML is the session page as one self-contained file: assets inlined,
// the session payload and every diagram (light theme) baked in, nothing that
// writes. It needs no daemon and requests nothing once opened.
func ExportHTML(dir string, font bool) ([]byte, error) {
	sess, err := session.LoadSession(dir)
	if err != nil {
		return nil, err
	}
	p := payloadOf(dir)
	p.Comments = json.RawMessage("[]")
	d, cfg := diff.Load(dir), diagram.DefaultConfig()
	svg := map[string]string{}
	for _, e := range p.Diagrams {
		if len(e.Boards) > 1 {
			for i := range e.Boards {
				svg[e.ID+"/"+strconv.Itoa(i)] = string(diagram.Render(dir, sess.Repo, d, e.ID, "light", cfg, i))
			}
		} else {
			svg[e.ID] = string(diagram.Render(dir, sess.Repo, d, e.ID, "light", cfg, 0))
		}
	}
	// json.Marshal escapes < and >, so nothing in here can close the <script>.
	data, err := json.Marshal(map[string]any{"session": p, "svg": svg, "keymap": config.Load().MergedKeymap()})
	if err != nil {
		return nil, err
	}

	html := string(page("session.html"))
	html = strings.Replace(html, `href="/favicon.svg"`,
		`href="data:image/svg+xml;base64,`+base64.StdEncoding.EncodeToString(asset("favicon.svg"))+`"`, 1)
	html = cssLink.ReplaceAllStringFunc(html, func(tag string) string {
		m := cssLink.FindStringSubmatch(tag)
		css := string(asset(m[2]))
		if m[2] == "app.css" {
			if font {
				css = strings.Replace(css, `url("/vendor/JetBrainsMono.ttf")`,
					`url("data:font/ttf;base64,`+base64.StdEncoding.EncodeToString(asset("vendor/JetBrainsMono.ttf"))+`")`, 1)
			} else {
				css = fontFace.ReplaceAllString(css, "") // falls back to the system monospace
			}
		}
		id := ""
		if m[1] != "" {
			id = ` id="` + m[1] + `"`
		}
		return "<style" + id + ">" + css + "</style>"
	})
	html = jsTag.ReplaceAllStringFunc(html, func(tag string) string {
		name := jsTag.FindStringSubmatch(tag)[1]
		js := strings.ReplaceAll(string(asset(name)), "</script", `<\/script`)
		if name == "app.js" {
			return "<script>window.HUDIFF_STATIC=" + string(data) + "</script>\n<script>" + js + "</script>"
		}
		return "<script>" + js + "</script>"
	})
	if m := regexp.MustCompile(`<(link|script)[^>]* (href|src)="/[^"]*"`).FindString(html); m != "" {
		return nil, fmt.Errorf("export still points at the daemon: %s", m)
	}
	return []byte(html), nil
}
