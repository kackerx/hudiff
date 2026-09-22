package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/kackerx/hudiff/internal/daemon"
	"github.com/kackerx/hudiff/internal/diff"
	"github.com/kackerx/hudiff/internal/session"
)

// call sends a write to the daemon, which owns comments.json. It reports false
// when no daemon is up so the caller can fall back to writing directly.
func call(sid, method, path string, body any) (bool, error) {
	info := daemon.ReadInfo()
	if !info.Alive() {
		return false, nil
	}
	var buf io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return true, err
		}
		buf = bytes.NewReader(b)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/s/%s%s", info.Port, sid, path)
	req, err := http.NewRequest(method, url, buf)
	if err != nil {
		return true, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return true, fmt.Errorf("%s", e.Error)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return true, nil
}

func readTextArg(v string) (string, error) {
	if v != "-" {
		return strings.TrimSpace(v), nil
	}
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", usage("cannot read stdin: %v", err)
	}
	return strings.TrimSpace(string(b)), nil
}

func (c *ReplyCmd) Run(g *Globals) error {
	sid, dir, err := session.Current(g.Session)
	if err != nil {
		return usage("%v", err)
	}
	text, err := readTextArg(c.Text)
	if err != nil {
		return err
	}
	if text == "" {
		return usage("the reply is empty")
	}
	sent, err := call(sid, "POST", "/api/comments/"+c.ID+"/reply", map[string]any{"who": "agent", "body": text})
	if err != nil {
		return usage("%v", err)
	}
	if !sent {
		if _, err := session.AddReply(dir, c.ID, "agent", text); err != nil {
			return usage("%v", err)
		}
	}
	if c.Resolve {
		if err := patch(sid, dir, c.ID, map[string]any{"agent_done": true}); err != nil {
			return usage("%v", err)
		}
	}
	msg := "replied to " + c.ID
	if c.Resolve {
		msg += ", marked handled and awaiting the user's confirmation (only the user can resolve)"
	}
	fmt.Println(msg)
	return nil
}

func patch(sid, dir, id string, fields map[string]any) error {
	sent, err := call(sid, "PATCH", "/api/comments/"+id, fields)
	if err != nil {
		return err
	}
	if !sent {
		_, err = session.PatchComment(dir, id, fields)
	}
	return err
}

func (c *ResolveCmd) Run(g *Globals) error {
	sid, dir, err := session.Current(g.Session)
	if err != nil {
		return usage("%v", err)
	}
	for _, id := range c.IDs {
		fields, state := map[string]any{"agent_done": true}, "handled by the agent, awaiting the user"
		switch {
		case c.Reopen:
			fields, state = map[string]any{"status": "open", "agent_done": false}, "open"
		case c.By == "user":
			fields, state = map[string]any{"status": "resolved"}, "resolved"
		}
		if err := patch(sid, dir, id, fields); err != nil {
			return usage("%v", err)
		}
		fmt.Printf("%s → %s\n", id, state)
	}
	return nil
}

func (c *CommentsCmd) Run(g *Globals) error {
	sid, dir, err := session.Current(g.Session)
	if err != nil {
		return usage("%v", err)
	}
	sess, _ := session.LoadSession(dir)
	all := session.Comments(dir)
	var shown []session.Comment
	for _, x := range all {
		st, _ := x["status"].(string)
		switch {
		case c.New:
			if st == "sent" {
				shown = append(shown, x)
			}
		case c.All:
			shown = append(shown, x)
		case st != "resolved":
			shown = append(shown, x)
		}
	}
	header := fmt.Sprintf("# session %s: %s  %d total, showing %d", sid, sess.Title, len(all), len(shown))
	return printComments(shown, diff.Load(dir), c.JSON, header)
}

var statusWord = map[string]string{
	"open": "not sent", "sent": "pending", "replied": "replied", "resolved": "resolved",
}

func printComments(cs []session.Comment, d diff.Diff, asJSON bool, header string) error {
	if asJSON {
		if cs == nil {
			cs = []session.Comment{}
		}
		return printJSON(cs)
	}
	if header != "" {
		fmt.Println(header)
	}
	if len(cs) == 0 {
		fmt.Println("(no comments)")
		return nil
	}
	for _, c := range cs {
		printComment(c, d)
	}
	return nil
}

func printComment(c session.Comment, d diff.Diff) {
	st, _ := c["status"].(string)
	word := statusWord[st]
	if word == "" {
		word = st
	}
	flag := ""
	if b, _ := c["stale"].(bool); b {
		flag += " ⚠ the line's content changed, the position may be off"
	}
	if b, _ := c["agent_done"].(bool); b && st != "resolved" {
		flag += "  ✔ handled by the agent, awaiting the user"
	}
	fmt.Printf("\n[%v] %s  %s%s\n", c["id"], word, lineRef(c), flag)
	if q := quoted(c, d); q != "" {
		fmt.Println("    > " + q)
	}
	body, _ := c["body"].(string)
	for _, line := range strings.Split(body, "\n") {
		fmt.Println("    " + line)
	}
	printReplies(c)
}

func printReplies(c session.Comment) {
	reps, _ := c["replies"].([]any)
	lastAgent := -1
	for i, r := range reps {
		if m, ok := r.(map[string]any); ok && m["who"] == "agent" {
			lastAgent = i
		}
	}
	for i, r := range reps {
		m, ok := r.(map[string]any)
		if !ok {
			continue
		}
		who := "me"
		if m["who"] == "agent" {
			who = "agent"
		} else if i > lastAgent {
			who = "me (new)"
		}
		body, _ := m["body"].(string)
		for j, line := range strings.Split(body, "\n") {
			label := strings.Repeat(" ", len(who)+1)
			if j == 0 {
				label = who + ":"
			}
			fmt.Printf("    %s %s\n", label, line)
		}
	}
}

func lineRef(c session.Comment) string {
	file, _ := c["file"].(string)
	switch c["target"] {
	case "line":
		side := "old"
		if c["side"] == "new" {
			side = "new"
		}
		return fmt.Sprintf("%s  %s L%v  (hunk %v)", file, side, c["line"], c["hunk"])
	case "hunk":
		return fmt.Sprintf("%s  whole hunk %v", file, c["hunk"])
	}
	return "overall"
}

func quoted(c session.Comment, d diff.Diff) string {
	file, _ := c["file"].(string)
	switch c["target"] {
	case "line":
		if ln := d.FindLine(file, toInt(c["hunk"]), fmt.Sprint(c["side"]), toInt(c["line"])); ln != nil {
			return ln.T + ln.S
		}
		if s, ok := c["snippet"].(string); ok {
			return s
		}
	case "hunk":
		for _, f := range d.Files {
			if f.Path == file && toInt(c["hunk"]) < len(f.Hunks) {
				return f.Hunks[toInt(c["hunk"])].Header
			}
		}
	}
	return ""
}

func toInt(v any) int {
	f, _ := v.(float64)
	return int(f)
}

func (c *WaitCmd) Run(g *Globals) error {
	sid, dir, err := session.Current(g.Session)
	if err != nil {
		return usage("%v", err)
	}
	d := diff.Load(dir)
	if pending := pendingComments(dir); len(pending) > 0 {
		return printComments(pending, d, c.JSON,
			fmt.Sprintf("# %d comments were already pending (the user hit Send earlier)", len(pending)))
	}
	if _, err := daemon.Ensure(g.Port); err != nil {
		return envErr("%v", err)
	}
	since := session.ReadState(dir).SubmitSeq
	if err := waitForSubmit(sid, since, c.Timeout); err != nil {
		return err
	}
	pending := pendingComments(dir)
	return printComments(pending, diff.Load(dir), c.JSON,
		fmt.Sprintf("# the user sent %d comments", len(pending)))
}

func pendingComments(dir string) []session.Comment {
	var out []session.Comment
	for _, c := range session.Comments(dir) {
		if st, _ := c["status"].(string); st == "sent" {
			out = append(out, c)
		}
	}
	return out
}

// waitForSubmit long-polls the daemon. A daemon that goes away mid-wait (a
// restart after a page change) is waited out rather than treated as an error.
func waitForSubmit(sid string, since int, timeout float64) error {
	deadline := time.Now().Add(time.Duration(timeout * float64(time.Second)))
	for {
		left := time.Until(deadline).Seconds()
		if left <= 0 {
			fmt.Fprintf(os.Stderr, "(waited %.0fs, the user has not hit Send)\n", timeout)
			return &ExitError{Code: 124, Msg: ""}
		}
		changed, ok := pollWait(sid, since, min(left, 30))
		if ok && changed {
			return nil
		}
		if !ok && !awaitDaemon() {
			return envErr("the daemon went away and will not come back")
		}
	}
}

func pollWait(sid string, since int, secs float64) (changed, ok bool) {
	info := daemon.ReadInfo()
	if !info.Alive() {
		return false, false
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/s/%s/api/wait?since=%d&timeout=%.0f&pane=pid:%d",
		info.Port, sid, since, secs, os.Getpid())
	client := &http.Client{Timeout: time.Duration(secs+5) * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false, false
	}
	defer resp.Body.Close()
	var r struct {
		Changed bool `json:"changed"`
	}
	if json.NewDecoder(resp.Body).Decode(&r) != nil {
		return false, false
	}
	return r.Changed, true
}

func awaitDaemon() bool {
	for range 15 {
		time.Sleep(time.Second)
		if _, err := daemon.Ensure(0); err == nil {
			return true
		}
	}
	return false
}
