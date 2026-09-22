// Package diff models diff.json (specs/spec.md §8.2). Hunk indices are
// positional and must stay identical to the Python oracle: notes key off them.
package diff

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Line struct {
	T    string `json:"t"`
	O    *int   `json:"o"`
	N    *int   `json:"n"`
	S    string `json:"s"`
	NoNL bool   `json:"nonl,omitempty"`
}

type Hunk struct {
	Header   string `json:"header"`
	OldStart int    `json:"old_start"`
	OldCount int    `json:"old_count"`
	NewStart int    `json:"new_start"`
	NewCount int    `json:"new_count"`
	Section  string `json:"section"`
	Lines    []Line `json:"lines"`
}

// Path and OldPath are plain strings, not pointers: the oracle only leaves
// them null for input that is not a git diff at all.
type File struct {
	Path    string `json:"path"`
	OldPath string `json:"old_path"`
	Status  string `json:"status"`
	Binary  bool   `json:"binary"`
	Hunks   []Hunk `json:"hunks"`
	Add     int    `json:"add"`
	Del     int    `json:"del"`
}

type Diff struct {
	Files []File `json:"files"`
}

func Load(dir string) Diff {
	d := Diff{Files: []File{}}
	b, err := os.ReadFile(filepath.Join(dir, "diff.json"))
	if err != nil {
		return d
	}
	_ = json.Unmarshal(b, &d)
	if d.Files == nil {
		d.Files = []File{}
	}
	return d
}

func (d Diff) Totals() (files, add, del, hunks int) {
	for _, f := range d.Files {
		add += f.Add
		del += f.Del
		hunks += len(f.Hunks)
	}
	return len(d.Files), add, del, hunks
}

type Diagram struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Kind    string   `json:"kind"`
	Caption string   `json:"caption"`
	Hunks   []string `json:"hunks"`
	Src     string   `json:"src"`
}

type NoteFile struct {
	Summary string                     `json:"summary"`
	Hunks   map[string]json.RawMessage `json:"hunks"`
}

type Notes struct {
	Title    string              `json:"title"`
	Summary  string              `json:"summary"`
	Diagrams []Diagram           `json:"diagrams"`
	Files    map[string]NoteFile `json:"files"`
}

func LoadNotes(dir string) Notes {
	var n Notes
	b, err := os.ReadFile(filepath.Join(dir, "notes.json"))
	if err != nil {
		return n
	}
	_ = json.Unmarshal(b, &n)
	return n
}

// NotedHunks counts hunks that carry any entry, null included — the oracle's
// `sum(1 for f in notes.files.values() for v in f.hunks.values())`.
func (n Notes) NotedHunks() int {
	c := 0
	for _, f := range n.Files {
		c += len(f.Hunks)
	}
	return c
}

// HunkMark is the column shown by `read`: ✓ noted, · explicitly skipped,
// space when nothing is written yet.
func (n Notes) HunkMark(path, i string) string {
	f, ok := n.Files[path]
	if !ok || f.Hunks == nil {
		return " "
	}
	raw, ok := f.Hunks[i]
	if !ok {
		return " "
	}
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return "✓"
	}
	switch x := v.(type) {
	case nil:
		return "·"
	case float64:
		if x == 0 {
			return " "
		}
	case bool:
		if !x {
			return " "
		}
	case map[string]any:
		if truthy(x["skip"]) {
			return "·"
		}
	}
	return "✓"
}

func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case float64:
		return x != 0
	case string:
		return x != ""
	}
	return true
}
