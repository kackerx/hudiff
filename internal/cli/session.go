package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kackerx/hudiff/internal/daemon"
	"github.com/kackerx/hudiff/internal/diff"
	"github.com/kackerx/hudiff/internal/session"
	"github.com/kackerx/hudiff/internal/source"
)

type SessionCmd struct {
	List    SessionListCmd    `cmd:"" help:"List sessions."`
	Info    SessionInfoCmd    `cmd:"" help:"Print session info."`
	URL     SessionURLCmd     `cmd:"" name:"url" help:"Print the session URL, starting the daemon if needed."`
	Refresh SessionRefreshCmd `cmd:"" help:"Re-read the diff and re-anchor comments."`
	Archive SessionArchiveCmd `cmd:"" help:"Archive a session; the data and link stay."`
	Rm      SessionRmCmd      `cmd:"" help:"Delete sessions, comments included."`
}

type SessionListCmd struct {
	All bool `help:"Include archived sessions."`
}

func (c *SessionListCmd) Run(g *Globals) error {
	info := daemon.ReadInfo()
	alive := info.Alive()
	fmt.Println(daemon.StatusLine())
	ids := session.IDs()
	if len(ids) == 0 {
		fmt.Println("(no sessions)")
		return nil
	}
	cur := currentID()
	hidden := 0
	for _, id := range ids {
		sess, err := session.LoadSession(session.Dir(id))
		if err != nil || sess.ID == "" {
			continue
		}
		if sess.IsArchived() && !c.All {
			hidden++
			continue
		}
		fmt.Println(sessionRowLine(sess, cur))
		if alive {
			fmt.Printf("      http://127.0.0.1:%d/s/%s/\n", info.Port, sess.ID)
		}
	}
	if hidden > 0 {
		fmt.Printf("(%d more archived; --all to see them)\n", hidden)
	}
	return nil
}

func sessionRowLine(sess *session.Session, cur string) string {
	mark := " "
	if sess.ID == cur {
		mark = "*"
	}
	cs := session.Comments(session.Dir(sess.ID))
	sent := 0
	for _, c := range cs {
		if c["status"] == "sent" {
			sent++
		}
	}
	tail := ""
	if sent > 0 {
		tail = fmt.Sprintf("  %d pending", sent)
	}
	if sess.IsArchived() {
		tail += "  [archived]"
	}
	return fmt.Sprintf("%s %s  [%s] %s  %d comments%s  %s",
		mark, sess.ID, sess.Kind, padRunes(sess.Title, 40), len(cs), tail, filepath.Base(sess.Repo))
}

// padRunes matches Python's `{s[:40]:<40}`: slice and pad by characters.
func padRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}
	return string(r) + strings.Repeat(" ", n-len(r))
}

func currentID() string {
	b, err := os.ReadFile(session.CurrentFile())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

type SessionInfoCmd struct {
	JSON bool `help:"Machine-readable output."`
}

// fileRow is the per-file summary both `session info --json` and the create
// commands print.
type fileRow struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Add    int    `json:"add"`
	Del    int    `json:"del"`
	Hunks  int    `json:"hunks"`
}

type infoOut struct {
	Session         string          `json:"session"`
	Dir             string          `json:"dir"`
	URL             string          `json:"url"`
	Kind            string          `json:"kind"`
	Title           string          `json:"title"`
	Repo            string          `json:"repo"`
	Meta            json.RawMessage `json:"meta"`
	Brief           json.RawMessage `json:"brief"`
	Archived        bool            `json:"archived"`
	Files           []fileRow       `json:"files"`
	HunksTotal      int             `json:"hunks_total"`
	HunksNoted      int             `json:"hunks_noted"`
	Diagrams        int             `json:"diagrams"`
	Comments        int             `json:"comments"`
	CommentsPending int             `json:"comments_pending"`
	ReviewItems     int             `json:"review_items"`
	ReviewOpen      int             `json:"review_open"`
}

func (c *SessionInfoCmd) Run(g *Globals) error {
	out, err := buildInfo(g)
	if err != nil {
		return err
	}
	if !c.JSON {
		fmt.Printf("%s  %s  [%s]\n  %s\n  %s\n  %d files  %d/%d hunks noted  %d comments (%d pending)\n",
			out.Session, out.Title, out.Kind, out.URL, out.Dir,
			len(out.Files), out.HunksNoted, out.HunksTotal, out.Comments, out.CommentsPending)
		return nil
	}
	return printJSON(out)
}

func buildInfo(g *Globals) (*infoOut, error) {
	sid, dir, err := session.Current(g.Session)
	if err != nil {
		return nil, usage("%v", err)
	}
	sess, err := session.LoadSession(dir)
	if err != nil {
		return nil, usage("%v", err)
	}
	url, err := daemon.SessionURL(sid, g.Port)
	if err != nil {
		return nil, envErr("%v", err)
	}
	d, notes := diff.Load(dir), diff.LoadNotes(dir)
	title := notes.Title
	if title == "" {
		title = sess.Title
	}
	files := make([]fileRow, 0, len(d.Files))
	total := 0
	for _, f := range d.Files {
		files = append(files, fileRow{f.Path, f.Status, f.Add, f.Del, len(f.Hunks)})
		total += len(f.Hunks)
	}
	cs := session.Comments(dir)
	pending := 0
	for _, c := range cs {
		if c["status"] == "sent" {
			pending++
		}
	}
	items, open := reviewCounts(dir)
	return &infoOut{
		Session: sid, Dir: dir, URL: url, Kind: sess.Kind, Title: title, Repo: sess.Repo,
		Meta: rawField(sess.Raw, "meta"), Brief: rawField(sess.Raw, "brief"),
		Archived: sess.IsArchived(), Files: files, HunksTotal: total, HunksNoted: notes.NotedHunks(),
		Diagrams: len(notes.Diagrams), Comments: len(cs), CommentsPending: pending,
		ReviewItems: items, ReviewOpen: open,
	}, nil
}

func rawField(raw json.RawMessage, key string) json.RawMessage {
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return json.RawMessage("null")
	}
	if v, ok := m[key]; ok {
		return v
	}
	return json.RawMessage("null")
}

func reviewCounts(dir string) (items, open int) {
	var rv struct {
		Items []struct {
			Status string `json:"status"`
		} `json:"items"`
	}
	b, err := os.ReadFile(filepath.Join(dir, "review.json"))
	if err != nil || json.Unmarshal(b, &rv) != nil {
		return 0, 0
	}
	for _, it := range rv.Items {
		if it.Status == "" || it.Status == "open" {
			open++
		}
	}
	return len(rv.Items), open
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	return enc.Encode(v)
}

type SessionURLCmd struct {
	Home bool `help:"Print the session list URL instead."`
}

func (c *SessionURLCmd) Run(g *Globals) error {
	if c.Home {
		url, err := daemon.HomeURL(g.Port)
		if err != nil {
			return envErr("%v", err)
		}
		fmt.Println(url)
		return nil
	}
	sid, _, err := session.Current(g.Session)
	if err != nil {
		return usage("%v", err)
	}
	url, err := daemon.SessionURL(sid, g.Port)
	if err != nil {
		return envErr("%v", err)
	}
	fmt.Println(url)
	return nil
}

type SessionRefreshCmd struct{}

func (c *SessionRefreshCmd) Run(g *Globals) error {
	_, dir, err := session.Current(g.Session)
	if err != nil {
		return usage("%v", err)
	}
	sess, err := session.LoadSession(dir)
	if err != nil {
		return usage("%v", err)
	}
	if sess.Refresh == nil {
		return usage("a patch session cannot be refreshed")
	}
	src, err := sourceFromRefresh(sess)
	if err != nil {
		return envErr("%v", err)
	}
	r, err := session.ApplyRefresh(dir, src)
	if err != nil {
		return envErr("%v", err)
	}
	if !r.Changed {
		fmt.Println("the diff is unchanged")
		return nil
	}
	_, add, del, _ := diff.Diff{Files: r.Files}.Totals()
	fmt.Printf("refreshed: %d files +%d -%d\n", len(r.Files), add, del)
	if len(r.Dropped) > 0 {
		fmt.Println("notes void (hunks moved, rewrite them): " + strings.Join(r.Dropped, ", "))
	}
	if len(r.Stale) > 0 {
		fmt.Println("comments that lost their line (marked stale): " + strings.Join(r.Stale, ", "))
	}
	return nil
}

// sourceFromRefresh replays the tokens stored in session.refresh. They are the
// oracle's argv fragments, so the mapping has to match source_from_args().
func sourceFromRefresh(sess *session.Session) (*source.Source, error) {
	if err := os.Chdir(sess.Repo); err != nil {
		return nil, err
	}
	o := source.Opts{Title: sess.Title}
	scope, target := source.Worktree, []string{}
	for i := 0; i < len(sess.Refresh); i++ {
		switch tok := sess.Refresh[i]; tok {
		case "--staged":
			scope = source.Staged
		case "--unstaged":
			scope = source.Unstaged
		case "--":
			o.Pathspec = sess.Refresh[i+1:]
			i = len(sess.Refresh)
		default:
			target = append(target, tok)
		}
	}
	if len(target) == 0 {
		return source.FromWorktree(scope, o)
	}
	if target[0] == "pr" && len(target) > 1 {
		return source.FromPR(target[1], o)
	}
	if target[0] == "files" && len(target) == 3 {
		return source.FromFiles(target[1], target[2], o)
	}
	spec := strings.Join(target, " ")
	if strings.Contains(spec, "..") {
		return source.FromRange(spec, o)
	}
	return source.FromCommit(spec, o)
}

type SessionArchiveCmd struct {
	IDs  []string `arg:"" optional:"" help:"Session ids."`
	Undo bool     `help:"Un-archive instead."`
}

func (c *SessionArchiveCmd) Run(g *Globals) error {
	for _, sid := range c.IDs {
		dir := session.Dir(sid)
		if !session.Exists(dir) {
			fmt.Fprintf(os.Stderr, "hudiff: no such session: %s\n", sid)
			continue
		}
		if err := session.SetArchived(dir, !c.Undo); err != nil {
			return envErr("%v", err)
		}
		state := "archived (hidden from the home page)"
		if c.Undo {
			state = "unarchived"
		}
		fmt.Printf("%s → %s\n", sid, state)
	}
	return nil
}

type SessionRmCmd struct {
	IDs []string `arg:"" optional:"" help:"Session ids."`
}

func (c *SessionRmCmd) Run(g *Globals) error {
	for _, sid := range c.IDs {
		if err := session.Remove(sid); err != nil {
			fmt.Fprintf(os.Stderr, "hudiff: %v\n", err)
			continue
		}
		fmt.Printf("deleted %s\n", sid)
	}
	return nil
}
