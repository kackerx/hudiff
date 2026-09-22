package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"

	"github.com/kackerx/hudiff/internal/diff"
	"github.com/kackerx/hudiff/internal/source"
)

type RefreshResult struct {
	Files   []diff.File
	Dropped []string // paths whose notes no longer match the hunks
	Stale   []string // comment ids that lost their line
	Changed bool
}

// ApplyRefresh re-reads the diff in place: notes survive for files whose hunk
// headers are unchanged, comments are re-anchored by line content.
func ApplyRefresh(dir string, src *source.Source) (RefreshResult, error) {
	var r RefreshResult
	oldPatch, _ := os.ReadFile(filepath.Join(dir, "diff.patch"))
	oldDiff := diff.Load(dir)
	if string(oldPatch) == src.Patch {
		return RefreshResult{Files: oldDiff.Files}, nil
	}
	r.Files, r.Changed = diff.ParsePatch(src.Patch), true

	notes := loadAny(filepath.Join(dir, "notes.json"))
	comments := Comments(dir)
	r.Dropped = dropStaleNotes(notes, oldDiff.Files, r.Files)
	Reanchor(r.Files, comments)
	for _, c := range comments {
		if stale, _ := c["stale"].(bool); stale {
			if id, ok := c["id"].(string); ok {
				r.Stale = append(r.Stale, id)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "diff.patch"), []byte(src.Patch), 0o644); err != nil {
		return r, err
	}
	if err := writeAll(dir, map[string]any{
		"diff.json": diff.Diff{Files: r.Files}, "notes.json": notes, "comments.json": comments,
	}); err != nil {
		return r, err
	}
	return r, os.RemoveAll(filepath.Join(dir, "diagrams"))
}

func loadAny(path string) map[string]any {
	var m map[string]any
	b, err := os.ReadFile(path)
	if err != nil {
		return map[string]any{}
	}
	if json.Unmarshal(b, &m) != nil || m == nil {
		return map[string]any{}
	}
	return m
}

// dropStaleNotes removes per-file notes whose hunk headers moved; those notes
// describe hunks that no longer exist, so the agent has to rewrite them.
func dropStaleNotes(notes map[string]any, old, new []diff.File) []string {
	nf, ok := notes["files"].(map[string]any)
	if !ok {
		return nil
	}
	var dropped []string
	for path := range nf {
		if !sameHunks(find(old, path), find(new, path)) {
			delete(nf, path)
			dropped = append(dropped, path)
		}
	}
	slices.Sort(dropped) // map order is random; the oracle printed file order
	return dropped
}

func find(files []diff.File, path string) *diff.File {
	for i := range files {
		if files[i].Path == path {
			return &files[i]
		}
	}
	return nil
}

func sameHunks(a, b *diff.File) bool {
	if a == nil || b == nil || len(a.Hunks) != len(b.Hunks) {
		return false
	}
	for i := range a.Hunks {
		if a.Hunks[i].Header != b.Hunks[i].Header {
			return false
		}
	}
	return true
}

// Reanchor re-points comments at the line with the same content; a comment
// whose line is gone is marked stale rather than dropped.
func Reanchor(files []diff.File, comments []map[string]any) {
	for _, c := range comments {
		target, _ := c["target"].(string)
		path, _ := c["file"].(string)
		f := find(files, path)
		if target != "line" {
			if target == "hunk" {
				c["stale"] = !(f != nil && asInt(c["hunk"]) < len(f.Hunks))
			}
			continue
		}
		if f == nil {
			c["stale"] = true
			continue
		}
		if hi, line, ok := bestLine(f, c); ok {
			c["hunk"], c["line"], c["stale"] = hi, line, false
		} else {
			c["stale"] = true
		}
	}
}

func bestLine(f *diff.File, c map[string]any) (int, int, bool) {
	side, _ := c["side"].(string)
	snippet, _ := c["snippet"].(string)
	want := asInt(c["line"])
	bestDist, bestHunk, bestNum, found := 0, 0, 0, false
	for hi := range f.Hunks {
		for _, ln := range f.Hunks[hi].Lines {
			n := ln.O
			if side == "new" {
				n = ln.N
			}
			if n == nil || ln.S != snippet {
				continue
			}
			if (side == "new" && ln.T == "-") || (side == "old" && ln.T == "+") {
				continue
			}
			if d := abs(*n - want); !found || d < bestDist {
				bestDist, bestHunk, bestNum, found = d, hi, *n, true
			}
		}
	}
	return bestHunk, bestNum, found
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
