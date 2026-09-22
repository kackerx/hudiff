package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/kackerx/hudiff/internal/diagram"
	"github.com/kackerx/hudiff/internal/diff"
	"github.com/kackerx/hudiff/internal/session"
)

func (c *DiagramCmd) Run(g *Globals) error {
	_, dir, err := session.Current(g.Session)
	if err != nil {
		return usage("%v", err)
	}
	sess, err := session.LoadSession(dir)
	if err != nil {
		return usage("%v", err)
	}
	d, cfg := diff.Load(dir), diagram.DefaultConfig()
	if c.Which == "" || c.Which == "list" {
		return listDiagrams(dir, d, cfg)
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

func listDiagrams(dir string, d diff.Diff, cfg diagram.Config) error {
	for _, dg := range diagram.List(dir, d, cfg) {
		line := fmt.Sprintf("%-12s [%s] %s", dg.ID, dg.Kind, dg.Title)
		if len(dg.Hunks) > 0 {
			line += "  → " + strings.Join(dg.Hunks, ", ")
		}
		fmt.Println(line)
	}
	return nil
}
