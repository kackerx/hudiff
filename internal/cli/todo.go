package cli

import "fmt"

// Everything below is the M1 skeleton: the flags are the real spec §3 surface
// so `hudiff -h` is already the agent's manual, the bodies land in M2–M5.

type WorkspaceCmd struct {
	ReviewOpts
	Staged   bool     `help:"Only staged changes." xor:"scope"`
	Unstaged bool     `help:"Only unstaged changes." xor:"scope"`
	Pathspec []string `arg:"" optional:"" help:"Limit to these paths." placeholder:"<pathspec>"`
}

type CommitCmd struct {
	ReviewOpts
	Rev string `arg:"" optional:"" default:"HEAD" help:"Commit to show." placeholder:"<rev>"`
}

type RangeCmd struct {
	ReviewOpts
	Spec string `arg:"" help:"<a>..<b>, <a>...<b>, or <a>." placeholder:"<range>"`
}

type PRCmd struct {
	ReviewOpts
	Ref string `arg:"" help:"PR number, #number or URL." placeholder:"<n|#n|url>"`
}

type PatchCmd struct {
	ReviewOpts
	File string `arg:"" optional:"" default:"-" help:"Patch file, or - for stdin." placeholder:"<file|->"`
}

type ContextCmd struct {
	File string `short:"f" help:"Only this path." placeholder:"<path>"`
	Refs int    `default:"6" help:"Max references listed per symbol."`
}

type NotesCmd struct {
	Path  string `arg:"" help:"notes.json path, or - for stdin." placeholder:"<file|->"`
	Force bool   `help:"Write even if validation fails."`
}

type DiagramCmd struct {
	Which string `arg:"" optional:"" help:"list, auto-map, or a diagram id." placeholder:"<id>"`
	Src   bool   `help:"Print the diagram source instead of rendering."`
	ASCII bool   `name:"ascii" help:"Render as text, readable in a terminal."`
	Theme string `default:"light" enum:"light,dark" help:"Render theme."`
	Out   string `short:"o" help:"Write the SVG here instead of stdout." placeholder:"<svg>"`
}

type ReviewCmd struct {
	Show    ReviewShowCmd     `cmd:"" default:"1" help:"Show the report."`
	Set     ReviewSetCmd      `cmd:"" help:"Write a report; merges into the existing one by default."`
	Done    ReviewDoneCmd     `cmd:"" help:"Mark an item handled."`
	Wontfix ReviewWontfixCmd  `cmd:"" help:"Mark an item wontfix."`
	Reopen  ReviewReopenCmd   `cmd:"" help:"Reopen an item."`
	Note    ReviewNoteCmd     `cmd:"" help:"Append a re-review note to an item."`
	MD      ReviewMarkdownCmd `cmd:"" name:"md" help:"Export the report as markdown."`
}

type ReviewShowCmd struct {
	Open bool `help:"Only open items."`
	JSON bool `help:"Machine-readable output."`
}

type ReviewSetCmd struct {
	Path    string `arg:"" help:"Report JSON, or - for stdin." placeholder:"<file|->"`
	Author  string `help:"Reviewer identity." placeholder:"<name>"`
	Replace bool   `help:"Replace the whole report instead of merging."`
	Force   bool   `help:"Write even if validation fails."`
}

// kong resolves Run() on the node it built, so the three verbs are three
// structs rather than one shared one.
type ReviewDoneCmd struct {
	ID   string `arg:"" help:"Review item id."`
	Text string `arg:"" optional:"" help:"Resolution, or - for stdin."`
	By   string `default:"agent" help:"Who resolved it."`
}

type ReviewWontfixCmd struct {
	ID   string `arg:"" help:"Review item id."`
	Text string `arg:"" optional:"" help:"Reason, or - for stdin."`
	By   string `default:"agent" help:"Who decided."`
}

type ReviewReopenCmd struct {
	ID   string `arg:"" help:"Review item id."`
	Text string `arg:"" optional:"" help:"Unused; kept for symmetry."`
	By   string `default:"agent" help:"Who reopened it."`
}

type ReviewNoteCmd struct {
	ID      string `arg:"" help:"Review item id."`
	Text    string `arg:"" help:"Note body, or - for stdin."`
	Verdict string `default:"info" enum:"ok,disagree,info" help:"ok / disagree / info."`
	By      string `default:"reviewer" help:"Who is writing."`
}

type ReviewMarkdownCmd struct{}

type CommentsCmd struct {
	New  bool `help:"Only comments sent to the agent and not yet replied to."`
	All  bool `help:"Include resolved comments."`
	JSON bool `help:"Machine-readable output."`
}

type WaitCmd struct {
	Timeout float64 `default:"1800" help:"Seconds; exit 124 on timeout."`
	JSON    bool    `help:"Machine-readable output."`
}

type ReplyCmd struct {
	ID      string `arg:"" help:"Comment id."`
	Text    string `arg:"" help:"Reply body, or - for stdin."`
	Resolve bool   `help:"Also mark it handled, awaiting confirmation."`
}

type ResolveCmd struct {
	IDs    []string `arg:"" help:"Comment ids."`
	Reopen bool     `help:"Reopen instead."`
	By     string   `default:"agent" help:"agent or user."`
}

type SkillCmd struct {
	Path    SkillPathCmd    `cmd:"" default:"1" help:"Print the skill path, materialising it if needed."`
	Install SkillInstallCmd `cmd:"" help:"Install the skill for an agent."`
}

type ConfigCmd struct {
	Path ConfigPathCmd `cmd:"" default:"1" help:"Print the config file path."`
	Show ConfigShowCmd `cmd:"" help:"Print the effective config."`
	Init ConfigInitCmd `cmd:"" help:"Write a config file with the defaults."`
}

type VersionCmd struct{}

func (c *VersionCmd) Run(g *Globals, v Version) error {
	fmt.Println(string(v))
	return nil
}

type Version string
