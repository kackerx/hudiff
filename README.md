```
██╗  ██╗██╗   ██╗██████╗ ██╗███████╗███████╗
██║  ██║██║   ██║██╔══██╗██║██╔════╝██╔════╝
███████║██║   ██║██║  ██║██║█████╗  █████╗
██╔══██║██║   ██║██║  ██║██║██╔══╝  ██╔══╝
██║  ██║╚██████╔╝██████╔╝██║██║     ██║
╚═╝  ╚═╝ ╚═════╝ ╚═════╝ ╚═╝╚═╝     ╚═╝
```

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/logo-dark.svg">
    <img src="assets/logo.svg" alt="hudiff — diffs for humans" width="300">
  </picture>
</p>

<p align="center"><b>hudiff</b> — diffs for humans.<br>
Let your coding agent explain a diff; review it line by line in your browser; send your comments straight back to the agent.</p>

<p align="center">
  <a href="#install">Install</a> ·
  <a href="#quickstart">Quickstart</a> ·
  <a href="#agent-skill">Agent skill</a> ·
  <a href="#configuration">Configuration</a> ·
  <a href="#how-it-works">How it works</a>
</p>

---

> **Status:** early. The current CLI is a single-file Python script that works today; a Go rewrite (single static binary) is in progress — see [`specs/spec.md`](specs/spec.md).

## Why

Your agent (Claude Code, pi, opencode, codex, …) just changed forty files. Or a teammate opened a PR.
`git diff` tells you *what* changed. It does not tell you *why*, *what it touches*, or *where the risk is*.

hudiff opens a local page that looks like a GitHub PR review, plus three things the agent fills in:

| Tab | What you get | Who writes it |
|---|---|---|
| **Code** | The diff, with a review-style note per hunk — why this change, blast radius, risks. Trivial hunks stay silent. | the explaining agent |
| **Diagrams** | An auto-generated change map (files → changed symbols → outside callers) and any flow/sequence diagrams the agent adds (d2 or mermaid) | auto + agent |
| **Review** | A cross-review from a *second* agent: verdict, strengths, high/medium/low findings. Re-reviews merge into the same report and never overwrite your checkmarks. | the reviewing agent |

You comment on a line, a hunk, or the whole change, hit **Send**, and the agent waiting on `hudiff wait` wakes up, replies, or edits the code.
After it edits, `hudiff session refresh` re-anchors your comments to the new diff.

## Features

- **Works with any agent that can run a shell.** The CLI is the product; the bundled `SKILL.md` teaches the agent the workflow.
- **Zero runtime deps.** One binary. `git` required; `gh` for PRs; `d2` only if you pick the d2 diagram engine.
- **Sessions, not tabs.** One local daemon serves every session at `/s/<id>/`. Sessions live on disk, reopen any time, archive or delete when done.
- **Idempotent.** Running hudiff twice on the same target reuses the session and refreshes it in place.
- **Fast on big diffs.** 5k+ line diffs scroll smoothly (`content-visibility`), incremental highlight, no framework.
- **You stay in charge.** The agent cannot close your comments — it can only mark them *resolved, awaiting your confirmation*.
- **Keyboard-driven.** `j`/`k` hunks, `c` comment, `s` send, `1`/`2`/`3` tabs, `?` for the full map. All rebindable.

## Install

> Until the Go binary ships, install the Python version (Python 3.9+, standard library only):

```bash
git clone https://github.com/kackerx/hudiff ~/proj/hudiff
ln -s ~/proj/hudiff/hudiff ~/.local/bin/hudiff     # any directory on your PATH
brew install d2                                     # optional: d2 diagram engine
```

Planned for the Go release:

```bash
brew install kackerx/tap/hudiff
# or
go install github.com/kackerx/hudiff@latest
```

## Quickstart

```bash
cd your-repo
hudiff                       # working tree vs HEAD  → opens the page
hudiff --staged              # only staged changes
hudiff commit HEAD~1         # one commit
hudiff range main..HEAD      # a range
hudiff pr 562                # a GitHub PR (number, #562, or URL)
hudiff patch fix.patch       # any patch file ('-' for stdin)
hudiff -p "focus on error handling; skip style changes"   # a brief for the agent
```

Then, in your agent: **`/hudiff`** (or just *"explain this diff"*). It reads the diff, writes the notes, and waits for your comments.
Open a second agent and say **`/hudiff review <session-id>`** for a cross-review.

Reopen or manage sessions later:

```bash
hudiff open                  # reopen the latest session
hudiff session list          # all sessions
hudiff session archive <id>  # hide from the home page, keep the data
```

## Agent skill

The skill is plain Markdown — the same format Claude Code, pi, opencode and codex read. Link it once:

```bash
ln -s ~/proj/hudiff/skills/hudiff ~/.claude/skills/hudiff       # Claude Code
ln -s ~/proj/hudiff/skills/hudiff ~/.pi/agent/skills/hudiff     # pi
# opencode / codex: symlink into their skills directory
```

(`hudiff skill install --for <agent>` will do this for you once the Go binary lands.)

What the agent runs, roughly:

```
hudiff read                 diff with [file#hunk] markers and both line-number columns
hudiff context              repo context: file history, changed symbols, callers, related tests
hudiff notes <json>         write the explanation (validated; null = "nothing to say")
hudiff wait                 block until you hit Send, then print your comments
hudiff reply / resolve      answer a comment / mark it resolved (pending your confirmation)
hudiff review set <json>    cross-review report (merged into earlier rounds)
```

## Configuration

`$XDG_CONFIG_HOME/hudiff/config.json` (usually `~/.config/hudiff/config.json`). Everything is optional:

```jsonc
{
  "viewer": "web",                          // "tui" reserved for a future terminal UI
  "web": { "port": 0, "open": true, "browser": "" },
  "diagram": { "engine": "d2" },            // "d2" | "mermaid"
  "data_dir": "",                           // default: $XDG_STATE_HOME/hudiff
  "prompts_dir": "",                        // default: <config dir>/prompts/{default,<repo>}.md
  "hooks": { "on_submit": "", "on_review_request": "" },   // shell commands; $HUDIFF_SESSION, $HUDIFF_URL
  "keymap": { "send": "s", "comment": "c" },
  "ui": { "theme": "auto", "language": "en" }
}
```

Environment: `HUDIFF_PORT`, `HUDIFF_PROMPTS`, `XDG_STATE_HOME`, `HUDIFF_DEV=1` (serve `web/` from disk while hacking on the UI).

## How it works

- **One daemon, many sessions.** Any subcommand starts it if needed. Session data is plain JSON under
  `~/.local/state/hudiff/sessions/<id>/` — readable by you and by the agent.
- **The daemon is the only writer of comments.** The CLI talks to it over HTTP on `127.0.0.1`.
- **A session is an inbox.** *Send* bumps a counter; whoever is blocked in `hudiff wait` on that session wakes up.
  No process registry, no terminal-multiplexer tricks. If nobody is waiting, the page tells you what to run —
  or fires your `on_submit` hook.
- **Reviews merge.** A second round keeps every earlier item and its done/won't-fix state, updates wording in place, and appends re-review notes.
- **Diagrams:** d2 renders server-side and is cached by source hash; mermaid renders in the browser.

## Development

```bash
HUDIFF_DEV=1 hudiff daemon start     # hot-reload web/ from disk
specs/parity.sh <session-id>         # Go vs Python oracle: read / session info / api/session must match
```

Design notes and the migration plan live in [`specs/`](specs/).

## License

[MIT](LICENSE) © 2026 kackerx
