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

func (c *WorkspaceCmd) Run(g *Globals) error { return todo("hudiff (workspace diff)") }

type CommitCmd struct {
	ReviewOpts
	Rev string `arg:"" optional:"" default:"HEAD" help:"Commit to show." placeholder:"<rev>"`
}

func (c *CommitCmd) Run(g *Globals) error { return todo("commit") }

type RangeCmd struct {
	ReviewOpts
	Spec string `arg:"" help:"<a>..<b>, <a>...<b>, or <a>." placeholder:"<range>"`
}

func (c *RangeCmd) Run(g *Globals) error { return todo("range") }

type PRCmd struct {
	ReviewOpts
	Ref string `arg:"" help:"PR number, #number or URL." placeholder:"<n|#n|url>"`
}

func (c *PRCmd) Run(g *Globals) error { return todo("pr") }

type PatchCmd struct {
	ReviewOpts
	File string `arg:"" optional:"" default:"-" help:"Patch file, or - for stdin." placeholder:"<file|->"`
}

func (c *PatchCmd) Run(g *Globals) error { return todo("patch") }

type ContextCmd struct {
	File string `short:"f" help:"Only this path." placeholder:"<path>"`
	Refs int    `default:"6" help:"Max references listed per symbol."`
}

func (c *ContextCmd) Run(g *Globals) error { return todo("context") }

type NotesCmd struct {
	Path  string `arg:"" help:"notes.json path, or - for stdin." placeholder:"<file|->"`
	Force bool   `help:"Write even if validation fails."`
}

func (c *NotesCmd) Run(g *Globals) error { return todo("notes") }

type DiagramCmd struct {
	Which string `arg:"" optional:"" help:"list, auto-map, or a diagram id." placeholder:"<id>"`
	Src   bool   `help:"Print the diagram source instead of rendering."`
	Theme string `default:"light" enum:"light,dark" help:"Render theme."`
	Out   string `short:"o" help:"Write the SVG here instead of stdout." placeholder:"<svg>"`
}

func (c *DiagramCmd) Run(g *Globals) error { return todo("diagram") }

type ReviewCmd struct {
	Show    ReviewShowCmd     `cmd:"" default:"1" help:"Show the report."`
	Set     ReviewSetCmd      `cmd:"" help:"Write a report; merges into the existing one by default."`
	Done    ReviewStatusCmd   `cmd:"" help:"Mark an item handled."`
	Wontfix ReviewStatusCmd   `cmd:"" help:"Mark an item wontfix."`
	Reopen  ReviewStatusCmd   `cmd:"" help:"Reopen an item."`
	Note    ReviewNoteCmd     `cmd:"" help:"Append a re-review note to an item."`
	MD      ReviewMarkdownCmd `cmd:"" name:"md" help:"Export the report as markdown."`
}

type ReviewShowCmd struct {
	Open bool `help:"Only open items."`
	JSON bool `help:"Machine-readable output."`
}

func (c *ReviewShowCmd) Run(g *Globals) error { return todo("review") }

type ReviewSetCmd struct {
	Path    string `arg:"" help:"Report JSON, or - for stdin." placeholder:"<file|->"`
	Author  string `help:"Reviewer identity." placeholder:"<name>"`
	Replace bool   `help:"Replace the whole report instead of merging."`
	Force   bool   `help:"Write even if validation fails."`
}

func (c *ReviewSetCmd) Run(g *Globals) error { return todo("review set") }

type ReviewStatusCmd struct {
	ID   string `arg:"" help:"Review item id."`
	Text string `arg:"" optional:"" help:"Resolution or reason, or - for stdin."`
	By   string `default:"agent" help:"Who resolved it."`
}

func (c *ReviewStatusCmd) Run(g *Globals) error { return todo("review done/wontfix/reopen") }

type ReviewNoteCmd struct {
	ID      string `arg:"" help:"Review item id."`
	Text    string `arg:"" help:"Note body, or - for stdin."`
	Verdict string `default:"info" enum:"ok,disagree,info" help:"ok / disagree / info."`
	By      string `default:"reviewer" help:"Who is writing."`
}

func (c *ReviewNoteCmd) Run(g *Globals) error { return todo("review note") }

type ReviewMarkdownCmd struct{}

func (c *ReviewMarkdownCmd) Run(g *Globals) error { return todo("review md") }

type CommentsCmd struct {
	New  bool `help:"Only comments sent to the agent and not yet replied to."`
	All  bool `help:"Include resolved comments."`
	JSON bool `help:"Machine-readable output."`
}

func (c *CommentsCmd) Run(g *Globals) error { return todo("comments") }

type WaitCmd struct {
	Timeout float64 `default:"1800" help:"Seconds; exit 124 on timeout."`
	JSON    bool    `help:"Machine-readable output."`
}

func (c *WaitCmd) Run(g *Globals) error { return todo("wait") }

type ReplyCmd struct {
	ID      string `arg:"" help:"Comment id."`
	Text    string `arg:"" help:"Reply body, or - for stdin."`
	Resolve bool   `help:"Also mark it handled, awaiting confirmation."`
}

func (c *ReplyCmd) Run(g *Globals) error { return todo("reply") }

type ResolveCmd struct {
	IDs    []string `arg:"" help:"Comment ids."`
	Reopen bool     `help:"Reopen instead."`
	By     string   `default:"agent" help:"agent or user."`
}

func (c *ResolveCmd) Run(g *Globals) error { return todo("resolve") }

type SkillCmd struct {
	Path    SkillPathCmd    `cmd:"" default:"1" help:"Print the skill path, materialising it if needed."`
	Install SkillInstallCmd `cmd:"" help:"Install the skill for an agent."`
}

type SkillPathCmd struct{}

func (c *SkillPathCmd) Run(g *Globals) error { return todo("skill path") }

type SkillInstallCmd struct {
	For string `default:"all" enum:"claude,pi,opencode,codex,all" help:"Which agent to install for."`
}

func (c *SkillInstallCmd) Run(g *Globals) error { return todo("skill install") }

type ConfigCmd struct {
	Path ConfigPathCmd `cmd:"" default:"1" help:"Print the config file path."`
	Show ConfigShowCmd `cmd:"" help:"Print the effective config."`
	Init ConfigInitCmd `cmd:"" help:"Write a config file with the defaults."`
}

type ConfigPathCmd struct{}

func (c *ConfigPathCmd) Run(g *Globals) error { return todo("config path") }

type ConfigShowCmd struct{}

func (c *ConfigShowCmd) Run(g *Globals) error { return todo("config show") }

type ConfigInitCmd struct{}

func (c *ConfigInitCmd) Run(g *Globals) error { return todo("config init") }

type VersionCmd struct{}

func (c *VersionCmd) Run(g *Globals, v Version) error {
	fmt.Println(string(v))
	return nil
}

type Version string
