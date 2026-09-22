package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/kackerx/hudiff/internal/config"
	"github.com/kackerx/hudiff/internal/session"
)

// runHook replaces the old tmux delivery: a user-configured shell command, told
// which session it is about through the environment. Fire and forget — the
// daemon must not block on someone's notifier.
func runHook(cmdline, sid, dir string, count int) {
	if cmdline == "" {
		return
	}
	repo := ""
	if sess, err := session.LoadSession(dir); err == nil {
		repo = sess.Repo
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/s/%s/", ReadInfo().Port, sid)
	cmd := exec.Command("sh", "-c", cmdline)
	cmd.Env = append(os.Environ(),
		"HUDIFF_SESSION="+sid, "HUDIFF_URL="+url,
		"HUDIFF_REPO="+repo, "HUDIFF_COUNT="+strconv.Itoa(count))
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "hudiff: hook failed: %v\n", err)
		return
	}
	go func() { _ = cmd.Wait() }()
}

func hooks() config.Hooks { return config.Load().Hooks }
