package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/kackerx/hudiff/internal/agents"
	"github.com/kackerx/hudiff/internal/config"
	"github.com/kackerx/hudiff/internal/session"
)

type AgentsCmd struct {
	List    AgentsListCmd    `cmd:"" default:"1" help:"List the agents running in this repo."`
	Deliver AgentsDeliverCmd `cmd:"" help:"Type a command into the terminal an agent is running in."`
}

type AgentsListCmd struct {
	JSON bool `help:"Machine-readable output."`
}

type AgentsDeliverCmd struct {
	PID  int    `arg:"" help:"Agent pid, from hudiff agents."`
	Text string `arg:"" help:"What to type, or - for stdin."`
}

func repoOf(g *Globals) (string, error) {
	_, dir, err := session.Current(g.Session)
	if err != nil {
		return "", usage("%v", err)
	}
	sess, err := session.LoadSession(dir)
	if err != nil {
		return "", usage("%v", err)
	}
	return sess.Repo, nil
}

func (c *AgentsListCmd) Run(g *Globals) error {
	repo, err := repoOf(g)
	if err != nil {
		return err
	}
	list := agents.InRepo(repo, config.Load().Agents.Names)
	if c.JSON {
		b, _ := json.MarshalIndent(list, "", " ")
		fmt.Println(string(b))
		return nil
	}
	if len(list) == 0 {
		fmt.Printf("(no agent running under %s)\n", repo)
		return nil
	}
	for _, a := range list {
		fmt.Printf("  %-22s %-9s %-16s pid %-7d %s\n", a.Name, a.Kind, a.Branch, a.PID, a.CWD)
	}
	return nil
}

func (c *AgentsDeliverCmd) Run(g *Globals) error {
	text, err := readTextArg(c.Text)
	if err != nil {
		return err
	}
	d := agents.Deliver(c.PID, strings.TrimSpace(text))
	if d.Via == "copy" {
		fmt.Fprintf(os.Stderr, "hudiff: nothing to deliver through (%s)\n", d.Detail)
		fmt.Println(strings.TrimSpace(text))
		return &ExitError{Code: 1}
	}
	fmt.Printf("sent via %s (%s)\n", d.Via, d.Detail)
	return nil
}
