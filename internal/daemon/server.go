// Package daemon is the single local server for every session (spec §5).
// It listens on 127.0.0.1 only and is the sole writer of comments/review.
package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kackerx/hudiff/internal/diagram"
	"github.com/kackerx/hudiff/internal/diff"
	"github.com/kackerx/hudiff/internal/session"
	"github.com/kackerx/hudiff/internal/web"
)

var sidRe = regexp.MustCompile(`^[0-9a-f-]+$`)

type server struct {
	port  int
	rooms *rooms
}

// Serve runs the foreground daemon: `hudiff daemon serve`.
func Serve(wantPort int) error {
	if err := os.MkdirAll(session.StateDir(), 0o755); err != nil {
		return err
	}
	ln, err := listen(wantPort)
	if err != nil {
		return err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	s := &server{port: port, rooms: newRooms()}
	if err := session.WriteJSON(session.ServerFile(), Info{Port: port, PID: os.Getpid(), Started: nowISO()}); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "hudiff serve http://127.0.0.1:%d/\n", port)
	return (&http.Server{Handler: s.mux()}).Serve(ln)
}

// listen mirrors the oracle's port ladder: --port, $HUDIFF_PORT, last used,
// then a random one so a restart keeps old tabs alive.
func listen(want int) (net.Listener, error) {
	var tries []int
	add := func(p int) {
		if p > 0 {
			tries = append(tries, p)
		}
	}
	add(want)
	if p, err := strconv.Atoi(os.Getenv("HUDIFF_PORT")); err == nil {
		add(p)
	}
	add(ReadInfo().Port)
	tries = append(tries, 0)
	var err error
	for _, p := range tries {
		var ln net.Listener
		ln, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			return ln, nil
		}
	}
	return nil, err
}

func (s *server) mux() *http.ServeMux {
	m := http.NewServeMux()
	m.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		sendHTML(w, 200, web.Page("index.html"))
	})
	m.HandleFunc("GET /api/sessions", func(w http.ResponseWriter, r *http.Request) {
		sendJSON(w, 200, map[string]any{"sessions": s.listSessions(), "port": s.port})
	})
	m.HandleFunc("GET /favicon.svg", s.favicon)
	m.HandleFunc("GET /favicon.ico", s.favicon)
	m.HandleFunc("GET /s/{sid}/{$}", s.withSession(s.page))
	m.HandleFunc("GET /s/{sid}/api/session", s.withSession(s.apiSession))
	m.HandleFunc("GET /s/{sid}/api/state", s.withSession(s.apiState))
	m.HandleFunc("GET /s/{sid}/api/diagram", s.withSession(s.apiDiagram))
	m.HandleFunc("GET /s/{sid}/api/agents", s.withSession(s.apiAgents))
	m.HandleFunc("GET /s/{sid}/api/hang", s.withSession(s.apiHang))
	m.HandleFunc("GET /s/{sid}/api/wait", s.withSession(s.apiWait))
	m.HandleFunc("/", notImplemented)
	return m
}

func (s *server) favicon(w http.ResponseWriter, r *http.Request) {
	send(w, 200, web.Read("favicon.svg"), "image/svg+xml")
}

// notImplemented keeps the M1 surface honest: every write route answers 501
// instead of silently doing nothing (M3/M4 fill them in).
func notImplemented(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		sendJSON(w, 404, map[string]any{"error": "not found"})
		return
	}
	sendJSON(w, 501, map[string]any{"error": "not implemented in M1", "route": r.Method + " " + r.URL.Path})
}

type sessionHandler func(w http.ResponseWriter, r *http.Request, sid, dir string)

func (s *server) withSession(h sessionHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sid := r.PathValue("sid")
		dir := session.Dir(sid)
		st, err := os.Stat(dir)
		if !sidRe.MatchString(sid) || err != nil || !st.IsDir() {
			sendHTML(w, 404, []byte(fmt.Sprintf("<h3>会话 %s 不存在</h3><a href='/'>← 全部会话</a>", sid)))
			return
		}
		h(w, r, sid, dir)
	}
}

func (s *server) page(w http.ResponseWriter, r *http.Request, sid, dir string) {
	sendHTML(w, 200, web.Page("session.html"))
}

type sessionPayload struct {
	Session  json.RawMessage `json:"session"`
	Diff     json.RawMessage `json:"diff"`
	Notes    json.RawMessage `json:"notes"`
	Comments json.RawMessage `json:"comments"`
	State    session.State   `json:"state"`
	Diagrams []diagram.Entry `json:"diagrams"`
	D2       bool            `json:"d2"`
	Review   json.RawMessage `json:"review"`
}

func (s *server) apiSession(w http.ResponseWriter, r *http.Request, sid, dir string) {
	b := session.LoadRaw(dir)
	sendJSON(w, 200, sessionPayload{
		Session: b.Session, Diff: b.Diff, Notes: b.Notes, Comments: b.Comments,
		State:    session.ReadState(dir),
		Diagrams: diagram.List(dir, diff.Load(dir)),
		D2:       diagram.HasD2(),
		Review:   b.Review,
	})
}

func (s *server) apiState(w http.ResponseWriter, r *http.Request, sid, dir string) {
	st := session.ReadState(dir)
	n, no := s.rooms.count(sid), false
	st.Waiting, st.BoundAlive = &n, &no
	sendJSON(w, 200, st)
}

func (s *server) apiDiagram(w http.ResponseWriter, r *http.Request, sid, dir string) {
	q := r.URL.Query()
	which := q.Get("i")
	if which == "" {
		which = "auto-map"
	}
	theme := "light"
	if q.Get("theme") == "dark" {
		theme = "dark"
	}
	sess, _ := session.LoadSession(dir)
	svg := diagram.Render(dir, sess.Repo, diff.Load(dir), which, theme)
	cache := "no-store"
	if q.Has("v") {
		cache = "max-age=31536000, immutable"
	}
	w.Header().Set("Cache-Control", cache)
	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(svg)))
	w.WriteHeader(200)
	_, _ = w.Write(svg)
}

// apiAgents is a stub: tmux agent discovery is gone (spec §5), the page's
// selector is removed in M5.
func (s *server) apiAgents(w http.ResponseWriter, r *http.Request, sid, dir string) {
	sendJSON(w, 200, map[string]any{"agents": []any{}, "waiting": s.rooms.count(sid), "bound": nil})
}

func (s *server) apiHang(w http.ResponseWriter, r *http.Request, sid, dir string) {
	ms, _ := strconv.ParseFloat(r.URL.Query().Get("ms"), 64)
	if ms == 0 {
		ms = 1000
	}
	time.Sleep(time.Duration(min(ms, 30000)) * time.Millisecond)
	send(w, 204, nil, "text/plain")
}

func (s *server) apiWait(w http.ResponseWriter, r *http.Request, sid, dir string) {
	q := r.URL.Query()
	since, _ := strconv.Atoi(q.Get("since"))
	timeout, err := strconv.ParseFloat(q.Get("timeout"), 64)
	if err != nil {
		timeout = 60
	}
	pane := q.Get("pane")
	if pane == "" {
		pane = "?"
	}
	seq := s.rooms.wait(r, sid, dir, pane, since, time.Duration(timeout*float64(time.Second)))
	sendJSON(w, 200, waitResult{SubmitSeq: seq, Changed: seq > since})
}

type waitResult struct {
	SubmitSeq int  `json:"submit_seq"`
	Changed   bool `json:"changed"`
}

type sessionRow struct {
	ID       string         `json:"id"`
	Kind     string         `json:"kind"`
	Title    string         `json:"title"`
	Repo     string         `json:"repo"`
	Created  string         `json:"created"`
	Meta     session.Meta   `json:"meta"`
	Archived bool           `json:"archived"`
	Files    int            `json:"files"`
	Add      int            `json:"add"`
	Del      int            `json:"del_"`
	Comments int            `json:"comments"`
	Counts   map[string]int `json:"counts"`
	Noted    bool           `json:"noted"`
	Waiting  bool           `json:"waiting"`
}

func (s *server) listSessions() []sessionRow {
	return ListSessions(func(sid string) bool { return s.rooms.count(sid) > 0 })
}

func ListSessions(waiting func(string) bool) []sessionRow {
	ents, err := os.ReadDir(session.SessionsDir())
	if err != nil {
		return []sessionRow{}
	}
	var ids []string
	for _, e := range ents {
		ids = append(ids, e.Name())
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	out := []sessionRow{}
	for _, id := range ids {
		dir := session.Dir(id)
		sess, err := session.LoadSession(dir)
		if err != nil || sess.ID == "" {
			continue
		}
		d := diff.Load(dir)
		notes := diff.LoadNotes(dir)
		files, add, del, _ := d.Totals()
		title := notes.Title
		if title == "" {
			title = sess.Title
		}
		meta := sess.Meta
		if meta == nil {
			meta = session.Meta{}
		}
		out = append(out, sessionRow{
			ID: sess.ID, Kind: sess.Kind, Title: title, Repo: sess.Repo, Created: sess.Created,
			Meta: meta, Archived: sess.IsArchived(), Files: files, Add: add, Del: del,
			Comments: countComments(dir), Counts: commentCounts(dir),
			Noted: len(notes.Files) > 0, Waiting: waiting(id),
		})
	}
	return out
}

type commentRow struct {
	Status string `json:"status"`
}

func readComments(dir string) []commentRow {
	var cs []commentRow
	b, err := os.ReadFile(filepath.Join(dir, "comments.json"))
	if err != nil {
		return nil
	}
	_ = json.Unmarshal(b, &cs)
	return cs
}

func countComments(dir string) int { return len(readComments(dir)) }

func commentCounts(dir string) map[string]int {
	c := map[string]int{}
	for _, x := range readComments(dir) {
		c[x.Status]++
	}
	return c
}

func send(w http.ResponseWriter, code int, body []byte, ctype string) {
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if w.Header().Get("Cache-Control") == "" {
		w.Header().Set("Cache-Control", "no-store")
	}
	w.WriteHeader(code)
	_, _ = w.Write(body)
}

func sendHTML(w http.ResponseWriter, code int, body []byte) {
	send(w, code, body, "text/html; charset=utf-8")
}

func sendJSON(w http.ResponseWriter, code int, v any) {
	var sb strings.Builder
	enc := json.NewEncoder(&sb)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		send(w, 500, []byte(`{"error":"encode"}`), "application/json; charset=utf-8")
		return
	}
	send(w, code, []byte(strings.TrimSuffix(sb.String(), "\n")), "application/json; charset=utf-8")
}

func nowISO() string {
	return time.Now().Format("2006-01-02T15:04:05-07:00")
}
