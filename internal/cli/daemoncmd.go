package cli

import (
	"fmt"

	"github.com/kackerx/hudiff/internal/daemon"
)

type DaemonCmd struct {
	Serve  DaemonServeCmd  `cmd:"" hidden:"" help:"(internal) run the server in the foreground."`
	Start  DaemonStartCmd  `cmd:"" help:"Start the daemon if it is not running."`
	Stop   DaemonStopCmd   `cmd:"" help:"Stop the daemon; session data is kept."`
	Status DaemonStatusCmd `cmd:"" help:"Print URL, port and pid."`
}

type DaemonServeCmd struct{}

func (c *DaemonServeCmd) Run(g *Globals) error {
	if err := daemon.Serve(g.Port); err != nil {
		return envErr("%v", err)
	}
	return nil
}

type DaemonStartCmd struct{}

func (c *DaemonStartCmd) Run(g *Globals) error {
	port, err := daemon.Ensure(g.Port)
	if err != nil {
		return envErr("%v", err)
	}
	fmt.Printf("http://127.0.0.1:%d/\n", port)
	return nil
}

type DaemonStopCmd struct{}

func (c *DaemonStopCmd) Run(g *Globals) error {
	fmt.Println(daemon.Stop())
	return nil
}

type DaemonStatusCmd struct {
	JSON bool `help:"Machine-readable output."`
}

func (c *DaemonStatusCmd) Run(g *Globals) error {
	info := daemon.ReadInfo()
	if c.JSON {
		return printJSON(map[string]any{
			"running": info.Alive(), "port": info.Port, "pid": info.PID,
			"started": info.Started, "url": fmt.Sprintf("http://127.0.0.1:%d/", info.Port),
		})
	}
	fmt.Println(daemon.StatusLine())
	if info.Alive() {
		fmt.Printf("  port %d  pid %d  started %s\n", info.Port, info.PID, info.Started)
	}
	return nil
}
