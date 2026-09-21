// Package session owns the on-disk layout under data_dir and the JSON
// contract shared with the Python oracle (specs/spec.md §8).
package session

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func StateDir() string {
	if d := os.Getenv("HUDIFF_STATE"); d != "" {
		return d
	}
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(base, "hudiff")
}

func SessionsDir() string { return filepath.Join(StateDir(), "sessions") }
func CurrentFile() string { return filepath.Join(StateDir(), "current") }
func ServerFile() string  { return filepath.Join(StateDir(), "server.json") }
func Dir(sid string) string {
	return filepath.Join(SessionsDir(), sid)
}

// ReadRaw returns the file bytes, or def when it does not exist.
func ReadRaw(path string, def string) json.RawMessage {
	b, err := os.ReadFile(path)
	if err != nil {
		return json.RawMessage(def)
	}
	return json.RawMessage(b)
}

func ReadJSON(path string, out any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

// WriteJSON matches the oracle's `json.dump(..., ensure_ascii=False, indent=1)`
// plus a temp+rename swap.
func WriteJSON(path string, v any) error {
	buf, err := marshalIndent1(v)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func marshalIndent1(v any) ([]byte, error) {
	var sb strings.Builder
	enc := json.NewEncoder(&sb)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return []byte(strings.TrimSuffix(sb.String(), "\n")), nil
}

type Meta map[string]any

type Session struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	Title     string          `json:"title"`
	Meta      Meta            `json:"meta"`
	Repo      string          `json:"repo"`
	Created   string          `json:"created"`
	SubmitSeq int             `json:"submit_seq"`
	Archived  any             `json:"archived"`
	Brief     *Brief          `json:"brief"`
	Key       string          `json:"key"`
	Raw       json.RawMessage `json:"-"`
}

type Brief struct {
	Presets []struct {
		Path string `json:"path"`
		Text string `json:"text"`
	} `json:"presets"`
	Prompt  string   `json:"prompt"`
	Context []string `json:"context"`
}

func (s *Session) IsArchived() bool {
	switch v := s.Archived.(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		return v != ""
	}
	return true
}

// Bundle is the four JSON files echoed verbatim by GET /api/session.
type Bundle struct {
	Session  json.RawMessage
	Diff     json.RawMessage
	Notes    json.RawMessage
	Comments json.RawMessage
	Review   json.RawMessage
}

func LoadRaw(dir string) Bundle {
	return Bundle{
		Session:  ReadRaw(filepath.Join(dir, "session.json"), "{}"),
		Diff:     ReadRaw(filepath.Join(dir, "diff.json"), `{"files": []}`),
		Notes:    ReadRaw(filepath.Join(dir, "notes.json"), "{}"),
		Comments: ReadRaw(filepath.Join(dir, "comments.json"), "[]"),
		Review:   ReadRaw(filepath.Join(dir, "review.json"), "null"),
	}
}

func LoadSession(dir string) (*Session, error) {
	raw := ReadRaw(filepath.Join(dir, "session.json"), "{}")
	var s Session
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	s.Raw = raw
	return &s, nil
}

// State mirrors sess_state(): mtimes in ns so the page can poll cheaply.
type State struct {
	NotesV     int64 `json:"notes_v"`
	CommentsV  int64 `json:"comments_v"`
	ReviewV    int64 `json:"review_v"`
	DiffV      int64 `json:"diff_v"`
	SubmitSeq  int   `json:"submit_seq"`
	Waiting    *int  `json:"waiting,omitempty"`
	BoundAlive *bool `json:"bound_alive,omitempty"`
}

func mtimeNS(path string) int64 {
	st, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return st.ModTime().UnixNano()
}

func ReadState(dir string) State {
	s, _ := LoadSession(dir)
	seq := 0
	if s != nil {
		seq = s.SubmitSeq
	}
	return State{
		NotesV:    mtimeNS(filepath.Join(dir, "notes.json")),
		CommentsV: mtimeNS(filepath.Join(dir, "comments.json")),
		ReviewV:   mtimeNS(filepath.Join(dir, "review.json")),
		DiffV:     mtimeNS(filepath.Join(dir, "diff.json")),
		SubmitSeq: seq,
	}
}

func repoRoot(cwd string) string {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func idsDescending() []string {
	ents, err := os.ReadDir(SessionsDir())
	if err != nil {
		return nil
	}
	var ids []string
	for _, e := range ents {
		if e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ids)))
	return ids
}

// Current resolves -s: explicit id, else newest live session of the current
// repo, else the global `current` file.
func Current(explicit string) (string, string, error) {
	sid := explicit
	if sid == "" {
		if root := repoRoot("."); root != "" {
			for _, id := range idsDescending() {
				s, err := LoadSession(Dir(id))
				if err == nil && s.Repo == root && !s.IsArchived() {
					sid = id
					break
				}
			}
		}
	}
	if sid == "" {
		b, err := os.ReadFile(CurrentFile())
		if err != nil {
			return "", "", fmt.Errorf("还没有会话, 先 hudiff")
		}
		sid = strings.TrimSpace(string(b))
	}
	d := Dir(sid)
	if st, err := os.Stat(d); err != nil || !st.IsDir() {
		return "", "", fmt.Errorf("会话不存在: %s", sid)
	}
	return sid, d, nil
}
