// Package daemon is the single local server for every session (spec §5).
// It listens on 127.0.0.1 only and is the sole writer of comments/review.
package daemon

import (
	"encoding/json"
	"fmt"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kackerx/hudiff/internal/diagram"
	"github.com/kackerx/hudiff/internal/diff"
	"github.com/kackerx/hudiff/internal/review"
	"github.com/kackerx/hudiff/internal/session"
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
	if err := session.WriteJSON(session.ServerFile(), Info{Port: port, PID: os.Getpid(), Started: session.NowISO()}); err != nil {
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
		sendHTML(w, 200, page("index.html"))
	})
	m.HandleFunc("GET /api/sessions", func(w http.ResponseWriter, r *http.Request) {
		sendJSON(w, 200, map[string]any{"sessions": s.listSessions(), "port": s.port})
	})
	m.HandleFunc("GET /favicon.svg", s.favicon)
	m.HandleFunc("GET /favicon.ico", s.favicon)
	for _, name := range rootAssets {
		m.HandleFunc("GET /"+name, func(w http.ResponseWriter, r *http.Request) {
			writeAsset(w, name)
		})
	}
	m.HandleFunc("GET /vendor/{file}", s.vendor)
	m.HandleFunc("GET /s/{sid}/{$}", s.withSession(s.page))
	m.HandleFunc("GET /s/{sid}/api/session", s.withSession(s.apiSession))
	m.HandleFunc("GET /s/{sid}/api/state", s.withSession(s.apiState))
	m.HandleFunc("GET /s/{sid}/api/diagram", s.withSession(s.apiDiagram))
	m.HandleFunc("GET /s/{sid}/api/agents", s.withSession(s.apiAgents))
	m.HandleFunc("GET /s/{sid}/api/hang", s.withSession(s.apiHang))
	m.HandleFunc("GET /s/{sid}/api/wait", s.withSession(s.apiWait))
	m.HandleFunc("POST /s/{sid}/api/comments", s.withSession(s.postComment))
	m.HandleFunc("POST /s/{sid}/api/comments/{id}/reply", s.withSession(s.postReply))
	m.HandleFunc("PATCH /s/{sid}/api/comments/{id}", s.withSession(s.patchComment))
	m.HandleFunc("DELETE /s/{sid}/api/comments/{id}", s.withSession(s.deleteComment))
	m.HandleFunc("POST /s/{sid}/api/submit", s.withSession(s.postSubmit))
	m.HandleFunc("POST /s/{sid}/api/notes", s.withSession(s.postNotes))
	m.HandleFunc("POST /s/{sid}/api/diagram/{id}/error", s.withSession(s.postDiagramError))
	m.HandleFunc("GET /s/{sid}/api/diagram-errors", s.withSession(s.diagramErrors))
	m.HandleFunc("POST /s/{sid}/api/review", s.withSession(s.postReview))
	m.HandleFunc("PATCH /s/{sid}/api/review/{id}", s.withSession(s.patchReview))
	m.HandleFunc("POST /s/{sid}/api/review/{id}/note", s.withSession(s.postReviewNote))
	m.HandleFunc("/", notImplemented)
	return m
}

func (s *server) favicon(w http.ResponseWriter, r *http.Request) {
	send(w, 200, asset("favicon.svg"), "image/svg+xml")
}

// rootAssets sit at / so both the index and /s/{sid}/ can link them.
var rootAssets = []string{"app.css", "app.js", "index.css", "index.js"}

// vendor serves the third-party bundles embedded under web/vendor, so the page
// works with no network (spec §7). The pattern is one path segment, so the name
// cannot walk out of the directory.
func (s *server) vendor(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	if name == "" || strings.HasPrefix(name, ".") {
		sendJSON(w, 404, map[string]any{"error": "not found"})
		return
	}
	writeAsset(w, "vendor/"+name)
}

func writeAsset(w http.ResponseWriter, name string) {
	body := asset(name)
	if body == nil {
		sendJSON(w, 404, map[string]any{"error": "not found"})
		return
	}
	send(w, 200, body, contentType(name))
}

func contentType(name string) string {
	if t := mime.TypeByExtension(filepath.Ext(name)); t != "" {
		return t
	}
	return "application/octet-stream"
}

// notImplemented keeps the surface honest: write routes answer 501 instead of
// silently doing nothing (M3 fills in comments/submit/notes, M4 review).
func notImplemented(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		sendJSON(w, 404, map[string]any{"error": "not found"})
		return
	}
	sendJSON(w, 501, map[string]any{"error": "not implemented yet", "route": r.Method + " " + r.URL.Path})
}

type sessionHandler func(w http.ResponseWriter, r *http.Request, sid, dir string)

func (s *server) withSession(h sessionHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sid := r.PathValue("sid")
		dir := session.Dir(sid)
		st, err := os.Stat(dir)
		if !sidRe.MatchString(sid) || err != nil || !st.IsDir() {
			// Page text stays Chinese until M5 rewrites the whole UI.
			sendHTML(w, 404, []byte(fmt.Sprintf("<h3>会话 %s 不存在</h3><a href='/'>← 全部会话</a>", sid)))
			return
		}
		h(w, r, sid, dir)
	}
}

func (s *server) page(w http.ResponseWriter, r *http.Request, sid, dir string) {
	sendHTML(w, 200, page("session.html"))
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
	raw := func(name, def string) json.RawMessage {
		return session.ReadRaw(filepath.Join(dir, name), def)
	}
	sendJSON(w, 200, sessionPayload{
		Session:  raw("session.json", "{}"),
		Diff:     raw("diff.json", `{"files": []}`),
		Notes:    raw("notes.json", "{}"),
		Comments: raw("comments.json", "[]"),
		State:    session.ReadState(dir),
		Diagrams: diagram.List(dir, diff.Load(dir), diagramCfg(dir)),
		D2:       true, // d2 is compiled in now; the page keeps the flag
		Review:   raw("review.json", "null"),
	})
}

func diagramCfg(dir string) diagram.Config {
	sess, err := session.LoadSession(dir)
	if err != nil {
		return diagram.DefaultConfig()
	}
	return diagram.ConfigFor(sess.Meta)
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
	svg := diagram.Render(dir, sess.Repo, diff.Load(dir), which, theme, diagram.ConfigFor(sess.Meta))
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

// renderErrors holds what the browser reported about a mermaid diagram it could
// not draw, so `hudiff diagram` and `hudiff wait` can show it to the agent.
// ponytail: in memory only — it dies with the daemon, which is the same
// lifetime as the page that reported it.
var renderErrors sync.Map // sid + "\x00" + diagram id -> string

func RenderError(sid, id string) string {
	if v, ok := renderErrors.Load(sid + "\x00" + id); ok {
		return v.(string)
	}
	return ""
}

func (s *server) diagramErrors(w http.ResponseWriter, r *http.Request, sid, dir string) {
	out := map[string]string{}
	renderErrors.Range(func(k, v any) bool {
		if id, ok := strings.CutPrefix(k.(string), sid+"\x00"); ok {
			out[id] = v.(string)
		}
		return true
	})
	sendJSON(w, 200, out)
}

func (s *server) postDiagramError(w http.ResponseWriter, r *http.Request, sid, dir string) {
	b := readBody(r)
	msg, _ := b["error"].(string)
	key := sid + "\x00" + r.PathValue("id")
	if strings.TrimSpace(msg) == "" {
		renderErrors.Delete(key)
	} else {
		renderErrors.Store(key, msg)
	}
	sendJSON(w, 200, map[string]any{"ok": true})
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
	seq := s.rooms.wait(r, sid, dir, since, time.Duration(timeout*float64(time.Second)))
	sendJSON(w, 200, waitResult{SubmitSeq: seq, Changed: seq > since})
}

func readBody(r *http.Request) map[string]any {
	var m map[string]any
	if json.NewDecoder(r.Body).Decode(&m) != nil || m == nil {
		return map[string]any{}
	}
	return m
}

func (s *server) postComment(w http.ResponseWriter, r *http.Request, sid, dir string) {
	c, err := session.AddComment(dir, readBody(r))
	if err != nil {
		sendJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	sendJSON(w, 200, c)
}

func (s *server) postReply(w http.ResponseWriter, r *http.Request, sid, dir string) {
	b := readBody(r)
	who, _ := b["who"].(string)
	if who == "" {
		who = "user"
	}
	body, _ := b["body"].(string)
	c, err := session.AddReply(dir, r.PathValue("id"), who, body)
	if err != nil {
		sendJSON(w, 404, map[string]any{"error": err.Error()})
		return
	}
	sendJSON(w, 200, c)
}

func (s *server) patchComment(w http.ResponseWriter, r *http.Request, sid, dir string) {
	c, err := session.PatchComment(dir, r.PathValue("id"), readBody(r))
	if err != nil {
		sendJSON(w, 404, map[string]any{"error": err.Error()})
		return
	}
	sendJSON(w, 200, c)
}

func (s *server) deleteComment(w http.ResponseWriter, r *http.Request, sid, dir string) {
	if err := session.DeleteComment(dir, r.PathValue("id")); err != nil {
		sendJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	sendJSON(w, 200, map[string]any{"ok": true})
}

// postSubmit is the session's inbox bell: whoever is in /api/wait wakes up.
func (s *server) postSubmit(w http.ResponseWriter, r *http.Request, sid, dir string) {
	n, seq, err := session.Submit(dir)
	if err != nil {
		sendJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	s.rooms.Notify()
	waiting := s.rooms.count(sid)
	var delivered any
	if waiting > 0 {
		delivered = "wait"
	}
	sendJSON(w, 200, map[string]any{"sent": n, "submit_seq": seq, "waiting": waiting,
		"delivered": delivered, "label": nil, "error": nil})
}

func (s *server) postNotes(w http.ResponseWriter, r *http.Request, sid, dir string) {
	var notes any
	if json.NewDecoder(r.Body).Decode(&notes) != nil {
		sendJSON(w, 400, map[string]any{"error": "notes is not valid JSON"})
		return
	}
	if err := session.WriteNotes(dir, notes); err != nil {
		sendJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	sendJSON(w, 200, map[string]any{"ok": true})
}

// postReview also bumps submit_seq: a new round is something an agent waiting
// on this session wants to hear about.
func (s *server) postReview(w http.ResponseWriter, r *http.Request, sid, dir string) {
	var in review.Report
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		sendJSON(w, 400, map[string]any{"error": "review is not valid JSON"})
		return
	}
	q := r.URL.Query()
	rv, problems, err := review.Set(dir, in, q.Get("replace") == "1", q.Get("force") == "1")
	if err != nil {
		sendJSON(w, 400, map[string]any{"error": "validation failed", "problems": problems})
		return
	}
	if _, _, err := session.BumpSubmit(dir); err == nil {
		s.rooms.Notify()
	}
	c := review.Count(rv)
	sendJSON(w, 200, map[string]any{"ok": true, "problems": problems, "merge": rv["_merge"],
		"counts": map[string]int{"high": c.High, "medium": c.Medium, "low": c.Low,
			"open": c.Open, "done": c.Done, "wontfix": c.Wontfix}})
}

func (s *server) patchReview(w http.ResponseWriter, r *http.Request, sid, dir string) {
	b := readBody(r)
	status, _ := b["status"].(string)
	resolution, _ := b["resolution"].(string)
	by, _ := b["by"].(string)
	if by == "" {
		by = "user"
	}
	it, err := review.SetStatus(dir, r.PathValue("id"), status, strings.TrimSpace(resolution), by)
	if err != nil {
		sendJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	sendJSON(w, 200, it)
}

func (s *server) postReviewNote(w http.ResponseWriter, r *http.Request, sid, dir string) {
	b := readBody(r)
	body, _ := b["body"].(string)
	verdict, _ := b["verdict"].(string)
	who, _ := b["who"].(string)
	if who == "" {
		who = "user"
	}
	it, err := review.Note(dir, r.PathValue("id"), strings.TrimSpace(body), verdict, who)
	if err != nil {
		sendJSON(w, 400, map[string]any{"error": err.Error()})
		return
	}
	sendJSON(w, 200, it)
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
	out := []sessionRow{}
	for _, id := range session.IDs() {
		dir := session.Dir(id)
		sess, err := session.LoadSession(dir)
		if err != nil || sess.ID == "" {
			continue
		}
		notes := diff.LoadNotes(dir)
		cs := session.Comments(dir)
		files, add, del, _ := diff.Load(dir).Totals()
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
			Comments: len(cs), Counts: commentCounts(cs),
			Noted: len(notes.Files) > 0, Waiting: s.rooms.count(id) > 0,
		})
	}
	return out
}

func commentCounts(cs []map[string]any) map[string]int {
	c := map[string]int{}
	for _, x := range cs {
		if st, ok := x["status"].(string); ok {
			c[st]++
		}
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

// rooms tracks who is long-polling each session, so the page can show
// "an agent is waiting" and submit knows whether anyone will pick it up.
type rooms struct {
	mu      sync.Mutex
	waiting map[string]int
	changed chan struct{}
}

func newRooms() *rooms {
	return &rooms{waiting: map[string]int{}, changed: make(chan struct{})}
}

func (r *rooms) count(sid string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.waiting[sid]
}

func (r *rooms) enter(sid string, delta int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.waiting[sid] += delta
}

// Notify wakes every /api/wait; called after submit_seq changes.
func (r *rooms) Notify() {
	r.mu.Lock()
	defer r.mu.Unlock()
	close(r.changed)
	r.changed = make(chan struct{})
}

func (r *rooms) sub() chan struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.changed
}

func (r *rooms) wait(req *http.Request, sid, dir string, since int, timeout time.Duration) int {
	r.enter(sid, 1)
	defer r.enter(sid, -1)
	deadline := time.After(timeout)
	for {
		ch := r.sub()
		if seq := session.ReadState(dir).SubmitSeq; seq > since {
			return seq
		}
		select {
		case <-ch:
		case <-deadline:
			return session.ReadState(dir).SubmitSeq
		case <-req.Context().Done():
			return session.ReadState(dir).SubmitSeq
		case <-time.After(5 * time.Second):
		}
	}
}
