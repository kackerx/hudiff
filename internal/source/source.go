// Package source turns CLI arguments into a diff plus the metadata a session
// stores. Port of the oracle's source_from_args(); the meta struct field order
// is the on-disk key order, so it must not be reshuffled.
package source

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var diffFlags = []string{"--no-color", "--no-ext-diff", "-M", "-U3"}

var prURL = regexp.MustCompile(`github\.com/([^/]+)/([^/]+)/pull/(\d+)`)

type Source struct {
	Kind    string
	Title   string
	Meta    any
	Patch   string
	Repo    string
	Refresh []string // nil = cannot be refreshed (patch sessions)
}

type WorktreeMeta struct {
	Base    string `json:"base"`
	Branch  string `json:"branch"`
	Scope   string `json:"scope"`
	Diagram string `json:"diagram_engine,omitempty"`
}

type RangeMeta struct {
	Base    string `json:"base"`
	Head    string `json:"head"`
	Branch  string `json:"branch"`
	Diagram string `json:"diagram_engine,omitempty"`
}

type CommitMeta struct {
	Rev     string  `json:"rev"`
	Full    string  `json:"full"`
	Author  string  `json:"author"`
	Date    string  `json:"date"`
	Body    string  `json:"body"`
	Base    *string `json:"base"`
	Merge   bool    `json:"merge"`
	Diagram string  `json:"diagram_engine,omitempty"`
}

type PRMeta struct {
	URL     string  `json:"url"`
	Base    string  `json:"base"`
	Head    string  `json:"head"`
	Author  *string `json:"author"`
	Body    string  `json:"body"`
	Diagram string  `json:"diagram_engine,omitempty"`
}

type PatchMeta struct {
	Source  string `json:"source"`
	Diagram string `json:"diagram_engine,omitempty"`
}

// Opts carries what the five entry commands have in common.
type Opts struct {
	Title    string
	Diagram  string
	Pathspec []string
}

type Error struct {
	Code int
	Msg  string
}

func (e *Error) Error() string { return e.Msg }

func usage(f string, a ...any) error { return &Error{1, fmt.Sprintf(f, a...)} }
func envErr(f string, a ...any) error {
	return &Error{2, fmt.Sprintf(f, a...)}
}

func git(repo string, args ...string) (string, error) { return gitEnv(repo, nil, args...) }

func gitEnv(repo string, env []string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"--no-pager"}, args...)...)
	cmd.Dir = repo
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", envErr("git %s failed:\n%s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

// diffUntracked runs the diff against a scratch index where the untracked
// files have been intent-to-added, so git itself produces their new-file
// hunks. .gitignore is honoured by --exclude-standard.
func diffUntracked(repo string, args []string) (string, error) {
	others, err := git(repo, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil || others == "" {
		return git(repo, args...)
	}
	idx, err := scratchIndex(repo)
	if err != nil {
		return git(repo, args...)
	}
	defer os.Remove(idx)
	env := []string{"GIT_INDEX_FILE=" + idx}
	add := append([]string{"add", "-N", "--"}, strings.Split(strings.TrimSuffix(others, "\x00"), "\x00")...)
	if _, err := gitEnv(repo, env, add...); err != nil {
		return git(repo, args...)
	}
	return gitEnv(repo, env, args...)
}

func scratchIndex(repo string) (string, error) {
	dir, err := git(repo, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return "", err
	}
	src, err := os.ReadFile(filepath.Join(strings.TrimSpace(dir), "index"))
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp("", "hudiff-index")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Write(src); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func gitOK(repo string, args ...string) string {
	out, err := git(repo, args...)
	if err != nil {
		return ""
	}
	return out
}

func Root() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", envErr("not inside a git repository (use `hudiff patch` for a plain patch file)")
	}
	return strings.TrimSpace(string(out)), nil
}

type head struct{ branch, short string }

func readHead(root string) (head, error) {
	b, err := git(root, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return head{}, err
	}
	s, err := git(root, "rev-parse", "--short", "HEAD")
	if err != nil {
		return head{}, err
	}
	return head{strings.TrimSpace(b), strings.TrimSpace(s)}, nil
}

// pathspec appends `-- <paths>` and the matching refresh tokens.
func pathspec(args, paths []string) []string {
	if len(paths) == 0 {
		return args
	}
	return append(append(args, "--"), paths...)
}

func refreshWith(base []string, paths []string) []string {
	if len(paths) == 0 {
		return base
	}
	return append(append(base, "--"), paths...)
}

func withUntracked(repo string, args ...string) (string, error) { return diffUntracked(repo, args) }
