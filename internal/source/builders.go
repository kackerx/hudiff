package source

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type Scope string

const (
	Worktree Scope = "worktree"
	Staged   Scope = "staged"
	Unstaged Scope = "unstaged"
)

// Default titles stay Chinese: they are stored in session.json and shown on the
// page, which turns English in M5 together with the rest of the UI.
var scopeTitle = map[Scope]string{
	Worktree: "工作区未提交的改动",
	Staged:   "已暂存的改动",
	Unstaged: "未暂存的改动",
}

func FromWorktree(scope Scope, o Opts) (*Source, error) {
	root, err := Root()
	if err != nil {
		return nil, err
	}
	h, err := readHead(root)
	if err != nil {
		return nil, err
	}
	args := append([]string{"diff"}, diffFlags...)
	var refresh []string
	run := git
	switch scope {
	case Staged:
		args, refresh = append(args, "--cached"), []string{"--staged"}
	case Unstaged:
		refresh = []string{"--unstaged"}
	default:
		// Only the default scope shows untracked files (spec §3, §11.8).
		args, refresh, run = append(args, "HEAD"), []string{}, withUntracked
	}
	patch, err := run(root, pathspec(args, o.Pathspec)...)
	if err != nil {
		return nil, err
	}
	return &Source{
		Kind: "worktree", Title: title(o, scopeTitle[scope]),
		Meta:  WorktreeMeta{Base: h.short, Branch: h.branch, Scope: string(scope), Diagram: o.Diagram},
		Patch: patch, Repo: root, Refresh: refreshWith(refresh, o.Pathspec),
	}, nil
}

func FromRange(spec string, o Opts) (*Source, error) {
	root, err := Root()
	if err != nil {
		return nil, err
	}
	h, err := readHead(root)
	if err != nil {
		return nil, err
	}
	if !strings.Contains(spec, "..") {
		return nil, usage("%q is not a range; use <a>..<b> or <a>...<b>", spec)
	}
	patch, err := git(root, pathspec(append(append([]string{"diff"}, diffFlags...), spec), o.Pathspec)...)
	if err != nil {
		return nil, err
	}
	a, b := splitRange(spec)
	if b == "" {
		b = "HEAD"
	}
	return &Source{
		Kind: "range", Title: title(o, spec),
		Meta:  RangeMeta{Base: a, Head: b, Branch: h.branch, Diagram: o.Diagram},
		Patch: patch, Repo: root, Refresh: refreshWith([]string{spec}, o.Pathspec),
	}, nil
}

func splitRange(spec string) (string, string) {
	if i := strings.Index(spec, "..."); i >= 0 {
		return spec[:i], spec[i+3:]
	}
	i := strings.Index(spec, "..")
	return spec[:i], spec[i+2:]
}

func FromCommit(spec string, o Opts) (*Source, error) {
	root, err := Root()
	if err != nil {
		return nil, err
	}
	rev := strings.TrimSpace(gitOK(root, "rev-parse", "--verify", spec+"^{commit}"))
	if rev == "" {
		return nil, usage("cannot resolve %q: not a commit, a range, or a PR", spec)
	}
	fields := strings.Fields(gitOK(root, "rev-list", "--parents", "-n1", rev))
	parents := fields[min(1, len(fields)):]
	args := append([]string{"diff"}, diffFlags...)
	if len(parents) > 0 {
		args = append(args, parents[0], rev)
	} else {
		args = append(append([]string{"diff-tree", "-p", "--root"}, diffFlags...), rev)
	}
	patch, err := git(root, pathspec(args, o.Pathspec)...)
	if err != nil {
		return nil, err
	}
	log, err := git(root, "log", "-1", "--format=%h%x00%s%x00%an%x00%ad%x00%b", "--date=short", rev)
	if err != nil {
		return nil, err
	}
	p := strings.SplitN(log, "\x00", 5)
	for len(p) < 5 {
		p = append(p, "")
	}
	var base *string
	if len(parents) > 0 {
		b := parents[0][:min(7, len(parents[0]))]
		base = &b
	}
	return &Source{
		Kind: "commit", Title: title(o, p[1]),
		Meta: CommitMeta{Rev: p[0], Full: rev, Author: p[2], Date: p[3], Body: strings.TrimSpace(p[4]),
			Base: base, Merge: len(parents) > 1, Diagram: o.Diagram},
		Patch: patch, Repo: root, Refresh: refreshWith([]string{spec}, o.Pathspec),
	}, nil
}

type prInfo struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	URL         string `json:"url"`
	BaseRefName string `json:"baseRefName"`
	HeadRefName string `json:"headRefName"`
	Author      *struct {
		Login string `json:"login"`
	} `json:"author"`
}

func FromPR(ref string, o Opts) (*Source, error) {
	root, err := Root()
	if err != nil {
		return nil, err
	}
	num := PRNumber(ref)
	if num == "" {
		return nil, usage("%q is not a PR number, #number or URL", ref)
	}
	if _, err := exec.LookPath("gh"); err != nil {
		return nil, envErr("viewing a PR needs the gh CLI")
	}
	raw, err := gh(root, "pr", "view", num, "--json",
		"number,title,body,url,baseRefName,headRefName,author,additions,deletions")
	if err != nil {
		return nil, err
	}
	var info prInfo
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		return nil, envErr("gh pr view returned unreadable JSON: %v", err)
	}
	patch, err := gh(root, "pr", "diff", num)
	if err != nil {
		return nil, err
	}
	var author *string
	if info.Author != nil {
		author = &info.Author.Login
	}
	return &Source{
		Kind: "pr", Title: title(o, fmt.Sprintf("#%d %s", info.Number, info.Title)),
		Meta: PRMeta{URL: info.URL, Base: info.BaseRefName, Head: info.HeadRefName,
			Author: author, Body: info.Body, Diagram: o.Diagram},
		Patch: patch, Repo: root, Refresh: []string{"pr", num},
	}, nil
}

// PRNumber accepts `123`, `#123` and a GitHub PR URL.
func PRNumber(ref string) string {
	if m := prURL.FindStringSubmatch(ref); m != nil {
		return m[3]
	}
	n := strings.TrimPrefix(ref, "#")
	if n == "" || strings.ContainsFunc(n, func(r rune) bool { return r < '0' || r > '9' }) {
		return ""
	}
	return n
}

func gh(root string, args ...string) (string, error) {
	cmd := exec.Command("gh", args...)
	cmd.Dir = root
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", envErr("gh %s failed:\n%s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

func FromPatch(file string, o Opts) (*Source, error) {
	var text string
	if file == "-" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, usage("cannot read stdin: %v", err)
		}
		text = string(b)
	} else {
		b, err := os.ReadFile(file)
		if err != nil {
			return nil, usage("cannot read %s: %v", file, err)
		}
		text = string(b)
	}
	cwd, _ := os.Getwd()
	return &Source{
		Kind: "patch", Title: title(o, "补丁: "+file),
		Meta: PatchMeta{Source: file, Diagram: o.Diagram}, Patch: text, Repo: cwd, Refresh: nil,
	}, nil
}

func title(o Opts, def string) string {
	if o.Title != "" {
		return o.Title
	}
	return def
}
