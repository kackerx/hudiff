package review

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kackerx/hudiff/internal/session"
)

type loc struct {
	file, side string
	line       any
}

func locOf(it Item) loc {
	side := str(it, "side")
	if side == "" {
		side = "new"
	}
	return loc{str(it, "file"), side, fmt.Sprint(it["line"])}
}

func round(rv Report) []any {
	r, _ := rv["rounds"].([]any)
	return r
}

// Merge folds a new round into the existing report. Nothing the user touched is
// lost: old items stay, their status and resolution stay, and a new item that
// points at the same place as an old one updates its text instead of piling up.
func Merge(old, new Report, replace bool) Report {
	if replace || old == nil {
		return firstRound(new)
	}
	if len(round(old)) == 0 {
		seedRound(old)
	}
	rnd := len(round(old)) + 1
	kept := items(old)
	byID := map[string]Item{}
	used := map[string]bool{}
	for _, it := range kept {
		byID[str(it, "id")] = it
		used[str(it, "id")] = true
	}
	author := str(new, "author")
	added, updated, noted := 0, 0, 0

	for _, it := range items(new) {
		it["round"] = rnd
		if noteOnly(it) {
			if cur := byID[str(it, "id")]; cur != nil {
				appendNote(cur, author, rnd, str(it, "note"), str(it, "verdict"))
				noted++
			}
			continue
		}
		takeNote(it, author, rnd)
		cur := byID[str(it, "id")]
		if cur == nil || locOf(cur) != locOf(it) {
			if same := sameSpot(kept, it); same != nil {
				cur = same
				it["id"] = str(same, "id")
			}
		}
		if cur != nil && locOf(cur) == locOf(it) {
			for _, k := range []string{"title", "body", "suggestion", "priority", "snippet"} {
				if v, ok := it[k]; ok && v != nil && v != "" {
					cur[k] = v
				}
			}
			if _, ok := cur["round"]; !ok {
				cur["round"] = 1
			}
			updated++
			continue
		}
		if cur != nil {
			it["id"] = freeID(used)
		}
		used[str(it, "id")] = true
		kept = append(kept, it)
		byID[str(it, "id")] = it
		added++
	}

	setItems(old, kept)
	if author != "" {
		old["author"] = author
	}
	if s := str(new, "summary"); s != "" {
		old["summary"] = s
	}
	if s, _ := new["strengths"].([]any); len(s) > 0 {
		old["strengths"] = s
	}
	if v, ok := new["verdict"]; ok && v != nil {
		old["verdict"] = v
	}
	old["rounds"] = append(round(old), roundRow(rnd, new, added, updated))
	old["_merge"] = map[string]any{"round": rnd, "added": added, "updated": updated,
		"noted": noted, "kept": len(kept) - added}
	return old
}

func noteOnly(it Item) bool {
	b, _ := it["_note_only"].(bool)
	delete(it, "_note_only")
	return b
}

// takeNote moves an inline `note` on a real item into its notes list.
func takeNote(it Item, author string, rnd int) {
	if it["note"] == nil {
		return
	}
	body, verdict := str(it, "note"), str(it, "verdict")
	delete(it, "note")
	delete(it, "verdict")
	appendNote(it, author, rnd, body, verdict)
}

func appendNote(it Item, author string, rnd int, body, verdict string) {
	if author == "" {
		author = "reviewer"
	}
	if verdict == "" {
		verdict = "info"
	}
	notes, _ := it["notes"].([]any)
	it["notes"] = append(notes, map[string]any{
		"who": author, "round": rnd, "body": body, "verdict": verdict, "ts": session.NowISO()})
}

// sameSpot is the reviewer-renumbered case: same file:line and same title.
func sameSpot(kept []Item, it Item) Item {
	if str(it, "file") == "" {
		return nil
	}
	for _, x := range kept {
		if locOf(x) == locOf(it) && x["title"] == it["title"] {
			return x
		}
	}
	return nil
}

func freeID(used map[string]bool) string {
	for n := 1; ; n++ {
		id := "r" + strconv.Itoa(n)
		if !used[id] {
			return id
		}
	}
}

func firstRound(new Report) Report {
	var keep []Item
	for _, it := range items(new) {
		if noteOnly(it) {
			continue
		}
		if _, ok := it["round"]; !ok {
			it["round"] = 1
		}
		takeNote(it, str(new, "author"), 1)
		keep = append(keep, it)
	}
	setItems(new, keep)
	new["rounds"] = []any{roundRow(1, new, len(keep), 0)}
	return new
}

// seedRound treats a report written before rounds existed as round 1.
func seedRound(old Report) {
	old["rounds"] = []any{map[string]any{
		"round": 1, "author": old["author"], "at": old["updated"],
		"summary": str(old, "summary"), "strengths": old["strengths"], "added": len(items(old))}}
	for _, it := range items(old) {
		if _, ok := it["round"]; !ok {
			it["round"] = 1
		}
	}
}

func roundRow(rnd int, rv Report, added, updated int) map[string]any {
	row := map[string]any{"round": rnd, "author": rv["author"], "at": session.NowISO(),
		"summary": str(rv, "summary"), "strengths": rv["strengths"], "added": added}
	if updated > 0 || rnd > 1 {
		row["updated"] = updated
	}
	return row
}

type Counts struct{ High, Medium, Low, Open, Done, Wontfix int }

func Count(rv Report) Counts {
	var c Counts
	for _, it := range items(rv) {
		st := str(it, "status")
		if st == "" {
			st = "open"
		}
		switch st {
		case "open":
			c.Open++
			switch str(it, "priority") {
			case "high":
				c.High++
			case "medium":
				c.Medium++
			case "low":
				c.Low++
			}
		case "done":
			c.Done++
		case "wontfix":
			c.Wontfix++
		}
	}
	return c
}

// Markdown exports the report in the template the user asked for.
func Markdown(rv Report) string {
	out := []string{}
	if v, _ := rv["verdict"].(map[string]any); v != nil {
		out = append(out, "## Verdict\n", fmt.Sprintf("**%s**: %s\n", Decisions[str(v, "decision")], strings.TrimSpace(str(v, "reason"))))
	}
	out = append(out, "## Overall\n", strings.TrimSpace(str(rv, "summary"))+"\n", "## Strengths\n")
	strengths, _ := rv["strengths"].([]any)
	if len(strengths) == 0 {
		out = append(out, "- (none)")
	}
	for _, s := range strengths {
		out = append(out, "- "+fmt.Sprint(s))
	}
	out = append(out, "\n## Items\n")
	for _, pr := range order {
		out = append(out, priorityBlock(rv, pr)...)
	}
	return strings.Join(out, "\n")
}

func priorityBlock(rv Report, pr string) []string {
	var its []Item
	for _, it := range items(rv) {
		if str(it, "priority") == pr {
			its = append(its, it)
		}
	}
	if len(its) == 0 {
		return nil
	}
	out := []string{fmt.Sprintf("### [%s]\n", prioLabel[pr])}
	for _, it := range its {
		out = append(out, itemMarkdown(it)...)
	}
	return out
}

func itemMarkdown(it Item) []string {
	box := " "
	switch str(it, "status") {
	case "done":
		box = "x"
	case "wontfix":
		box = "~"
	}
	locTxt := ""
	if f := str(it, "file"); f != "" {
		locTxt = "`" + f + "`"
		if it["line"] != nil {
			locTxt += fmt.Sprintf(" L%v", it["line"])
		}
	}
	out := []string{fmt.Sprintf("- [%s] **%v**  %s  `%s`", box, it["title"], locTxt, str(it, "id"))}
	if s := str(it, "snippet"); s != "" {
		out = append(out, "  ```\n  "+s+"\n  ```")
	}
	for _, k := range []string{"body", "suggestion"} {
		if v := strings.TrimSpace(str(it, k)); v != "" {
			prefix := "  "
			if k == "suggestion" {
				prefix = "  **Suggested fix**: "
			}
			out = append(out, prefix+strings.ReplaceAll(v, "\n", "\n  "))
		}
	}
	if r := str(it, "resolution"); r != "" {
		out = append(out, fmt.Sprintf("  ✔ resolution (%s): %s", str(it, "resolved_by"), r))
	}
	notes, _ := it["notes"].([]any)
	for _, n := range notes {
		m, _ := n.(map[string]any)
		mark := "·"
		switch str(m, "verdict") {
		case "ok":
			mark = "✓"
		case "disagree":
			mark = "⚠"
		}
		out = append(out, fmt.Sprintf("  - %s re-review (%s, round %v): %s", mark, str(m, "who"), m["round"], str(m, "body")))
	}
	return append(out, "")
}
