// hudiff — diffs for humans. See specs/spec.md for the CLI surface and the
// on-disk contract shared with legacy/hudiff.py.
package main

import (
	"embed"
	"errors"
	"fmt"
	"os"

	"github.com/kackerx/hudiff/internal/cli"
	"github.com/kackerx/hudiff/internal/daemon"
)

//go:embed web skills
var assets embed.FS

var version = "dev"

func main() {
	daemon.SetAssets(assets)
	ctx, c := cli.Parse(version)
	err := ctx.Run(&c.Globals, cli.Version(version))
	if err == nil {
		return
	}
	var ex *cli.ExitError
	if errors.As(err, &ex) {
		fmt.Fprintf(os.Stderr, "hudiff: %s\n", ex.Msg)
		os.Exit(ex.Code)
	}
	fmt.Fprintf(os.Stderr, "hudiff: %v\n", err)
	os.Exit(1)
}
