package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/kackerx/hudiff/internal/daemon"
	"github.com/kackerx/hudiff/internal/diagram"
	"github.com/kackerx/hudiff/internal/diff"
	"github.com/kackerx/hudiff/internal/session"
)

func (c *DiagramCmd) Run(g *Globals) error {
	sid, dir, err := session.Current(g.Session)
	if err != nil {
		return usage("%v", err)
	}
	sess, err := session.LoadSession(dir)
	if err != nil {
		return usage("%v", err)
	}
	d, cfg := diff.Load(dir), diagram.ConfigFor(sess.Meta)
	if c.Which == "" || c.Which == "list" {
		return listDiagrams(sid, dir, d, cfg)
	}
	if c.Src {
		src, _, err := diagram.Source(dir, sess.Repo, d, c.Which, cfg)
		if err != nil {
			return usage("%v", err)
		}
		fmt.Print(src)
		return nil
	}
	svg := diagram.Render(dir, sess.Repo, d, c.Which, c.Theme, cfg)
	if c.Out == "" {
		_, err = os.Stdout.Write(svg)
		return err
	}
	if err := os.WriteFile(c.Out, svg, 0o644); err != nil {
		return usage("%v", err)
	}
	fmt.Printf("wrote %s (%d bytes)\n", c.Out, len(svg))
	return nil
}

func listDiagrams(sid, dir string, d diff.Diff, cfg diagram.Config) error {
	errs := renderErrors(sid)
	for _, dg := range diagram.List(dir, d, cfg) {
		line := fmt.Sprintf("%-12s [%s] %s", dg.ID, dg.Kind, dg.Title)
		if len(dg.Hunks) > 0 {
			line += "  → " + strings.Join(dg.Hunks, ", ")
		}
		if e := errs[dg.ID]; e != "" {
			line += "\n             render error: " + e
		}
		fmt.Println(line)
	}
	return nil
}

// renderErrors asks the daemon what the browser could not draw. mermaid is
// rendered client side, so this is the only way an agent hears about it.
func renderErrors(sid string) map[string]string {
	info := daemon.ReadInfo()
	if !info.Alive() {
		return nil
	}
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/s/%s/api/diagram-errors", info.Port, sid))
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	out := map[string]string{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out
}
