package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kackerx/hudiff/internal/config"
	"github.com/kackerx/hudiff/internal/daemon"
	"github.com/kackerx/hudiff/internal/diff"
	"github.com/kackerx/hudiff/internal/session"
	"github.com/kackerx/hudiff/internal/source"
)

func (o *ReviewOpts) opts() source.Opts { return source.Opts{} }

func (c *WorkspaceCmd) Run(g *Globals) error {
	scope := source.Worktree
	switch {
	case c.Staged:
		scope = source.Staged
	case c.Unstaged:
		scope = source.Unstaged
	}
	o := c.opts()
	o.Pathspec = c.Pathspec
	src, err := source.FromWorktree(scope, o)
	if err != nil {
		return envErr("%v", err)
	}
	return c.create(g, src)
}

func (c *CommitCmd) Run(g *Globals) error {
	src, err := source.FromCommit(c.Rev, c.opts())
	if err != nil {
		return envErr("%v", err)
	}
	return c.create(g, src)
}

func (c *RangeCmd) Run(g *Globals) error {
	src, err := source.FromRange(c.Spec, c.opts())
	if err != nil {
		return envErr("%v", err)
	}
	return c.create(g, src)
}

func (c *PRCmd) Run(g *Globals) error {
	if source.PRNumber(c.Ref) == "" {
		return usage("%q is not a PR number, #number or URL", c.Ref)
	}
	src, err := source.FromPR(c.Ref, c.opts())
	if err != nil {
		return envErr("%v", err)
	}
	return c.create(g, src)
}

func (c *FilesCmd) Run(g *Globals) error {
	src, err := source.FromFiles(c.Left, c.Right, c.opts())
	if err != nil {
		return envErr("%v", err)
	}
	return c.create(g, src)
}

func (c *PatchCmd) Run(g *Globals) error {
	src, err := source.FromPatch(c.File, c.opts())
	if err != nil {
		return envErr("%v", err)
	}
	return c.create(g, src)
}

// create is the shared tail of the six entry commands: reuse or build the
// session, then hand it to the viewer.
func (o *ReviewOpts) create(g *Globals, src *source.Source) error {
	if o.TUI {
		return todo("--tui")
	}
	brief, err := buildBrief(o, src.Repo)
	if err != nil {
		return err
	}
	var existing *session.Session
	if !o.New {
		existing = session.FindByKey(session.KeyOf(src))
	}
	sid, dir, res, err := reuseOrCreate(existing, src, brief)
	if err != nil {
		return usage("%v", err)
	}
	url, err := daemon.SessionURL(sid, g.Port)
	if err != nil {
		return envErr("%v", err)
	}
	if o.JSON {
		return printJSON(createJSON(sid, dir, url, src, existing != nil, res))
	}
	printCreated(sid, dir, url, src, existing != nil, res)
	if !o.NoOpen && config.Load().Web.Open {
		if err := openBrowser(url); err != nil {
			fmt.Fprintf(os.Stderr, "hudiff: cannot open a browser: %v\n", err)
		}
	}
	return nil
}

func reuseOrCreate(existing *session.Session, src *source.Source, brief *session.Brief) (string, string, session.RefreshResult, error) {
	if existing == nil {
		sid, dir, err := session.New(src, brief)
		if err != nil {
			return "", "", session.RefreshResult{}, err
		}
		return sid, dir, session.RefreshResult{Files: diff.Load(dir).Files}, nil
	}
	sid, dir := existing.ID, session.Dir(existing.ID)
	res, err := session.ApplyRefresh(dir, src)
	if err != nil {
		return "", "", res, err
	}
	if err := updateBrief(dir, existing, brief); err != nil {
		return "", "", res, err
	}
	return sid, dir, res, os.WriteFile(session.CurrentFile(), []byte(sid), 0o644)
}

func updateBrief(dir string, sess *session.Session, brief *session.Brief) error {
	if brief == nil || sameBrief(sess.Brief, brief) {
		return nil
	}
	raw := map[string]any{}
	if err := json.Unmarshal(sess.Raw, &raw); err != nil {
		return err
	}
	raw["brief"] = brief
	return session.WriteJSON(filepath.Join(dir, "session.json"), raw)
}

func sameBrief(a, b *session.Brief) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

// buildBrief is the review request handed to the agent verbatim: preset files
// plus -p / -c. hudiff never interprets it.
func buildBrief(o *ReviewOpts, repo string) (*session.Brief, error) {
	b := &session.Brief{Presets: []session.Preset{}, Context: []string{}}
	for _, name := range []string{"default.md", filepath.Base(repo) + ".md"} {
		f := filepath.Join(config.PromptsDir(), name)
		if text, err := os.ReadFile(f); err == nil {
			b.Presets = append(b.Presets, session.Preset{Path: f, Text: strings.TrimSpace(string(text))})
		}
	}
	prompt := strings.TrimSpace(o.Prompt)
	if prompt == "-" {
		s, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, usage("cannot read stdin: %v", err)
		}
		prompt = strings.TrimSpace(string(s))
	}
	b.Prompt = prompt
	for _, c := range o.Context {
		p := expandHome(c)
		if !filepath.IsAbs(p) {
			p = filepath.Join(repo, p)
		}
		if _, err := os.Stat(p); err != nil {
			p += "  (missing)"
		}
		b.Context = append(b.Context, p)
	}
	if len(b.Presets) == 0 && b.Prompt == "" && len(b.Context) == 0 {
		return nil, nil
	}
	return b, nil
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

type createdOut struct {
	Session       string    `json:"session"`
	Dir           string    `json:"dir"`
	URL           string    `json:"url"`
	Kind          string    `json:"kind"`
	Title         string    `json:"title"`
	Reused        bool      `json:"reused"`
	Refreshed     bool      `json:"refreshed"`
	NotesDropped  []string  `json:"notes_dropped"`
	CommentsStale []string  `json:"comments_stale"`
	Meta          any       `json:"meta"`
	Files         []fileRow `json:"files"`
	Next          []string  `json:"next"`
}

func createJSON(sid, dir, url string, src *source.Source, reused bool, r session.RefreshResult) createdOut {
	out := createdOut{
		Session: sid, Dir: dir, URL: url, Kind: src.Kind, Title: src.Title,
		Reused: reused, Refreshed: r.Changed,
		NotesDropped: orEmpty(r.Dropped), CommentsStale: orEmpty(r.Stale), Meta: src.Meta,
		Files: []fileRow{},
	}
	for _, f := range r.Files {
		out.Files = append(out.Files, fileRow{f.Path, f.Status, f.Add, f.Del, len(f.Hunks)})
	}
	if reused {
		out.Next = append(out.Next, reuseNote(r))
	}
	out.Next = append(out.Next,
		fmt.Sprintf("hudiff -s %s read   # the diff with hunk tags", sid),
		fmt.Sprintf("hudiff -s %s notes <json>   # write the explanation", sid),
		fmt.Sprintf("hudiff -s %s wait   # block until the user comments", sid))
	return out
}

func reuseNote(r session.RefreshResult) string {
	s := "# session already existed, reused; "
	if !r.Changed {
		return s + "the diff is unchanged"
	}
	s += "the diff changed and was refreshed"
	if len(r.Dropped) > 0 {
		s += ", notes for these files are void and must be rewritten: " + strings.Join(r.Dropped, ", ")
	}
	return s
}

func printCreated(sid, dir, url string, src *source.Source, reused bool, r session.RefreshResult) {
	_, add, del, hunks := diff.Diff{Files: r.Files}.Totals()
	tag := ""
	if reused {
		tag = "  (existing, reused)"
	}
	fmt.Printf("session %s  %s%s\n", sid, src.Title, tag)
	if r.Changed && reused {
		fmt.Println("  " + strings.TrimPrefix(reuseNote(r), "# session already existed, reused; "))
		if len(r.Stale) > 0 {
			fmt.Println("  comments that lost their line: " + strings.Join(r.Stale, ", "))
		}
	}
	fmt.Printf("  %d files  +%d -%d  %d hunks\n  %s\n  %s\n", len(r.Files), add, del, hunks, url, dir)
}

func orEmpty(xs []string) []string {
	if xs == nil {
		return []string{}
	}
	return xs
}
