---
name: hudiff
description: "Explain a diff with line-by-line comments. Use when the user wants to *understand* a PR / commit / working-tree change: hudiff opens a local page, I write a note per hunk, the user comments on individual lines like a GitHub PR, and the comments flow back to me through `hudiff wait` / `hudiff comments` so I can answer or fix the code. Use when the user says /hudiff, 'walk me through this diff', 'look at this PR', 'explain this change', 'did you see my comments'."
user_invocable: true
triggers:
  - walk me through this diff
  - explain this change
  - look at this PR
  - check my comments
---

# hudiff — explain a diff, collect the user's line-by-line notes

The CLI is `hudiff` (must be on PATH); `hudiff -h` lists every subcommand.
Division of labour: **the CLI fetches the diff, serves the page and stores the
comments; I write the explanation and answer the comments.** The page polls on
its own — once I write notes it refreshes, so never tell the user to reload.

One local daemon serves every session (default `http://127.0.0.1:7788/`,
`HUDIFF_PORT` to change it). The home page lists sessions, each session lives at
`/s/<id>/`. Any subcommand starts the daemon if it is not running.

Usage: `/hudiff [target] [-- review request]`

    /hudiff                    working tree, uncommitted (staged and untracked included)
    /hudiff --staged           staged only
    /hudiff commit HEAD~1      one commit
    /hudiff range a1b2c3..HEAD a range
    /hudiff pr 562             a GitHub PR (also #562 or a PR URL)
    /hudiff patch x.patch      a patch file, or - for stdin
    /hudiff files old/ new/    two files or two directories on disk, no repo needed
    /hudiff comments           skip the explanation, go straight to the user's comments
    /hudiff commit HEAD~1 -- focus on concurrency and error handling, skip styling, see docs/spec.md
    /hudiff review <session>   cross-review: I am a *different* agent re-reviewing someone
                               else's session and writing a report (flow C)

Everything after `--` is **this run's review request**, passed through verbatim
as `-p`; files it mentions go in with `-c <path>` (repeatable). Presets may
apply even without one: `<config>/hudiff/prompts/default.md` and
`<config>/hudiff/prompts/<repo name>.md` are picked up when they exist. The
request shows at the top of `hudiff read` and on the page — **write the notes to
match it**, and use `null` for hunks it says not to bother with.

## Flow A: explain a diff

1. **Open a session**: `hudiff <target> --json [-p "<request>"] [-c <file>]...`.
   The output carries `url`, `dir`, `meta`, the file list and `next`. The browser
   opens on its own. Tell the user the url in one line, then start writing
   immediately — they are watching. Read anything in `brief.context` first.
   Creating is **idempotent**: the same commit / PR / range / working tree reuses
   its session (`reused: true`); a changed working tree is refreshed in place
   (`refreshed: true`, and `notes_dropped` lists the files whose notes are now
   void). On reuse, notes that survived are still there — only rewrite the
   dropped ones, do not start over.
   **To look up a session (directory, url, counts) use `hudiff -s <id> session
   info --json`, never create it again.**
2. **Read the diff with repo context** — notes written from the diff text alone
   are just a restatement of the code. This step decides the quality:
   - `hudiff read` for the whole thing. Every hunk is tagged `[path#n]`, and that
     `n` is the key in notes.json. The two number columns are old | new.
   - `hudiff context` is **not optional**: per file it gives recent history (the
     last 4 commits — the motive is often right there), which symbols this change
     adds (+) / removes (-) / edits (~), who references them outside the changed
     files (the blast radius), and whether the related tests moved too. Lines
     marked ⚠ (a symbol removed while something still calls it; no test coverage)
     **must** be called out in the matching note.
   - For a non-trivial hunk, `hudiff read -W --file <path>` shows the enclosing
     function or block (lines starting with `~` are context from the new file, not
     part of the diff), or `-U 20` for 20 lines either side. Whether a change is
     correct is a question about its function, not about those three lines.
   - Read the referencing files `context` lists, the files in `brief.context`, the
     commit message, the PR description. Do not guess.
   - Split a big diff by file: `read --file` / `context --file`.
3. **Write the notes**: build the JSON, write it to a temp file, then
   `hudiff notes <file>`. It checks that every path and hunk index exists,
   compiles every d2 source, and prints the hunks still uncovered. Schema:

   ```json
   {
     "title": "one line, page header, keep it short",
     "summary": "1–3 sentences of markdown: what changed and why",
     "diagrams": [
       {"id": "flow", "title": "diagram title", "kind": "d2",
        "caption": "one or two lines: which question this answers",
        "hunks": ["internal/cli/config.go#0", "internal/cli/memory.go#1"],
        "src": "direction: right\n..."}
     ],
     "files": {
       "internal/cli/config.go": {
         "summary": "optional, one line on this file's role in the change",
         "hunks": {
           "0": {"title": "short title", "body": "markdown", "lines": "new:42-47", "tags": ["#562"]},
           "1": null
         }
       }
     }
   }
   ```
   `lines` is optional and points at the few lines that matter
   (`new:42-47` / `old:10-12`); hovering the card highlights them. `tags` is
   optional, for grouping hunks into threads of the change. **`null` means this
   hunk needs no note**: no card, the hunk header says so, and validation counts
   it as covered. Import churn, rename fallout, blank lines, generated files,
   a token written back by a tool — all `null`. Do not pad with a card that says
   "mechanical change".
4. **What the notes have to do** (this is why the user reaches for this tool):
   - **Say why, do not restate the code.** They can read the diff; what they lack
     is intent, trade-offs and the links to elsewhere. "Adds a ContextWindow
     field" is noise; "context_window and --context-window now share one parser so
     the K/M suffix is not implemented twice" is the point.
   - A card draws on four things, and only where it has something non-obvious to
     say: **why** (evidence from recent history, the brief, the commit message) →
     **the point** (what behaviour changed inside its function) → **blast radius**
     (the callers `context` found: who is affected, did they move too) → **risk and
     verification** (edges, concurrency, error paths; is it tested).
     Not four headings to fill in.
   - Write the cross-file links: if A adds a function B calls, name it in both
     cards (``` `bufferline.lua#3` uses `bound_paths()` from here ```) so the user
     can follow it.
   - One card per non-trivial hunk, `null` for the rest. **Every hunk key must be
     present** (card or null) so the user can tell "not worth explaining" from
     "missed it". A file that is all nulls renders as "all mechanical". A
     mechanical change that does deserve a mention (a token accidentally committed,
     say) goes in `summary` in one line.
   - With a `brief`, follow it: answer its concerns head-on in the relevant cards
     and null out what it says to skip.
   - **Say so** when something is risky or doubtful: "the nil check is gone here —
     worth confirming the caller guarantees non-nil". The user wants a reviewer's
     eye, not a press release.
   - Keep identifiers and terms verbatim, in backticks.
   - **Write for a human skimming the page** — see *Keep it short* below.
4b. **Diagrams** (`diagrams`, optional but worth it): a wall of text explains less
   than one picture. The middle pane has a Diagrams tab and the right pane turns
   into a thumbnail list. The **change map** (file → changed symbols → callers) is
   drawn automatically from `context` data; you do not write it. You draw what a
   machine cannot: the **semantics**.
   - **When**: a call path across ≥3 files → sequence diagram; a branch, state
     machine or retry path changed → flowchart; types, interfaces or struct fields
     changed → class diagram; a migration or refactor "old shape → new shape" →
     two containers `before: {…}` / `after: {…}` in **one** diagram, side by side.
     Usually 1–3 per change. Never draw for the sake of drawing; a purely
     mechanical change gets none.
   - **One board.** Do not reach for `steps` / `layers` / `scenarios`: hudiff
     renders every board as its own static picture and stacks them, so a
     before/after split across boards just makes the reader scroll between two
     nearly identical images. Put both states in one picture instead.
   - **Keep it small.** At most 12 nodes and 3 lines per label; past that, split
     it into two diagrams. A picture that needs zooming has stopped explaining.
   - **d2 only.** `kind` is `"d2"`; hudiff compiles it in-process, so there is
     nothing to install and `hudiff notes` fails loudly on a syntax error.
   - **`hunks` is required**: which hunks the diagram is about (`path#n`). The page
     turns them into chips that jump to the code, and the note cards get a chip
     back to the diagram. A diagram with no anchor is an island.
   - **Colour**: use only the four predefined classes — `added`, `removed`,
     `changed`, `ext` (hudiff defines them for every diagram, yours included).
     Do not set your own fills or strokes; they will not match the page theme.
     Do not set `border-radius` either, hudiff applies it.
   - **Mark the main path**: put `style.animated: true` on the 1–3 edges that carry
     the primary flow or the data direction — that flowing line is the only
     animation hudiff renders. Animating every edge makes the diagram unreadable.
   - **d2 notes**: an id containing `.` must be quoted (`"CLAUDE.md"`), otherwise
     the `.` reads as nesting; a sequence diagram starts with
     `shape: sequence_diagram`; a class diagram is `x.shape: class` plus members;
     before/after is two containers in one diagram, not `steps`; label the edges with *why*
     (`a -> b: fetch the token, fall back on failure`), not just an arrow; keep it
     under about 15 nodes or split it in two.
   - `hudiff diagram <id> --ascii` renders it as text so you can check your own
     diagram in the terminal.
   - Diagrams carry the global relationships, cards carry the local reasoning.
     Do not restate a call chain in prose that the diagram already draws.
5. **Wait for comments**: once the notes are in, tell the user "notes are up,
   comment on the page and hit Send to agent", then run `hudiff wait` **in the
   background** (Bash `run_in_background`, full 600000 timeout). It blocks until
   the user hits Send, then prints the new comments. Default wait is 30 minutes;
   a background command is not bound by the 10-minute foreground limit, so one
   `wait` is enough — do not chain them. Exit code 124 means nobody came within
   30 minutes: stop, tell the user "say the word when you want me to look at the
   comments", do not loop. You can do other work while it waits.

**Headless (CI, no browser, nobody to wait for)**: create the session with
`--no-open --json`, write the notes, and if the request is "can this be merged",
also write a review report (Flow C schema, with `verdict`). Then skip step 5 and
export: `hudiff -s <id> export html -o page.html` writes one self-contained,
read-only file (no daemon, no network; `--font` embeds the mono font, ~400KB
more). Finish with `hudiff daemon stop`.

## Flow B: handle the comments

Once comments arrive (`wait` returns, or the user says "check the comments" and
you run `hudiff comments --new`):

1. Every comment carries `[id]`, the file, the old/new line number, the line it
   is attached to, the body, `author`, and the round trip so far. Handle them
   **one by one, by id** — the user reads them that way.
2. Three cases:
   - A question → answer it: `hudiff reply <id> "<answer>"` (`-` reads stdin).
   - A code change → make the change first, then
     `hudiff reply <id> --resolve "changed: <what you did>"`. That only marks
     "handled by the agent, awaiting you"; **the comment stays open**. Closing a
     comment is the user's call on the page — an agent never closes one
     (`hudiff resolve` only marks it too).
     On a working-tree session run `hudiff session refresh` afterwards so the page
     shows the new diff. Comments re-anchor by line content; ones that cannot be
     found are flagged stale, and the output names the notes that are now void —
     rewrite those hunks.
   - You disagree, or the user has to decide → `reply` with the reasoning and do
     **not** mark it handled. Let them answer on the page.
3. Summarise in one or two lines ("answered N, changed M"), then background
   `hudiff wait` again. The user may push back on your reply; it arrives as a new
   reply on the same comment.

## Flow C: cross-review (`/hudiff review <session>`)

You are the reviewer, not the author: look at it fresh and produce a **review
report**. The page's Review tab renders it, each item is a tickable todo, and the
explaining agent (or the user) ticks it off with a resolution.

1. Read everything:
   - The change: `hudiff -s <id> read`, `context`, `read -W` on the key functions.
   - The author's notes: hunks marked ✓ in `read` have notes in
     `<session dir>/notes.json` (`hudiff -s <id> session info --json` gives the
     directory). List the diagrams with `hudiff -s <id> diagram`, read a source
     with `diagram <id> --src`, or look at one in the terminal with
     `diagram <id> --ascii`. A diagram's structure and edge labels *are* the
     author's claim about the call graph. **Verify, do not copy** — a wrong note
     is itself a finding.
   - The human's input: `hudiff -s <id> comments --all`. The line comments and the
     author's replies often hold the real motive and the consensus already
     reached. Do not re-litigate a settled thread, but do push on a question the
     user raised and the author dodged.
   - **The previous round**: `hudiff -s <id> review show` (not `--open`, you want
     all of it). An existing report means this is round two or later: the user has
     marked some items done or wontfix with a reason. For **every** such item, read
     the code as it stands now, compare it against the resolution or the reason,
     and write a **re-review note** (the note item in the schema below):
     `verdict: ok` = the fix is right / the refusal holds; `disagree` = not fixed,
     fixed incompletely, or the reason does not stand — cite file:line;
     `info` = extra context. The note renders under that item's resolution. Do not
     re-raise the same problem as a new item under a different name; if you
     disagree, say so in the re-review note. A new report = re-review notes + new
     findings. `review set` **merges** by default: old items and their statuses are
     all kept and new items are appended as a new round. **Never** use `--replace`.
   - The `brief` outranks everything else; check the repo's own conventions too.
2. Write the report JSON and run
   `hudiff -s <id> review set <file> --author <who you are>` (merges, does not
   overwrite). Validation checks each `file`/`line` against the diff (line numbers
   come from `read`'s **new** column; for a deleted line pass `"side":"old"`) and
   fills in the hunk and snippet. Writing it wakes an author agent sitting in
   `wait`. Schema:

   ```json
   {
     "summary": "1–2 sentences: the overall judgement, not a list of the items",
     "verdict": {"decision": "fix-then-merge", "reason": "one short line naming the blocking items: r1, r2"},
     "strengths": ["one short line each, at most 3, something concretely good"],
     "items": [
       {"priority": "high", "title": "the problem in one line", "file": "a/b.go", "line": 42,
        "body": "1–2 sentences: what breaks and when",
        "suggestion": "the concrete fix:\n```diff\n-old\n+new\n```"},
       {"id": "r1", "note": "re-review: workflows.go:146 now reads as history, consistent with the text above", "verdict": "ok"},
       {"id": "r3", "note": "the refusal says 'deliberate', but settings.json switched back to opus only last commit — worth confirming", "verdict": "disagree"}
     ]
   }
   ```
   An item with only `id` + `note` is a **re-review note**: it is appended under
   the item with that id from the previous round and is not a new finding. You can
   also add one later: `hudiff -s <id> review note r1 "…" --verdict ok`.
   `verdict.decision` is `merge`, `fix-then-merge` or `do-not-merge`; it shows as
   a banner atop the page and the report (optional; the latest round's wins).
   `priority`: `high` (or P0) = must be fixed before shipping (a real bug, a clear
   violation); `medium` (P1) = should be fixed but not blocking; `low` (P2) = style and
   readability. An overall problem with no location can omit `file`.
3. Every item must be **independently actionable**: a location, the offending
   code, a fix. "Consider refactoring" is not an item. Strengths should name the
   specific thing that is good. Fewer and better: 3–10 items.
4. Then stop. Do **not** change the code yourself and do **not** `wait`. A second
   round is the user's call.

The author agent picks it up (`wait` says the review report was updated, or the
user says "work through the review"): `hudiff review show --open`, fix the code,
`hudiff review done <id> "what you did"`; `hudiff review wontfix <id> "why not"`
where you disagree — **leave genuine disagreements to the user, do not force a
done**. Run `session refresh` on a working-tree session afterwards. When it is
all handled, `hudiff review md` exports markdown to paste into the PR.

## Keep it short

The reader skims the page next to the code; every extra sentence costs them.
Notes, review and verdict all land on one page, so the budgets below are for
the whole page, not each part.

- **Say each thing once.** The notes explain, the review judges. When there is
  also a review report (cross-review, or headless with a verdict), a risk that
  is a review item stays out of the cards and the summary — at most a pointer
  ("see review r1"). The verdict reason names item ids; the review summary does
  not re-list the items; strengths do not repeat what a card already said.
- **Budgets.** Summary 1–3 sentences. A card: one lead sentence plus at most
  3–4 bullets, each one line — a card longer than its hunk has failed. Review
  item: a title that states the claim, a body of 1–2 sentences, a suggestion
  that is a minimal diff or one line.
- **Lead with the point**, then the evidence. Cut hedges, background the reader
  can see in the diff, and asides ("the comment says X, but…" → just say what
  is true).
- **Layout over prose**: short bullets, `L42` line refs, identifiers in
  backticks; no bold headings like **Why** / **Risks** inside a card, no nested
  bullets, no paragraph over 3 lines.
- Match the user's language; keep code terms verbatim.

## What "Send to agent" does

The session is the inbox. Whoever runs `hudiff wait` on it gets woken; there is
no notion of sending to a particular process. If nobody is waiting, the comments
are marked pending and the page shows the user the exact command to run —
so you may be handed `hudiff -s <id> wait` to run. If the user configured
`hooks.on_submit`, that shell command runs instead of any built-in delivery.

## Traps

- **Do not create a session twice for the same diff.** Creating is idempotent by
  source, but `--new` forces a duplicate and so does changing the target's shape.
  One diff, one session; everything after that takes `-s <id>`. To look again use
  `read` / `context -s <id>`; for the url, `hudiff session url -s <id>`; to list,
  `hudiff session list`. `hudiff open <id>` reopens an existing one in the browser.
- **Pass `-s <id>` on every command** (the `session` field of the `--json`
  output). Without it hudiff takes "the most recent session in this repo", which
  collides when another agent is working in the same repo. `-s` goes anywhere in
  the command line.
- Sessions live in `~/.local/state/hudiff/sessions/<id>/`, not in /tmp, and are
  never cleaned automatically. `hudiff session list` shows them,
  `hudiff session url -s <id>` prints the address, `--home` prints the index.
  A daemon restart changes nothing: the port is stable, the page reconnects, and
  `wait` waits for it to come back.
- **`notes` replaces the whole file, it does not merge.** A second call wipes the
  first, including files this JSON never mentions. The "still unexplained" list it
  prints is about *this* document, not an invitation to send another. Reading a
  big diff in batches is fine; the JSON has to be assembled locally and written
  once (`title` / `summary` / `diagrams` included — omitting them deletes them).
- When `notes` validation fails nothing is written. Fix it and rerun; do not
  reach for `--force`.
- Never edit `comments.json` yourself. The daemon is the only writer; `reply` and
  `resolve` go through it.
- `session refresh` works on working-tree / commit / range / PR sessions. A patch
  session has no source to re-read.
- `hudiff daemon stop` stops the daemon (nothing is deleted); the next command
  starts it again.
- `hudiff diagram` lists the diagrams; `hudiff diagram auto-map --src` prints the
  generated change map's d2 source (a useful starting point for a better diagram);
  `hudiff diagram <id> -o x.svg` exports one; `--ascii` prints it as text.
