//go:build !windows

package agents

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestMatchTmuxPane(t *testing.T) {
	list := "/dev/ttys028 %0\n/dev/ttys031 %4\n/dev/ttys999 %9\n"
	for _, c := range []struct{ tty, want string }{
		{"ttys031", "%4"},
		{"/dev/ttys028", "%0"},
		{"ttys031 ", "%4"}, // ps pads the column
		{"ttys777", ""},    // a tty tmux does not own
		{"??", ""},         // no controlling terminal
		{"", ""},
	} {
		if got := matchTmuxPane(c.tty, list); got != c.want {
			t.Errorf("matchTmuxPane(%q) = %q, want %q", c.tty, got, c.want)
		}
	}
}

// One os-window, two tabs; the agent is the foreground process of window 3, and
// in the second case only its parent shell is.
const kittyLSFixture = `[{"tabs":[
 {"windows":[{"id":1,"foreground_processes":[{"pid":100}]},
             {"id":2,"foreground_processes":[{"pid":200},{"pid":201}]}]},
 {"windows":[{"id":3,"foreground_processes":[{"pid":300}]}]}]}]`

func TestMatchKittyWindow(t *testing.T) {
	parent := map[int]int{300: 299, 299: 1, 201: 200, 200: 1, 555: 554, 554: 1}
	for _, c := range []struct {
		name string
		pid  int
		want int
		ok   bool
	}{
		{"direct hit", 300, 3, true},
		{"second process of a window", 201, 2, true},
		{"matched through the parent shell", 301, 0, false},
		{"unknown pid", 555, 0, false},
	} {
		got, ok := matchKittyWindow(c.pid, parent, []byte(kittyLSFixture))
		if got != c.want || ok != c.ok {
			t.Errorf("%s: matchKittyWindow(%d) = %d,%v want %d,%v", c.name, c.pid, got, ok, c.want, c.ok)
		}
	}
	// a child of the process kitty sees still reaches its window
	if id, ok := matchKittyWindow(400, map[int]int{400: 300, 300: 299}, []byte(kittyLSFixture)); !ok || id != 3 {
		t.Errorf("ancestor walk: got %d,%v want 3,true", id, ok)
	}
	if _, ok := matchKittyWindow(300, nil, []byte("not json")); ok {
		t.Error("broken ls output should not match")
	}
}

func TestDeliverRefusesNothing(t *testing.T) {
	if d := Deliver(0, "x"); d.Via != "copy" {
		t.Errorf("no pid should fall through to copy, got %q", d.Via)
	}
	if d := Deliver(1, ""); d.Via != "copy" {
		t.Errorf("no text should fall through to copy, got %q", d.Via)
	}
}

func TestSocketsIn(t *testing.T) {
	dir := t.TempDir()
	sock := filepath.Join(dir, "kitty-123")
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Skipf("no unix sockets here: %v", err)
	}
	defer l.Close()
	// the kind of stray file /tmp collects
	if err := os.WriteFile(filepath.Join(dir, "kitty-changelog.rst"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := socketsIn(filepath.Join(dir, "kitty*"), filepath.Join(dir, "kitty*")) // twice: must not duplicate
	if len(got) != 1 || got[0] != sock {
		t.Errorf("socketsIn = %v, want just %q", got, sock)
	}
}
