// hudiff — diffs for humans. See specs/spec.md for the CLI surface and the
// on-disk contract.
package main

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/kackerx/hudiff/internal/cli"
	"github.com/kackerx/hudiff/internal/daemon"
)

//go:embed web skills
var assets embed.FS

// Set with -ldflags -X main.version by a release build; otherwise taken from
// the build info, so `go install .` and `go install …@v1.2.3` both say something.
var version = "dev"

func buildVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if version != "dev" || !ok {
		return version
	}
	rev, dirty := "", ""
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				dirty = "-dirty"
			}
		}
	}
	if rev != "" { // built from the checkout: the commit is more useful than v0.0.0-…
		return rev[:min(7, len(rev))] + dirty
	}
	if v := bi.Main.Version; v != "" && v != "(devel)" {
		return v // `go install …@v1.2.3` has no vcs stamp, just the tag
	}
	return version
}

func main() {
	daemon.SetAssets(assets)
	cli.SetSkills(assets)
	v := buildVersion()
	ctx, c := cli.Parse(v)
	err := ctx.Run(&c.Globals, cli.Version(v))
	if err == nil {
		return
	}
	var ex *cli.ExitError
	if errors.As(err, &ex) {
		if ex.Msg != "" { // `wait` already said why it is exiting 124
			fmt.Fprintf(os.Stderr, "hudiff: %s\n", ex.Msg)
		}
		os.Exit(ex.Code)
	}
	fmt.Fprintf(os.Stderr, "hudiff: %v\n", err)
	os.Exit(1)
}
