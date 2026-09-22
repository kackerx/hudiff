package diff

import (
	"regexp"
	"strconv"
	"strings"
)

var hunkRe = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@ ?(.*)$`)

// splitLines splits on \n and drops a trailing \r, which is all a real patch
// needs. ponytail: skips Python splitlines' \v \f \x1c-\x1e \x85 \u2028 \u2029 —
// the oracle mangles the hunk on those anyway; revisit if a patch ever has one.
func splitLines(s string) []string {
	out := strings.Split(s, "\n")
	if n := len(out); n > 0 && out[n-1] == "" {
		out = out[:n-1]
	}
	for i, l := range out {
		out[i] = strings.TrimSuffix(l, "\r")
	}
	return out
}

func atoiOr(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// ParsePatch turns a git-style unified diff into the diff.json model. Hunk
// order is positional and is the key notes.json writes against, so this has to
// stay identical to the oracle's parse_patch().
func ParsePatch(text string) []File {
	files := []File{}
	var cur *File
	var hunk *Hunk
	old, new := 0, 0

	for _, raw := range splitLines(text) {
		if strings.HasPrefix(raw, "diff --git ") {
			hunk = nil
			files = append(files, File{Status: "M", Hunks: []Hunk{}})
			cur = &files[len(files)-1]
			if m := gitHeaderRe.FindStringSubmatch(raw); m != nil {
				cur.OldPath, cur.Path = m[1], m[2]
			}
			continue
		}
		if cur == nil {
			continue
		}
		if hunk == nil {
			applyFileHeader(cur, raw)
		}
		if m := hunkRe.FindStringSubmatch(raw); m != nil {
			old, new = atoiOr(m[1], 0), atoiOr(m[3], 0)
			cur.Hunks = append(cur.Hunks, Hunk{
				Header: raw, OldStart: old, OldCount: atoiOr(m[2], 1),
				NewStart: new, NewCount: atoiOr(m[4], 1), Section: m[5], Lines: []Line{},
			})
			hunk = &cur.Hunks[len(cur.Hunks)-1]
			continue
		}
		if hunk == nil {
			continue
		}
		if strings.HasPrefix(raw, `\`) { // "\ No newline at end of file"
			if n := len(hunk.Lines); n > 0 {
				hunk.Lines[n-1].NoNL = true
			}
			continue
		}
		t, s := " ", ""
		if rs := []rune(raw); len(rs) > 0 {
			t, s = string(rs[0]), string(rs[1:])
		}
		switch t {
		case " ":
			hunk.Lines = append(hunk.Lines, Line{T: " ", O: ptrInt(old), N: ptrInt(new), S: s})
			old, new = old+1, new+1
		case "+":
			hunk.Lines = append(hunk.Lines, Line{T: "+", N: ptrInt(new), S: s})
			new++
			cur.Add++
		case "-":
			hunk.Lines = append(hunk.Lines, Line{T: "-", O: ptrInt(old), S: s})
			old++
			cur.Del++
		default:
			hunk = nil
		}
	}
	for i := range files {
		if files[i].Path == "" {
			files[i].Path = files[i].OldPath
		}
	}
	return files
}

var gitHeaderRe = regexp.MustCompile(`^diff --git a/(.*) b/(.*)$`)

func applyFileHeader(f *File, raw string) {
	switch {
	case strings.HasPrefix(raw, "new file mode"):
		f.Status = "A"
	case strings.HasPrefix(raw, "deleted file mode"):
		f.Status = "D"
	case strings.HasPrefix(raw, "rename from "):
		f.OldPath = strings.TrimPrefix(raw, "rename from ")
		f.Status = "R"
	case strings.HasPrefix(raw, "rename to "):
		f.Path = strings.TrimPrefix(raw, "rename to ")
	case strings.HasPrefix(raw, "Binary files"), strings.HasPrefix(raw, "GIT binary patch"):
		f.Binary = true
	case strings.HasPrefix(raw, "--- "):
		if p := raw[4:]; p != "/dev/null" {
			f.OldPath = strings.TrimPrefix(p, "a/")
		}
	case strings.HasPrefix(raw, "+++ "):
		if p := raw[4:]; p != "/dev/null" {
			f.Path = strings.TrimPrefix(p, "b/")
		}
	}
}

func ptrInt(n int) *int { return &n }
