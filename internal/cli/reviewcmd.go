package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/kackerx/hudiff/internal/daemon"
	"github.com/kackerx/hudiff/internal/review"
	"github.com/kackerx/hudiff/internal/session"
)

func (c *ReviewShowCmd) Run(g *Globals) error {
	_, dir, err := session.Current(g.Session)
	if err != nil {
		return usage("%v", err)
	}
	rv := review.Load(dir)
	if rv == nil {
		if c.JSON {
			return printJSON(nil)
		}
		fmt.Println("(no review report yet)")
		return nil
	}
	if c.JSON {
		return printJSON(rv)
	}
	printReview(rv, c.Open)
	return nil
}

func printReview(rv review.Report, onlyOpen bool) {
	cnt := review.Count(rv)
	fmt.Printf("# review by %v  updated %v\n", rv["author"], rv["updated"])
	if v, _ := rv["verdict"].(map[string]any); v != nil {
		fmt.Printf("verdict: %v — %v\n", v["decision"], v["reason"])
	}
	fmt.Printf("%v\n\n", rv["summary"])
	if s, _ := rv["strengths"].([]any); len(s) > 0 {
		fmt.Println("strengths:")
		for _, x := range s {
			fmt.Printf("  - %v\n", x)
		}
		fmt.Println()
	}
	fmt.Printf("open %d (high %d / medium %d / low %d)  done %d  wontfix %d\n",
		cnt.Open, cnt.High, cnt.Medium, cnt.Low, cnt.Done, cnt.Wontfix)
	arr, _ := rv["items"].([]any)
	for _, x := range arr {
		it, _ := x.(review.Item)
		st, _ := it["status"].(string)
		if st == "" {
			st = "open"
		}
		if onlyOpen && st != "open" {
			continue
		}
		fmt.Printf("\n[%v] %v  %v  %v\n", it["id"], st, it["priority"], it["title"])
		if it["file"] != nil {
			fmt.Printf("    %v", it["file"])
			if it["line"] != nil {
				fmt.Printf(" L%v", it["line"])
			}
			fmt.Printf("  (hunk %v)\n", it["hunk"])
		}
		for _, k := range []string{"body", "suggestion", "resolution"} {
			if v, _ := it[k].(string); strings.TrimSpace(v) != "" {
				fmt.Printf("    %s: %s\n", k, v)
			}
		}
	}
}

func (c *ReviewSetCmd) Run(g *Globals) error {
	_, dir, err := session.Current(g.Session)
	if err != nil {
		return usage("%v", err)
	}
	text, err := readFileArg(c.Path)
	if err != nil {
		return err
	}
	var rv review.Report
	if err := json.Unmarshal([]byte(text), &rv); err != nil {
		return usage("review is not valid JSON: %v", err)
	}
	if c.Author != "" {
		rv["author"] = c.Author
	}
	out, problems, err := review.Set(dir, rv, c.Replace, c.Force)
	for _, p := range problems {
		fmt.Fprintln(os.Stderr, "✗ "+p)
	}
	if err != nil {
		return usage("not written; pass --force to write anyway")
	}
	cnt := review.Count(out)
	m, _ := out["_merge"].(map[string]any)
	if m != nil {
		fmt.Printf("round %v: %v added, %v updated, %v re-review notes, %v kept; open %d\n",
			m["round"], m["added"], m["updated"], m["noted"], m["kept"], cnt.Open)
		return nil
	}
	fmt.Printf("wrote review.json: %d items, open %d\n", cnt.Open+cnt.Done+cnt.Wontfix, cnt.Open)
	return nil
}

func (c *ReviewDoneCmd) Run(g *Globals) error {
	return setReviewStatus(g, c.ID, c.Text, c.By, "done")
}

func (c *ReviewWontfixCmd) Run(g *Globals) error {
	return setReviewStatus(g, c.ID, c.Text, c.By, "wontfix")
}

func (c *ReviewReopenCmd) Run(g *Globals) error {
	return setReviewStatus(g, c.ID, c.Text, c.By, "open")
}

func setReviewStatus(g *Globals, id, arg, by, status string) error {
	_, dir, err := session.Current(g.Session)
	if err != nil {
		return usage("%v", err)
	}
	text, err := readTextArg(arg)
	if err != nil {
		return err
	}
	if _, err := review.SetStatus(dir, id, status, text, by); err != nil {
		return usage("%v", err)
	}
	fmt.Printf("%s → %s\n", id, status)
	return nil
}

func (c *ReviewNoteCmd) Run(g *Globals) error {
	_, dir, err := session.Current(g.Session)
	if err != nil {
		return usage("%v", err)
	}
	text, err := readTextArg(c.Text)
	if err != nil {
		return err
	}
	if _, err := review.Note(dir, c.ID, text, c.Verdict, c.By); err != nil {
		return usage("%v", err)
	}
	fmt.Printf("%s → note added (%s)\n", c.ID, c.Verdict)
	return nil
}

func (c *ReviewMarkdownCmd) Run(g *Globals) error {
	_, dir, err := session.Current(g.Session)
	if err != nil {
		return usage("%v", err)
	}
	rv := review.Load(dir)
	if rv == nil {
		return usage("no review report yet")
	}
	fmt.Println(review.Markdown(rv))
	return nil
}

func (c *ExportHTMLCmd) Run(g *Globals) error {
	_, dir, err := session.Current(g.Session)
	if err != nil {
		return usage("%v", err)
	}
	b, err := daemon.ExportHTML(dir, c.Font)
	if err != nil {
		return envErr("%v", err)
	}
	if c.Out == "" {
		_, err = os.Stdout.Write(b)
	} else {
		err = os.WriteFile(c.Out, b, 0o644)
	}
	if err != nil {
		return envErr("%v", err)
	}
	fmt.Fprintf(os.Stderr, "exported %d bytes\n", len(b))
	if len(b) > 1<<20 {
		fmt.Fprintln(os.Stderr, "hudiff: over 1MB; gist's web view truncates there (the raw URL does not)")
	}
	return nil
}
