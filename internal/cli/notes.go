package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/kackerx/hudiff/internal/diagram"
	"github.com/kackerx/hudiff/internal/diff"
	"github.com/kackerx/hudiff/internal/session"
)

var hunkRef = regexp.MustCompile(`^(.+)#(\d+)$`)

func compileD2(src string) error {
	if strings.TrimSpace(src) == "" {
		return nil // "missing src" is already reported
	}
	return diagram.CheckD2(src)
}

func (c *NotesCmd) Run(g *Globals) error {
	sid, dir, err := session.Current(g.Session)
	if err != nil {
		return usage("%v", err)
	}
	text, err := readFileArg(c.Path)
	if err != nil {
		return err
	}
	var notes map[string]any
	if err := json.Unmarshal([]byte(text), &notes); err != nil {
		return usage("notes is not valid JSON: %v", err)
	}
	d := diff.Load(dir)
	problems, covered, skipped := validateNotes(notes, d)
	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, "✗ "+p)
		}
		if !c.Force {
			return usage("not written; pass --force to write anyway")
		}
	}
	sent, err := call(sid, "POST", "/api/notes", notes)
	if err != nil {
		return usage("%v", err)
	}
	if !sent {
		if err := session.WriteNotes(dir, notes); err != nil {
			return usage("%v", err)
		}
	}
	fmt.Println(notesSummary(notes, d, covered, skipped))
	return nil
}

func readFileArg(path string) (string, error) {
	if path == "-" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", usage("cannot read stdin: %v", err)
		}
		return string(b), nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", usage("cannot read %s: %v", path, err)
	}
	return string(b), nil
}

// validateNotes is a trust boundary: an agent wrote this JSON, a hunk key that
// does not exist would silently drop an explanation, and every d2 source is
// really compiled before it lands.
func validateNotes(notes map[string]any, d diff.Diff) (problems []string, covered, skipped int) {
	byPath := map[string]diff.File{}
	for _, f := range d.Files {
		byPath[f.Path] = f
	}
	files, _ := notes["files"].(map[string]any)
	if _, isList := notes["files"].([]any); isList {
		return []string{"notes.files must be an object keyed by path, not an array"}, 0, 0
	}
	for _, path := range sortedKeys(files) {
		f, ok := byPath[path]
		if !ok {
			problems = append(problems, "file is not in the diff: "+path)
			continue
		}
		fn, _ := files[path].(map[string]any)
		hunks, _ := fn["hunks"].(map[string]any)
		for _, k := range sortedKeys(hunks) {
			n, err := strconv.Atoi(k)
			if err != nil || n >= len(f.Hunks) {
				problems = append(problems, fmt.Sprintf("%s#%s: no such hunk (%d in this file)", path, k, len(f.Hunks)))
				continue
			}
			switch hn := hunks[k].(type) {
			case nil:
				skipped++
			case map[string]any:
				if skip, _ := hn["skip"].(bool); skip {
					skipped++
					continue
				}
				if hn["body"] == nil && hn["title"] == nil {
					problems = append(problems, fmt.Sprintf("%s#%s: empty note (write null for a hunk that needs none)", path, k))
				}
				covered++
			default:
				problems = append(problems, fmt.Sprintf("%s#%s: note must be an object or null", path, k))
			}
		}
	}
	return append(problems, validateDiagrams(notes, byPath)...), covered, skipped
}

func validateDiagrams(notes map[string]any, byPath map[string]diff.File) []string {
	raw, ok := notes["diagrams"]
	if !ok || raw == nil {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		return []string{"notes.diagrams must be an array"}
	}
	var problems []string
	seen := map[string]bool{"auto-map": true}
	for i, item := range list {
		tag := fmt.Sprintf("diagrams[%d]", i)
		dg, ok := item.(map[string]any)
		if !ok {
			problems = append(problems, tag+": not an object")
			continue
		}
		src, _ := dg["src"].(string)
		if strings.TrimSpace(src) == "" {
			problems = append(problems, tag+": missing src")
		}
		if dg["title"] == nil {
			problems = append(problems, tag+": missing title")
		}
		if kind, _ := dg["kind"].(string); kind != "" && kind != "d2" {
			problems = append(problems, tag+": kind must be d2")
		} else if err := compileD2(src); err != nil {
			problems = append(problems, fmt.Sprintf("%s (%v): d2 failed to compile — %v", tag, dg["title"], err))
		}
		id, _ := dg["id"].(string)
		if id == "" {
			id = "d" + strconv.Itoa(i)
		}
		if seen[id] {
			problems = append(problems, fmt.Sprintf("%s: duplicate or reserved id (%s)", tag, id))
		}
		seen[id] = true
		problems = append(problems, badHunkRefs(tag, dg["hunks"], byPath)...)
	}
	return problems
}

func badHunkRefs(tag string, raw any, byPath map[string]diff.File) []string {
	refs, _ := raw.([]any)
	var problems []string
	for _, r := range refs {
		ref := fmt.Sprint(r)
		m := hunkRef.FindStringSubmatch(ref)
		if m == nil {
			problems = append(problems, fmt.Sprintf("%s: %q in hunks is not <path>#<n>", tag, ref))
			continue
		}
		f, ok := byPath[m[1]]
		n, _ := strconv.Atoi(m[2])
		if !ok || n >= len(f.Hunks) {
			problems = append(problems, fmt.Sprintf("%s: %q in hunks does not exist", tag, ref))
		}
	}
	return problems
}

func notesSummary(notes map[string]any, d diff.Diff, covered, skipped int) string {
	files, _ := notes["files"].(map[string]any)
	total := 0
	var missing []string
	for _, f := range d.Files {
		total += len(f.Hunks)
		fn, _ := files[f.Path].(map[string]any)
		hunks, _ := fn["hunks"].(map[string]any)
		for i := range f.Hunks {
			if _, ok := hunks[strconv.Itoa(i)]; !ok {
				missing = append(missing, fmt.Sprintf("%s#%d", f.Path, i))
			}
		}
	}
	s := fmt.Sprintf("wrote notes.json: %d explained, %d skipped, %d hunks total", covered, skipped, total)
	if dgs, _ := notes["diagrams"].([]any); len(dgs) > 0 {
		s += fmt.Sprintf("; %d diagrams", len(dgs))
	}
	if len(missing) > 0 {
		shown := missing
		tail := ""
		if len(shown) > 12 {
			shown, tail = shown[:12], " …"
		}
		s += "; still unexplained: " + strings.Join(shown, ", ") + tail
	}
	return s
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
