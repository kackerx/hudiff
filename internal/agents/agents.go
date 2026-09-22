// Package agents finds the coding agents running inside a repo: ps for the
// process table, lsof for each one's cwd, git for its branch. Nothing else —
// no terminal multiplexer, no helper binary (spec §5).
package agents

import (
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

type Agent struct {
	PID      int    `json:"pid"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	CWD      string `json:"cwd"`
	Branch   string `json:"branch"`
	Started  string `json:"started"`
	SameRepo bool   `json:"same_repo"`
	Waiting  bool   `json:"waiting"`
	Bound    bool   `json:"bound"`
}

// Kinds are the executables that count as an agent; config.agents.names adds to it.
var Kinds = []string{"claude", "pi", "opencode", "codex"}

type proc struct {
	pid, ppid int
	age       time.Duration
	kind      string
	cmd       string
	cwd       string
}

// A --resume argument is a session id, not something to show a human.
var idLike = regexp.MustCompile(`^[0-9a-fA-F-]{8,}$`)

// nameOf: what the agent was started as, else the directory it works in. The
// explicit flag says it came from the command line, so it never gets a suffix.
func nameOf(cmd, cwd string) (name string, explicit bool) {
	fields := strings.Fields(cmd)
	for i, f := range fields {
		if (f == "--name" || f == "--resume") && i+1 < len(fields) {
			if v := fields[i+1]; !idLike.MatchString(v) {
				return v, true
			}
		}
	}
	if base := filepath.Base(strings.TrimRight(cwd, "/")); base != "" && base != "." && base != "/" {
		return base, false
	}
	return "", false
}

// label numbers the agents that share a directory by which started first: pid
// order is no help, macOS reuses them. A name from the command line stands on
// its own, so only the ones falling back to the directory get numbered.
func label(ps []*proc) map[int]string {
	byDir := map[string][]*proc{}
	for _, p := range ps {
		byDir[p.cwd] = append(byDir[p.cwd], p)
	}
	out := map[int]string{}
	for _, group := range byDir {
		slices.SortFunc(group, func(a, b *proc) int { return int(b.age - a.age) })
		names := make([]string, len(group))
		fallbacks := 0
		for i, p := range group {
			n, explicit := nameOf(p.cmd, p.cwd)
			if explicit {
				names[i] = n
				continue
			}
			if n == "" {
				n = p.kind
			}
			fallbacks++
			names[i] = n + "\x00" // marked: needs a number if it is not alone
		}
		nth := 0
		for i, p := range group {
			n, marked := strings.CutSuffix(names[i], "\x00")
			if marked {
				nth++
				if fallbacks > 1 {
					n += ":" + strconv.Itoa(nth)
				}
			}
			out[p.pid] = n
		}
	}
	return out
}

func kindOf(cmd string, extra []string) string {
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return ""
	}
	base := strings.ToLower(filepath.Base(fields[0]))
	if slices.Contains(Kinds, base) || slices.Contains(extra, base) {
		return base
	}
	return ""
}

// parseEtime reads ps's [[dd-]hh:]mm:ss.
func parseEtime(s string) time.Duration {
	days := 0
	if d, rest, ok := strings.Cut(s, "-"); ok {
		days = atoi(d)
		s = rest
	}
	parts := strings.Split(s, ":")
	secs := 0
	for _, p := range parts {
		secs = secs*60 + atoi(p)
	}
	return time.Duration(days*86400+secs) * time.Second
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return n
		}
		n = n*10 + int(c-'0')
	}
	return n
}
