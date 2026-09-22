package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// skillFS is the embedded skills/ tree, handed over by main.
var skillFS fs.FS

func SetSkills(f fs.FS) { skillFS = f }

// skillDirs are where each agent looks for skills.
var skillDirs = map[string]string{
	"claude":   "~/.claude/skills",
	"pi":       "~/.pi/skills",
	"opencode": "~/.config/opencode/skills",
	"codex":    "~/.codex/skills",
}

type SkillPathCmd struct{}

// Run materialises the embedded skill on disk and prints where it landed, so
// an agent can read it without hudiff shipping a loose file.
func (c *SkillPathCmd) Run(g *Globals) error {
	dir, err := materialiseSkill()
	if err != nil {
		return envErr("%v", err)
	}
	fmt.Println(filepath.Join(dir, "SKILL.md"))
	return nil
}

type SkillInstallCmd struct {
	For string `default:"all" enum:"claude,pi,opencode,codex,all" help:"Which agent to install for."`
}

func (c *SkillInstallCmd) Run(g *Globals) error {
	src, err := materialiseSkill()
	if err != nil {
		return envErr("%v", err)
	}
	targets := []string{c.For}
	if c.For == "all" {
		targets = []string{"claude", "pi", "opencode", "codex"}
	}
	for _, name := range targets {
		dest := filepath.Join(expandHome(skillDirs[name]), "hudiff")
		if err := link(src, dest); err != nil {
			fmt.Fprintf(os.Stderr, "hudiff: %s: %v\n", name, err)
			continue
		}
		fmt.Printf("%-9s %s → %s\n", name, dest, src)
	}
	return nil
}

// link points dest at src, replacing an existing symlink but never a real
// directory someone put there.
func link(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if fi, err := os.Lstat(dest); err == nil {
		if fi.Mode()&os.ModeSymlink == 0 {
			return fmt.Errorf("%s exists and is not a symlink; leaving it alone", dest)
		}
		if err := os.Remove(dest); err != nil {
			return err
		}
	}
	return os.Symlink(src, dest)
}

// materialiseSkill copies the embedded skill to $XDG_DATA_HOME/hudiff/skill.
func materialiseSkill() (string, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "share")
	}
	out := filepath.Join(base, "hudiff", "skill")
	if err := os.MkdirAll(out, 0o755); err != nil {
		return "", err
	}
	return out, fs.WalkDir(skillFS, "skills/hudiff", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(skillFS, p)
		if err != nil {
			return err
		}
		dest := filepath.Join(out, strings.TrimPrefix(p, "skills/hudiff/"))
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		return os.WriteFile(dest, b, 0o644)
	})
}
