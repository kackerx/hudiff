//go:build !windows

package agents

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// List returns every agent on the machine, marked against one session: whose
// working directory is in its repo, who is blocked in `wait` on it, and which
// one it is bound to. The reviewer usually sits in a different worktree, which
// is why the list is not filtered down to the repo.
func List(repo string, extra []string, waiters []int, boundPID int) []Agent {
	ps, parent := scan(extra)
	if len(ps) == 0 {
		return nil
	}
	fillCWD(ps)
	keep := make([]*proc, 0, len(ps))
	for _, p := range ps {
		if p.cwd != "" && !nested(p, ps, parent) {
			keep = append(keep, p)
		}
	}
	slices.SortFunc(keep, func(a, b *proc) int {
		if a.age != b.age {
			return int(b.age - a.age)
		}
		return a.pid - b.pid
	})

	// A waiter is `hudiff wait` run by an agent, so the agent is its ancestor —
	// the NEAREST one: agents nest (one can run another), and only the one that
	// actually started the wait is waiting.
	isAgent := make(map[int]bool, len(keep))
	for _, p := range keep {
		isAgent[p.pid] = true
	}
	waiting := map[int]bool{}
	for _, w := range waiters {
		for pid, n := w, 0; pid > 1 && n < 40; pid, n = parent[pid], n+1 {
			if isAgent[pid] {
				waiting[pid] = true
				break
			}
		}
	}

	root := real(repo)
	names, branch := label(keep), map[string]string{}
	out := make([]Agent, 0, len(keep))
	for _, p := range keep {
		if _, ok := branch[p.cwd]; !ok {
			branch[p.cwd] = branchOf(p.cwd)
		}
		out = append(out, Agent{PID: p.pid, Name: names[p.pid], Kind: p.kind, CWD: p.cwd,
			Branch: branch[p.cwd], Started: time.Now().Add(-p.age).Format(time.RFC3339),
			SameRepo: sameRepo(real(p.cwd), root), Waiting: waiting[p.pid], Bound: p.pid == boundPID && boundPID != 0})
	}
	slices.SortStableFunc(out, byPicking)
	return out
}

// sameRepo: either directory contains the other. An agent one level up in the
// worktree still counts — it can reach these files.
func sameRepo(cwd, root string) bool {
	if cwd == "" || root == "" {
		return false
	}
	sep := string(filepath.Separator)
	return cwd == root || strings.HasPrefix(cwd, root+sep) || strings.HasPrefix(root, cwd+sep)
}

// byPicking is the order the list is offered in: whoever is already waiting on
// this session, then the bound one, then this repo, then by kind and name.
func byPicking(a, b Agent) int {
	for _, c := range []int{btoi(b.Waiting) - btoi(a.Waiting), btoi(b.Bound) - btoi(a.Bound), btoi(b.SameRepo) - btoi(a.SameRepo)} {
		if c != 0 {
			return c
		}
	}
	if c := strings.Compare(a.Kind, b.Kind); c != 0 {
		return c
	}
	return strings.Compare(a.Name, b.Name)
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// nested drops the helper processes an agent spawns of its own kind. Matching on
// the kind alone would also hide an agent someone started from another agent's
// terminal, so the working directory has to match too — a helper inherits it, a
// separate session in another checkout does not.
func nested(p *proc, all []*proc, parent map[int]int) bool {
	byPID := make(map[int]*proc, len(all))
	for _, x := range all {
		byPID[x.pid] = x
	}
	for pid, n := parent[p.pid], 0; pid > 1 && n < 40; pid, n = parent[pid], n+1 {
		if up := byPID[pid]; up != nil && up.kind == p.kind && up.cwd == p.cwd {
			return true
		}
	}
	return false
}

// Alive is what the header's dot reads: signal 0 only checks that the pid is there.
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	return err == nil && p.Signal(syscall.Signal(0)) == nil
}

// scan reads the process table: the agent-looking processes, plus every pid's
// parent so nested() can walk up through the shells in between.
func scan(extra []string) ([]*proc, map[int]int) {
	out, err := exec.Command("ps", "-eo", "pid,ppid,etime,command").Output()
	if err != nil {
		return nil, nil
	}
	parent := map[int]int{}
	var found []*proc
	for line := range strings.Lines(string(out)) {
		f := strings.Fields(line)
		if len(f) < 4 {
			continue
		}
		pid, ppid := atoi(f[0]), atoi(f[1])
		if pid == 0 {
			continue
		}
		parent[pid] = ppid
		cmd := strings.TrimSpace(strings.SplitN(strings.TrimSpace(line), f[2], 2)[1])
		if kind := kindOf(cmd, extra); kind != "" {
			found = append(found, &proc{pid: pid, ppid: ppid, age: parseEtime(f[2]), cmd: cmd, kind: kind})
		}
	}
	return found, parent
}

// fillCWD asks lsof for every candidate at once: one process, not one per agent.
func fillCWD(ps []*proc) {
	pids := make([]string, len(ps))
	byPID := map[int]*proc{}
	for i, p := range ps {
		pids[i] = strconv.Itoa(p.pid)
		byPID[p.pid] = p
	}
	out, err := exec.Command("lsof", "-a", "-d", "cwd", "-p", strings.Join(pids, ","), "-Fpn").Output()
	if err != nil {
		return
	}
	cur := 0
	for line := range strings.Lines(string(out)) {
		line = strings.TrimRight(line, "\n")
		switch {
		case strings.HasPrefix(line, "p"):
			cur = atoi(line[1:])
		case strings.HasPrefix(line, "n") && byPID[cur] != nil:
			byPID[cur].cwd = line[1:]
		}
	}
}

func branchOf(cwd string) string {
	out, err := exec.Command("git", "-C", cwd, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func real(p string) string {
	if p == "" {
		return ""
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}
