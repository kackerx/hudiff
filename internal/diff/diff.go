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
