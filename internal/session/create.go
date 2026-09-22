package session

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kackerx/hudiff/internal/diff"
	"github.com/kackerx/hudiff/internal/source"
)

// ponytail: notes.json / comments.json are rewritten from map[string]any, so
// Go sorts their keys where the oracle kept file order. Same JSON, different
// bytes; compare them with a JSON-aware diff. Revisit only if bytes must match.

type opener struct {
	CWD          string `json:"cwd"`
	AgentSession string `json:"agent_session"`
}

// onDisk is session.json's field order, which is part of the contract (§8.1).
type onDisk struct {
	ID        string   `json:"id"`
	Kind      string   `json:"kind"`
	Title     string   `json:"title"`
	Meta      any      `json:"meta"`
	Repo      string   `json:"repo"`
	Refresh   []string `json:"refresh"`
	Created   string   `json:"created"`
	SubmitSeq int      `json:"submit_seq"`
	Brief     *Brief   `json:"brief"`
	Key       string   `json:"key"`
	Opener    opener   `json:"opener"`
}

func NowISO() string { return time.Now().Format("2006-01-02T15:04:05-07:00") }

// SourceKey identifies what a session points at, so a second run on the same
// commit / PR / range / working tree reuses it instead of piling up.
func SourceKey(kind, repo string, meta map[string]any, refresh []string) string {
	real, err := filepath.EvalSymlinks(repo)
	if err != nil {
		real = repo
	}
	str := func(k string) string {
		v, _ := meta[k].(string)
		return v
	}
	var ident string
	switch kind {
	case "commit":
		if ident = str("full"); ident == "" {
			ident = str("rev")
		}
	case "range":
		ident = fmt.Sprintf("%s..%s", metaOrNone(meta, "base"), metaOrNone(meta, "head"))
	case "pr":
		if ident = str("url"); ident == "" {
			ident = pyList(refresh)
		}
	case "worktree":
		if ident = str("scope"); ident == "" {
			ident = "worktree"
		}
	default:
		ident = str("source")
	}
	return fmt.Sprintf("%s:%s:%s", kind, real, ident)
}

// metaOrNone renders a missing key the way Python's f-string renders None.
func metaOrNone(meta map[string]any, k string) string {
	v, ok := meta[k]
	if !ok || v == nil {
		return "None"
	}
	return fmt.Sprint(v)
}

// pyList matches str(["pr", "260"]) — only reachable when gh returns no URL.
func pyList(xs []string) string {
	out := "["
	for i, x := range xs {
		if i > 0 {
			out += ", "
		}
		out += "'" + x + "'"
	}
	return out + "]"
}

func metaMap(v any) map[string]any {
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

func KeyOf(src *source.Source) string {
	return SourceKey(src.Kind, src.Repo, metaMap(src.Meta), src.Refresh)
}

// FindByKey returns the newest live session pointing at the same source. Old
// sessions predate the stored key, so it is recomputed for them.
func FindByKey(key string) *Session {
	for _, id := range idsDescending() {
		s, err := LoadSession(Dir(id))
		if err != nil || s.ID == "" || s.IsArchived() {
			continue
		}
		k := s.Key
		if k == "" && s.Repo != "" && s.Kind != "" {
			k = SourceKey(s.Kind, s.Repo, s.Meta, s.Refresh)
		}
		if k == key {
			return s
		}
	}
	return nil
}

func New(src *source.Source, brief *Brief) (string, string, error) {
	if err := os.MkdirAll(SessionsDir(), 0o755); err != nil {
		return "", "", err
	}
	var b [2]byte
	_, _ = rand.Read(b[:])
	sid := time.Now().Format("20060102-150405") + "-" + hex.EncodeToString(b[:])
	dir := Dir(sid)
	if err := os.Mkdir(dir, 0o755); err != nil {
		return "", "", err
	}
	files := diff.ParsePatch(src.Patch)
	if len(files) == 0 {
		_ = os.RemoveAll(dir)
		return "", "", fmt.Errorf("the diff is empty, nothing to look at")
	}
	if err := os.WriteFile(filepath.Join(dir, "diff.patch"), []byte(src.Patch), 0o644); err != nil {
		return "", "", err
	}
	if err := writeAll(dir, map[string]any{
		"diff.json": diff.Diff{Files: files}, "notes.json": map[string]any{}, "comments.json": []any{},
	}); err != nil {
		return "", "", err
	}
	cwd, _ := os.Getwd()
	err := WriteJSON(filepath.Join(dir, "session.json"), onDisk{
		ID: sid, Kind: src.Kind, Title: src.Title, Meta: src.Meta, Repo: src.Repo,
		Refresh: src.Refresh, Created: NowISO(), SubmitSeq: 0, Brief: brief, Key: KeyOf(src),
		Opener: opener{CWD: cwd, AgentSession: agentSession()},
	})
	if err != nil {
		return "", "", err
	}
	return sid, dir, os.WriteFile(CurrentFile(), []byte(sid), 0o644)
}

func writeAll(dir string, files map[string]any) error {
	for name, v := range files {
		if err := WriteJSON(filepath.Join(dir, name), v); err != nil {
			return err
		}
	}
	return nil
}

func agentSession() string {
	for _, k := range []string{"CLAUDE_SESSION_ID", "PI_COMPANION_SESSION_ID"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}
