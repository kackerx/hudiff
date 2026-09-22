package review

import (
	"fmt"

	"github.com/kackerx/hudiff/internal/diff"
	"github.com/kackerx/hudiff/internal/session"
)

// Set merges a round in and writes it. Validation problems are returned rather
// than swallowed: a reviewer agent wrote this.
func Set(dir string, incoming Report, replace, force bool) (Report, []string, error) {
	mu := session.Lock(dir)
	mu.Lock()
	defer mu.Unlock()

	problems := Validate(incoming, diff.Load(dir))
	if len(problems) > 0 && !force {
		return nil, problems, fmt.Errorf("validation failed")
	}
	rv := Merge(Load(dir), incoming, replace)
	rv["updated"] = session.NowISO()
	return rv, problems, session.WriteJSON(Path(dir), rv)
}

// SetStatus is the tick on a review item: done / wontfix / reopen.
func SetStatus(dir, id, status, resolution, by string) (Item, error) {
	mu := session.Lock(dir)
	mu.Lock()
	defer mu.Unlock()

	rv := Load(dir)
	it := byID(rv, id)
	if it == nil {
		return nil, fmt.Errorf("no such review item: %s", id)
	}
	if status != "open" && status != "done" && status != "wontfix" {
		return nil, fmt.Errorf("status must be open / done / wontfix")
	}
	it["status"] = status
	if status == "open" {
		delete(it, "resolution")
		delete(it, "resolved_by")
		delete(it, "resolved_at")
	} else {
		it["resolution"] = resolution
		it["resolved_by"] = by
		it["resolved_at"] = session.NowISO()
	}
	return it, session.WriteJSON(Path(dir), rv)
}

// Note appends a re-review remark to one item.
func Note(dir, id, body, verdict, who string) (Item, error) {
	mu := session.Lock(dir)
	mu.Lock()
	defer mu.Unlock()

	if verdict != "" && verdict != "ok" && verdict != "disagree" && verdict != "info" {
		return nil, fmt.Errorf("verdict must be ok / disagree / info")
	}
	rv := Load(dir)
	it := byID(rv, id)
	if it == nil {
		return nil, fmt.Errorf("no such review item: %s", id)
	}
	rnd := len(round(rv))
	if rnd == 0 {
		rnd = 1
	}
	appendNote(it, who, rnd, body, verdict)
	return it, session.WriteJSON(Path(dir), rv)
}

func byID(rv Report, id string) Item {
	for _, it := range items(rv) {
		if str(it, "id") == id {
			return it
		}
	}
	return nil
}
