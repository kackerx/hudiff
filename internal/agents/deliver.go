//go:build !windows

package agents

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Delivery says what actually happened, so the page can tell the user instead
// of leaving them guessing.
type Delivery struct {
	Via    string `json:"via"` // tmux | kitty | cmux | copy
	Detail string `json:"detail"`
}

// Deliver types text into the terminal the agent is running in. Adapters are
// tried in order and the first one that can reach the process wins; when none
// can, the caller copies the command instead — never silence.
//
// Reading the agent's environment would be the obvious way to find its
// terminal, but macOS does not let one process read another's env, so every
// adapter locates the target by pid or tty from the outside.
func Deliver(pid int, text string) Delivery {
	if pid <= 0 || text == "" {
		return Delivery{Via: "copy", Detail: "no agent picked"}
	}
	if d, ok := viaTmux(pid, text); ok {
		return d
	}
	if d, ok := viaKitty(pid, text); ok {
		return d
	}
	if d, ok := viaCmux(pid, text); ok {
		return d
	}
	return Delivery{Via: "copy", Detail: "no tmux pane or kitty window found for this agent"}
}

// ── tmux ────────────────────────────────────────────────────────────────────
// The agent's controlling tty is the pane's tty, which is how a pane is found
// without asking the agent anything.

func matchTmuxPane(tty, paneList string) string {
	tty = strings.TrimPrefix(strings.TrimSpace(tty), "/dev/")
	if tty == "" || tty == "??" {
		return ""
	}
	for line := range strings.Lines(paneList) {
		f := strings.Fields(line)
		if len(f) == 2 && strings.TrimPrefix(f[0], "/dev/") == tty {
			return f[1]
		}
	}
	return ""
}

// socketsIn keeps only the real sockets a glob turned up: /tmp also holds files
// like kitty-changelog.rst, and probing one costs a failed exec every delivery.
func socketsIn(patterns ...string) []string {
	var out []string
	seen := map[string]bool{}
	for _, pat := range patterns {
		paths, _ := filepath.Glob(pat)
		for _, p := range paths {
			st, err := os.Lstat(p)
			if err != nil || st.Mode()&os.ModeSocket == 0 || seen[p] {
				continue
			}
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// tmuxSockets: every server, not just the default one — a user can run tmux on
// a named socket, and the parity fixture does exactly that.
func tmuxSockets() []string {
	dir := os.Getenv("TMUX_TMPDIR")
	if dir == "" {
		dir = "/tmp"
	}
	return socketsIn(filepath.Join(dir, "tmux-*", "*"))
}

func viaTmux(pid int, text string) (Delivery, bool) {
	if _, err := exec.LookPath("tmux"); err != nil {
		return Delivery{}, false
	}
	tty := ttyOf(pid)
	for _, sock := range tmuxSockets() {
		panes, err := exec.Command("tmux", "-S", sock, "list-panes", "-a", "-F", "#{pane_tty} #{pane_id}").Output()
		if err != nil {
			continue
		}
		pane := matchTmuxPane(tty, string(panes))
		if pane == "" {
			continue
		}
		// -l sends the text literally; Enter has to be its own call or it
		// arrives as the five characters "Enter".
		if exec.Command("tmux", "-S", sock, "send-keys", "-t", pane, "-l", text).Run() != nil {
			continue
		}
		if exec.Command("tmux", "-S", sock, "send-keys", "-t", pane, "Enter").Run() != nil {
			continue
		}
		return Delivery{Via: "tmux", Detail: pane}, true
	}
	return Delivery{}, false
}

func ttyOf(pid int) string {
	out, err := exec.Command("ps", "-o", "tty=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ── kitty ───────────────────────────────────────────────────────────────────
// Remote control has to be on and listening on a socket: the daemon has no tty,
// so `kitten @` cannot reach kitty the way it does from inside a window.

type kittyLS []struct {
	Tabs []struct {
		Windows []struct {
			ID    int `json:"id"`
			Procs []struct {
				PID int `json:"pid"`
			} `json:"foreground_processes"`
		} `json:"windows"`
	} `json:"tabs"`
}

// matchKittyWindow finds the window running pid, or one of its ancestors: the
// foreground process is often the shell the agent was started from.
func matchKittyWindow(pid int, parent map[int]int, lsJSON []byte) (int, bool) {
	var ls kittyLS
	if json.Unmarshal(lsJSON, &ls) != nil {
		return 0, false
	}
	want := map[int]bool{pid: true}
	for p, n := parent[pid], 0; p > 1 && n < 40; p, n = parent[p], n+1 {
		want[p] = true
	}
	for _, osw := range ls {
		for _, tab := range osw.Tabs {
			for _, win := range tab.Windows {
				for _, pr := range win.Procs {
					if want[pr.PID] {
						return win.ID, true
					}
				}
			}
		}
	}
	return 0, false
}

func kittySockets() []string {
	return socketsIn(filepath.Join(os.TempDir(), "kitty*"), "/tmp/kitty*")
}

func viaKitty(pid int, text string) (Delivery, bool) {
	if _, err := exec.LookPath("kitten"); err != nil {
		return Delivery{}, false
	}
	_, parent := scan(nil)
	for _, sock := range kittySockets() {
		to := "unix:" + sock
		ls, err := exec.Command("kitten", "@", "--to", to, "ls").Output()
		if err != nil {
			continue
		}
		id, ok := matchKittyWindow(pid, parent, ls)
		if !ok {
			continue
		}
		if exec.Command("kitten", "@", "--to", to, "send-text", "--match", "id:"+strconv.Itoa(id), text+"\r").Run() == nil {
			return Delivery{Via: "kitty", Detail: "window " + strconv.Itoa(id)}, true
		}
	}
	return Delivery{}, false
}

// ── cmux ────────────────────────────────────────────────────────────────────
// cmux (manaflow-ai/cmux) delivers with `cmux send --surface <id>`. Which
// surface belongs to a pid is the part we cannot write blind: the shape of
// `cmux list` is not documented anywhere we can check, and guessing it would
// ship a matcher nobody has ever run. So: report it honestly and copy.
func viaCmux(pid int, text string) (Delivery, bool) {
	if _, err := exec.LookPath("cmux"); err != nil {
		return Delivery{}, false
	}
	return Delivery{Via: "copy", Detail: "cmux is installed but hudiff cannot locate its surface yet — paste the command"}, true
}
