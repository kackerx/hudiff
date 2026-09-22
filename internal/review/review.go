// Package review owns review.json: normalising what a reviewer agent writes,
// and merging a new round into the existing report without losing the user's
// ticks (specs/spec.md §8.5).
package review

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kackerx/hudiff/internal/diff"
)

type Item = map[string]any
type Report = map[string]any

var prioAlias = map[string]string{
	"高": "high", "高优": "high", "h": "high", "p0": "high",
	"中": "medium", "中优": "medium", "m": "medium", "p1": "medium",
	"低": "low", "低优": "low", "l": "low", "p2": "low",
}

var prioLabel = map[string]string{"high": "高优", "medium": "中优", "low": "低优"}

var order = []string{"high", "medium", "low"}

func Load(dir string) Report {
	b, err := os.ReadFile(filepath.Join(dir, "review.json"))
	if err != nil {
		return nil
	}
	var rv Report
	if json.Unmarshal(b, &rv) != nil {
		return nil
	}
	return rv
}

func Path(dir string) string { return filepath.Join(dir, "review.json") }

func str(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return s
}

func items(rv Report) []Item {
	arr, _ := rv["items"].([]any)
	out := make([]Item, 0, len(arr))
	for _, x := range arr {
		if it, ok := x.(Item); ok {
			out = append(out, it)
		}
	}
	return out
}

func setItems(rv Report, its []Item) {
	arr := make([]any, len(its))
	for i, it := range its {
		arr[i] = it
	}
	rv["items"] = arr
}

// Validate normalises priorities and anchors file:line to a hunk, the way the
// oracle did. An item that carries only a note is a re-review of an existing
// item and is marked as such for Merge.
func Validate(rv Report, d diff.Diff) []string {
	var problems []string
	if rv == nil {
		return []string{"review must be an object"}
	}
	byPath := map[string]diff.File{}
	for _, f := range d.Files {
		byPath[f.Path] = f
	}
	if _, ok := rv["summary"]; !ok {
		rv["summary"] = ""
	}
	if s, ok := rv["strengths"].(string); ok {
		rv["strengths"] = []any{s}
	}
	if _, ok := rv["strengths"]; !ok {
		rv["strengths"] = []any{}
	}
	if raw, ok := rv["items"]; ok && raw != nil {
		if _, isList := raw.([]any); !isList {
			return []string{"review.items must be an array"}
		}
	}
	seen := map[string]bool{}
	for i, it := range items(rv) {
		problems = append(problems, validateItem(it, i, byPath, seen)...)
	}
	return problems
}

func validateItem(it Item, i int, byPath map[string]diff.File, seen map[string]bool) []string {
	tag := fmt.Sprintf("items[%d]", i)
	var problems []string
	pr := strings.ToLower(str(it, "priority"))
	if alias, ok := prioAlias[pr]; ok {
		pr = alias
	}
	noteOnly := it["note"] != nil && it["title"] == nil
	if _, ok := prioLabel[pr]; !ok && !noteOnly {
		problems = append(problems, tag+": priority must be high / medium / low")
	}
	it["priority"] = pr
	if str(it, "id") == "" {
		it["id"] = "r" + strconv.Itoa(i+1)
	}
	if seen[str(it, "id")] {
		problems = append(problems, tag+": duplicate id "+str(it, "id"))
	}
	seen[str(it, "id")] = true
	if noteOnly {
		it["_note_only"] = true
		if v := str(it, "verdict"); v != "" && v != "ok" && v != "disagree" && v != "info" {
			problems = append(problems, tag+": verdict must be ok / disagree / info")
		}
		return problems
	}
	if it["title"] == nil {
		problems = append(problems, tag+": missing title (the problem in one sentence)")
	}
	for k, def := range map[string]any{"body": "", "suggestion": "", "status": "open"} {
		if _, ok := it[k]; !ok {
			it[k] = def
		}
	}
	if st := str(it, "status"); st != "open" && st != "done" && st != "wontfix" {
		problems = append(problems, tag+": status must be open / done / wontfix")
	}
	return append(problems, anchor(it, tag, byPath)...)
}

// anchor resolves file:line to a hunk and records the source line, so the page
// can show the item next to the code.
func anchor(it Item, tag string, byPath map[string]diff.File) []string {
	file := str(it, "file")
	if file == "" {
		return nil
	}
	f, ok := byPath[file]
	if !ok {
		return []string{tag + ": file is not in the diff: " + file}
	}
	raw, ok := it["line"]
	if !ok || raw == nil {
		return nil
	}
	want, _ := raw.(float64)
	side := str(it, "side")
	if side == "" {
		side = "new"
	}
	for hi := range f.Hunks {
		for _, ln := range f.Hunks[hi].Lines {
			n := ln.O
			skip := ln.T == "+"
			if side == "new" {
				n, skip = ln.N, ln.T == "-"
			}
			if n == nil || *n != int(want) || skip {
				continue
			}
			it["side"], it["hunk"] = side, hi
			if _, has := it["snippet"]; !has {
				it["snippet"] = ln.S
			}
			return nil
		}
	}
	return []string{fmt.Sprintf("%s: %s %s L%v is not in the diff (use the line numbers from hudiff read, new column)", tag, file, side, raw)}
}
