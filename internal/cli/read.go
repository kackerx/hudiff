package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/kackerx/hudiff/internal/diff"
	"github.com/kackerx/hudiff/internal/session"
)

type ReadCmd struct {
	File    string `short:"f" help:"Only this path." placeholder:"<path>"`
	Func    bool   `short:"W" help:"Pad each hunk out to the enclosing top-level block."`
	Unified int    `short:"U" help:"Pad each hunk with N lines of the new file." placeholder:"<n>"`
}

func (c *ReadCmd) Run(g *Globals) error {
	sid, dir, err := session.Current(g.Session)
	if err != nil {
		return usage("%v", err)
	}
	sess, err := session.LoadSession(dir)
	if err != nil {
		return usage("%v", err)
	}
	d, notes := diff.Load(dir), diff.LoadNotes(dir)
	var out strings.Builder
	c.header(&out, sid, sess)
	for _, f := range d.Files {
		if c.File != "" && f.Path != c.File && f.OldPath != c.File {
			continue
		}
		c.file(&out, sess, notes, f)
	}
	_, _ = os.Stdout.WriteString(out.String())
	return nil
}

func (c *ReadCmd) header(out *strings.Builder, sid string, sess *session.Session) {
	fmt.Fprintf(out, "# hudiff session %s: %s  [%s]\n", sid, sess.Title, sess.Kind)
	out.WriteString("# Every hunk is tagged [path#n]; use n as the key in notes.json. The two number columns are old | new.\n")
	out.WriteString("# Mark after the number: ✓ noted  · explicitly needs no note (null)  blank = not written yet\n")
	if c.padding() {
		out.WriteString("# Lines starting with ~ are extra context from the new file, not part of the diff; they only show the enclosing block.\n")
	}
	b := sess.Brief
	if b == nil || (len(b.Presets) == 0 && b.Prompt == "" && len(b.Context) == 0) {
		return
	}
	out.WriteString("\n## Review request (from the user; write the notes to match)\n")
	for _, p := range b.Presets {
		fmt.Fprintf(out, "### Preset %s\n%s\n", p.Path, p.Text)
	}
	if b.Prompt != "" {
		fmt.Fprintf(out, "### This time\n%s\n", b.Prompt)
	}
	if len(b.Context) > 0 {
		out.WriteString("### Context files (read these first)\n")
		for i, p := range b.Context {
			if i > 0 {
				out.WriteString("\n")
			}
			out.WriteString("  " + p)
		}
		out.WriteString("\n")
	}
}

func (c *ReadCmd) padding() bool { return c.Func || c.Unified != 0 }

func (c *ReadCmd) file(out *strings.Builder, sess *session.Session, notes diff.Notes, f diff.File) {
	ren := ""
	if f.Status == "R" {
		ren = " (was " + f.OldPath + ")"
	}
	bin := ""
	if f.Binary {
		bin = "  (binary)"
	}
	fmt.Fprintf(out, "\n=== %s%s  [%s]  +%d -%d%s\n", f.Path, ren, f.Status, f.Add, f.Del, bin)

	var newLines []string
	if c.padding() {
		_, newLines = fileVersions(sess, f.Path, f.OldPath)
		if newLines == nil {
			out.WriteString("    (cannot read the new file; only the diff's own context is shown)\n")
		}
	}
	w := numWidth(f, newLines)
	for i, h := range f.Hunks {
		mark := notes.HunkMark(f.Path, strconv.Itoa(i))
		fmt.Fprintf(out, "--- [%s#%d] %s %s\n", f.Path, i, mark, h.Header)
		pre, post := c.context(h, newLines)
		for _, p := range pre {
			fmt.Fprintf(out, "%*s %*d ~%s\n", w, "", w, p.no, p.text)
		}
		for _, ln := range h.Lines {
			fmt.Fprintf(out, "%*s %*s %s%s\n", w, num(ln.O), w, num(ln.N), ln.T, ln.S)
		}
		for _, p := range post {
			fmt.Fprintf(out, "%*s %*d ~%s\n", w, "", w, p.no, p.text)
		}
	}
}

type padLine struct {
	no   int
	text string
}

func (c *ReadCmd) context(h diff.Hunk, newLines []string) (pre, post []padLine) {
	if !c.padding() || newLines == nil || h.NewCount <= 0 {
		return nil, nil
	}
	hs, he := h.NewStart, h.NewStart+h.NewCount-1
	var a, b int
	if c.Func {
		a, b = enclosingBlock(newLines, hs, he)
	} else {
		a, b = max(1, hs-c.Unified), min(len(newLines), he+c.Unified)
	}
	for k := a; k < hs; k++ {
		pre = append(pre, padLine{k, newLines[k-1]})
	}
	for k := he + 1; k <= b; k++ {
		post = append(post, padLine{k, newLines[k-1]})
	}
	return pre, post
}

func num(p *int) string {
	if p == nil || *p == 0 {
		return ""
	}
	return strconv.Itoa(*p)
}

// numWidth mirrors the oracle: widest of `o or n` across every line, floor 3,
// widened to the new file's line count when padding is on.
func numWidth(f diff.File, newLines []string) int {
	w := 3
	for _, h := range f.Hunks {
		for _, ln := range h.Lines {
			v := ln.O
			if v == nil || *v == 0 {
				v = ln.N
			}
			n := 4 // str(None)
			if v != nil {
				n = len(strconv.Itoa(*v))
			}
			w = max(w, n)
		}
	}
	if newLines != nil {
		w = max(w, len(strconv.Itoa(len(newLines))))
	}
	return w
}

// funcStart is git's default funcname rule: a non-blank line starting in
// column 1 begins a top-level block.
var funcStart = regexp.MustCompile(`^[A-Za-z_$@#]`)

func enclosingBlock(lines []string, start, end int) (int, int) {
	const limit = 160
	n := len(lines)
	a := max(1, min(start, n))
	for a > 1 && !funcStart.MatchString(lines[a-1]) {
		a--
	}
	b := max(end, a)
	for b < n && !funcStart.MatchString(lines[b]) {
		b++
	}
	for b > end && b <= n && strings.TrimSpace(lines[b-1]) == "" {
		b--
	}
	if b-a+1 > limit {
		a, b = max(1, start-30), min(n, end+30)
	}
	return a, b
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return []string{}
	}
	return strings.Split(s, "\n")
}

// fileVersions finds the old and new full text of a path, by session kind.
func fileVersions(sess *session.Session, path, oldPath string) (old, new []string) {
	repo, meta := sess.Repo, sess.Meta
	at := func(spec string) []string {
		out, err := exec.Command("git", "-C", repo, "show", spec).Output()
		if err != nil {
			return nil
		}
		return splitLines(string(out))
	}
	show := func(rev, p string) []string {
		if rev == "" || p == "" {
			return nil
		}
		return at(rev + ":" + p)
	}
	disk := func(p string) []string {
		b, err := os.ReadFile(filepath.Join(repo, p))
		if err != nil {
			return nil
		}
		return splitLines(string(b))
	}
	m := func(k string) string {
		if v, ok := meta[k].(string); ok {
			return v
		}
		return ""
	}
	op := oldPath
	if op == "" {
		op = path
	}
	switch sess.Kind {
	case "worktree":
		if m("scope") == "staged" {
			return show("HEAD", op), at(":" + path) // the index version
		}
		return show("HEAD", op), disk(path)
	case "commit":
		return show(m("base"), op), show(m("full"), path)
	case "range":
		return show(m("base"), op), show(m("head"), path)
	case "pr":
		head := m("head")
		for _, rev := range []string{"origin/" + head, head} {
			if v := show(rev, path); v != nil {
				base := show("origin/"+m("base"), op)
				if base == nil {
					base = show(m("base"), op)
				}
				return base, v
			}
		}
	}
	return nil, nil
}
