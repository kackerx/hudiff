package session

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/kackerx/hudiff/internal/diff"
)

// Comment stays a loose map so unknown fields survive a rewrite.
type Comment = map[string]any

// Writers take the session's lock; readers do not need one, because every
// write lands via temp+rename and a reader sees either the old or the new file.
var locks sync.Map

// Lock is the session's write lock, shared with packages that own one of its
// files (review.json).
func Lock(dir string) *sync.RWMutex { return lockFor(dir) }

func lockFor(dir string) *sync.RWMutex {
	v, _ := locks.LoadOrStore(dir, &sync.RWMutex{})
	return v.(*sync.RWMutex)
}

var commentID = regexp.MustCompile(`^c(\d+)$`)

func nextID(cs []Comment) string {
	mx := 0
	for _, c := range cs {
		id, _ := c["id"].(string)
		if m := commentID.FindStringSubmatch(id); m != nil {
			if n, _ := strconv.Atoi(m[1]); n > mx {
				mx = n
			}
		}
	}
	return "c" + strconv.Itoa(mx+1)
}

func writeComments(dir string, cs []Comment) error {
	if cs == nil {
		cs = []Comment{}
	}
	return WriteJSON(filepath.Join(dir, "comments.json"), cs)
}

func findComment(cs []Comment, id string) Comment {
	for _, c := range cs {
		if s, _ := c["id"].(string); s == id {
			return c
		}
	}
	return nil
}

func str(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return s
}

func asInt(v any) int {
	f, _ := v.(float64)
	return int(f)
}

// AddComment appends a comment. A line comment records the line's text so
// refresh can re-anchor it by content.
func AddComment(dir string, in Comment) (Comment, error) {
	mu := lockFor(dir)
	mu.Lock()
	defer mu.Unlock()

	body := strings.TrimSpace(str(in, "body"))
	if body == "" {
		return nil, fmt.Errorf("empty comment")
	}
	target := str(in, "target")
	if target == "" {
		target = "line"
	}
	author := str(in, "author")
	if author == "" {
		author = "user"
	}
	cs := Comments(dir)
	c := Comment{
		"id": nextID(cs), "target": target, "file": in["file"], "hunk": in["hunk"],
		"side": in["side"], "line": in["line"], "snippet": nil, "body": body,
		"status": "open", "stale": false, "created": NowISO(), "replies": []any{},
		"author": author,
	}
	if target == "line" {
		ln := diff.Load(dir).FindLine(str(in, "file"), asInt(in["hunk"]), str(in, "side"), asInt(in["line"]))
		if ln == nil {
			return nil, fmt.Errorf("no such line")
		}
		c["snippet"] = ln.S
	}
	return c, writeComments(dir, append(cs, c))
}

// AddReply appends a reply. A user reply carries sent=false so the chip can
// follow the message rather than the thread.
func AddReply(dir, id, who, body string) (Comment, error) {
	mu := lockFor(dir)
	mu.Lock()
	defer mu.Unlock()

	cs := Comments(dir)
	c := findComment(cs, id)
	if c == nil {
		return nil, fmt.Errorf("no such comment: %s", id)
	}
	r := map[string]any{"who": who, "body": strings.TrimSpace(body), "ts": NowISO()}
	if who == "user" {
		r["sent"] = false
		c["status"] = "open"
	} else {
		c["status"] = "replied"
	}
	reps, _ := c["replies"].([]any)
	c["replies"] = append(reps, r)
	return c, writeComments(dir, cs)
}

// PatchComment applies the fields the page and the CLI are allowed to change.
func PatchComment(dir, id string, fields map[string]any) (Comment, error) {
	mu := lockFor(dir)
	mu.Lock()
	defer mu.Unlock()

	cs := Comments(dir)
	c := findComment(cs, id)
	if c == nil {
		return nil, fmt.Errorf("no such comment: %s", id)
	}
	for _, k := range []string{"body", "status", "agent_done"} {
		if v, ok := fields[k]; ok {
			c[k] = v
		}
	}
	if fields["status"] == "resolved" {
		c["agent_done"] = false
	}
	return c, writeComments(dir, cs)
}

func DeleteComment(dir, id string) error {
	mu := lockFor(dir)
	mu.Lock()
	defer mu.Unlock()

	var keep []Comment
	for _, c := range Comments(dir) {
		if s, _ := c["id"].(string); s != id {
			keep = append(keep, c)
		}
	}
	return writeComments(dir, keep)
}

// Submit is the page's "send to agent": open comments become sent, pending user
// replies are marked sent, and submit_seq moves so waiters wake.
func Submit(dir string) (int, int, error) {
	mu := lockFor(dir)
	mu.Lock()
	defer mu.Unlock()

	cs := Comments(dir)
	n := 0
	for _, c := range cs {
		if s, _ := c["status"].(string); s == "open" {
			c["status"] = "sent"
			n++
		}
		reps, _ := c["replies"].([]any)
		for _, r := range reps {
			if m, ok := r.(map[string]any); ok && m["who"] == "user" && m["sent"] == false {
				m["sent"] = true
			}
		}
	}
	if err := writeComments(dir, cs); err != nil {
		return 0, 0, err
	}
	seq, err := bumpSubmitSeq(dir)
	return n, seq, err
}

// BumpSubmit wakes waiters without touching comments; a new review round uses
// it.
func BumpSubmit(dir string) (int, int, error) {
	mu := lockFor(dir)
	mu.Lock()
	defer mu.Unlock()
	seq, err := bumpSubmitSeq(dir)
	return 0, seq, err
}

// bumpSubmitSeq rewrites session.json through a map so fields hudiff does not
// know about survive.
func bumpSubmitSeq(dir string) (int, error) {
	path := filepath.Join(dir, "session.json")
	raw := loadAny(path)
	seq := asInt(raw["submit_seq"]) + 1
	raw["submit_seq"], raw["last_submit"] = seq, NowISO()
	return seq, WriteJSON(path, raw)
}

func WriteNotes(dir string, notes any) error {
	mu := lockFor(dir)
	mu.Lock()
	defer mu.Unlock()
	return WriteJSON(filepath.Join(dir, "notes.json"), notes)
}

func Exists(dir string) bool {
	st, err := os.Stat(dir)
	return err == nil && st.IsDir()
}

// SetArchived hides a session from the home page; the data and the link stay.
// The oracle stores a timestamp, not a bool, and old data may hold either.
func SetArchived(dir string, on bool) error {
	mu := lockFor(dir)
	mu.Lock()
	defer mu.Unlock()
	path := filepath.Join(dir, "session.json")
	raw := loadAny(path)
	if on {
		raw["archived"] = NowISO()
	} else {
		raw["archived"] = nil
	}
	return WriteJSON(path, raw)
}

// Remove deletes a session and its comments. Irreversible.
func Remove(sid string) error {
	dir := Dir(sid)
	if !Exists(dir) {
		return fmt.Errorf("no such session: %s", sid)
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if b, err := os.ReadFile(CurrentFile()); err == nil && strings.TrimSpace(string(b)) == sid {
		_ = os.Remove(CurrentFile())
	}
	return nil
}
