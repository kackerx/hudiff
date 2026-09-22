// Package cli is the kong command surface (specs/spec.md §3). Commands only
// parse arguments and print; the work lives in internal/*.
package cli

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
)

type Globals struct {
	Session string `short:"s" help:"Session id; default = most recent in this repo." placeholder:"<id>"`
	Port    int    `help:"Daemon port; only used when the daemon has to be started." placeholder:"<n>"`
}

// ReviewOpts are shared by every command that creates a session.
type ReviewOpts struct {
	// Web names the only viewer there is; --tui is what actually branches.
	Web     bool     `help:"Force the web viewer (the default)." xor:"viewer"`
	TUI     bool     `name:"tui" help:"Force the TUI viewer (not implemented yet)." xor:"viewer"`
	NoOpen  bool     `name:"no-open" help:"Create/refresh the session and print its id only."`
	JSON    bool     `help:"Print session info as JSON (implies --no-open)."`
	New     bool     `help:"Do not reuse an existing session with the same source key."`
	Prompt  string   `short:"p" help:"Review request, passed to the agent verbatim ('-' reads stdin)." placeholder:"<text|->"`
	Context []string `short:"c" help:"Context file for the agent; repeatable." placeholder:"<file>"`
}

type CLI struct {
	Globals

	Workspace WorkspaceCmd `cmd:"" default:"withargs" help:"Working tree vs HEAD, staged included (the default, no verb)."`
	Commit    CommitCmd    `cmd:"" help:"A single commit against its first parent."`
	Range     RangeCmd     `cmd:"" help:"A range: <a>..<b>, <a>...<b>, or just <a> vs the working tree."`
	PR        PRCmd        `cmd:"" name:"pr" help:"A GitHub pull request."`
	Patch     PatchCmd     `cmd:"" help:"A patch file or stdin."`
	Open      OpenCmd      `cmd:"" help:"Reopen an existing session in the viewer."`

	Session  SessionCmd  `cmd:"" help:"Manage sessions."`
	Read     ReadCmd     `cmd:"" help:"Print the diff with [file#hunk] tags and both line-number columns."`
	Context  ContextCmd  `cmd:"" help:"Repo-level context: recent history, changed symbols, callers, tests."`
	Notes    NotesCmd    `cmd:"" help:"Write the agent's per-hunk explanation."`
	Diagram  DiagramCmd  `cmd:"" help:"List, dump or render diagrams."`
	Review   ReviewCmd   `cmd:"" help:"Cross-review report."`
	Comments CommentsCmd `cmd:"" help:"List comments."`
	Wait     WaitCmd     `cmd:"" help:"Block until the user hits Send, then print the new comments."`
	Reply    ReplyCmd    `cmd:"" help:"Reply to a comment."`
	Resolve  ResolveCmd  `cmd:"" help:"Mark a comment handled, awaiting the user's confirmation."`

	Daemon  DaemonCmd  `cmd:"" help:"The single local server that owns every session."`
	Skill   SkillCmd   `cmd:"" help:"The agent skill shipped with hudiff."`
	Config  ConfigCmd  `cmd:"" help:"Configuration file."`
	Version VersionCmd `cmd:"" help:"Print the version."`
}

// ExitError carries spec §3 exit codes: 1 usage/validation, 2 environment,
// 124 wait timeout.
type ExitError struct {
	Code int
	Msg  string
}

func (e *ExitError) Error() string { return e.Msg }

func usage(format string, a ...any) error {
	return &ExitError{Code: 1, Msg: fmt.Sprintf(format, a...)}
}

func envErr(format string, a ...any) error {
	return &ExitError{Code: 2, Msg: fmt.Sprintf(format, a...)}
}

func todo(what string) error {
	return &ExitError{Code: 1, Msg: what + ": not implemented yet"}
}

func Parse(version string) (*kong.Context, *CLI) {
	cli := &CLI{}
	parser := kong.Must(cli,
		kong.Name("hudiff"),
		kong.Description("diffs for humans — an agent explains a diff, you comment on it line by line in the browser, the comments flow back."),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{Compact: true, FlagsLast: true}),
		kong.Vars{"version": version},
		// kong exits 80 on a usage error; spec §3 says 1.
		kong.Exit(func(code int) {
			if code != 0 {
				code = 1
			}
			os.Exit(code)
		}),
	)
	ctx, err := parser.Parse(os.Args[1:])
	parser.FatalIfErrorf(err)
	return ctx, cli
}
