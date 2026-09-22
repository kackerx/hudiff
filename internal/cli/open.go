package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/kackerx/hudiff/internal/browser"
	"github.com/kackerx/hudiff/internal/daemon"
	"github.com/kackerx/hudiff/internal/session"
)

type OpenCmd struct {
	Target string `arg:"" optional:"" help:"Session id or id prefix; default = most recent." placeholder:"<session>"`
	NoOpen bool   `name:"no-open" help:"Print the URL without launching the viewer."`
	JSON   bool   `help:"Machine-readable output (implies --no-open)."`
	TUI    bool   `name:"tui" help:"Use the TUI viewer (not implemented yet)."`
}

func (c *OpenCmd) Run(g *Globals) error {
	if c.TUI {
		return todo("--tui")
	}
	sid, err := c.resolve(g)
	if err != nil {
		return err
	}
	url, err := daemon.SessionURL(sid, g.Port)
	if err != nil {
		return envErr("%v", err)
	}
	if c.JSON {
		return printJSON(map[string]any{"session": sid, "dir": session.Dir(sid), "url": url})
	}
	fmt.Println(url)
	if !c.NoOpen {
		if err := browser.Open(url); err != nil {
			fmt.Fprintf(os.Stderr, "hudiff: cannot open a browser: %v\n", err)
		}
	}
	return nil
}

func (c *OpenCmd) resolve(g *Globals) (string, error) {
	want := c.Target
	if want == "" {
		want = g.Session
	}
	if want != "" {
		if _, err := os.Stat(session.Dir(want)); err == nil {
			return want, nil
		}
		return matchPrefix(want)
	}
	sid, _, err := session.Current("")
	if err != nil {
		return "", usage("%v", err)
	}
	return sid, nil
}

func matchPrefix(prefix string) (string, error) {
	ids, err := sessionIDs()
	if err != nil {
		return "", usage("no such session: %s", prefix)
	}
	var hits []string
	for _, id := range ids {
		if strings.HasPrefix(id, prefix) || strings.HasSuffix(id, prefix) {
			hits = append(hits, id)
		}
	}
	switch len(hits) {
	case 0:
		return "", usage("no such session: %s", prefix)
	case 1:
		return hits[0], nil
	}
	return "", usage("session prefix %s matches several: %s", prefix, strings.Join(hits, ", "))
}
