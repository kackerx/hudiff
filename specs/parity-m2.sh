#!/bin/sh
# M2 section: session creation. Each source kind builds a session in its own
# throwaway state dir out of the fixture repo, and its diff.json / session.json
# are compared against the frozen goldens.
#
# Sourced by parity.sh, which provides ok/bad/gold/new_state/hud/sess_of and
# $FIX (the fixture repo), $STATE / $SESS / $SID (the session everything else
# works on).
set -eu

# The session parity.sh built, before anything below touches the repo.
gold workspace-diff.json    json "$SESS/diff.json"
gold workspace-session.json json "$SESS/session.json"

# kind <label> <repo> <args…> — build a session and freeze both files.
kind() {
    label=$1 repo=$2
    shift 2
    st=$(new_state)
    ( cd "$repo" && XDG_STATE_HOME=$st "$GO" "$@" --no-open --json ) >"$st/out" 2>&1 || true
    KS=$(sess_of "$st")
    KSTATE=$st
    if [ -z "${KS:-}" ]; then
        bad "$label-diff.json  (no session; see $st/out)"
        bad "$label-session.json"
        return 0
    fi
    gold "$label-diff.json"    json "$KS/diff.json"
    gold "$label-session.json" json "$KS/session.json"
}

kind commit "$FIX/repo" commit HEAD
kind range  "$FIX/repo" range HEAD~1..HEAD

# -p / -c land in session.brief, and `read` grows a review-request header that
# nothing else covers.
kind brief "$FIX/repo" -p check-errors -c a.go
if [ -n "${KS:-}" ]; then
    XDG_STATE_HOME=$KSTATE "$GO" -s "$(basename "$KS")" read >"$WORK/read-brief" 2>&1 || true
    gold read-brief mask "$WORK/read-brief"
else
    bad "read-brief  (no brief session)"
fi

# source_key idempotence: the same source in the same state dir reopens the same
# session, --new insists on a fresh one.
ik=$(new_state)
( cd "$FIX/repo" && XDG_STATE_HOME=$ik "$GO" --no-open --json ) >"$ik/a" 2>&1 || true
( cd "$FIX/repo" && XDG_STATE_HOME=$ik "$GO" --no-open --json ) >"$ik/b" 2>&1 || true
( cd "$FIX/repo" && XDG_STATE_HOME=$ik "$GO" --no-open --new --json ) >"$ik/c" 2>&1 || true
n=$(ls -d "$ik"/hudiff/sessions/* 2>/dev/null | wc -l | tr -d ' ')
ida=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["session"])' "$ik/a" 2>/dev/null || echo a)
idb=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["session"])' "$ik/b" 2>/dev/null || echo b)
if [ "$ida" = "$idb" ]; then
    ok "the same source reopens the same session (source_key)"
else
    bad "source_key: $ida != $idb"
fi
if [ "$n" = 2 ]; then
    ok "--new builds a second session for the same source"
else
    bad "--new: $n session(s) in the state dir, want 2"
fi

# Every session on disk is also a parser fixture: re-parsing its diff.patch has
# to reproduce the diff.json that is stored next to it. These are the user's own
# sessions — read, never written, and never frozen into a golden.
for fx in "$HOME"/.local/state/hudiff/sessions/*/diff.patch; do
    [ -f "$fx" ] || continue
    name=$(basename "$(dirname "$fx")")
    g=$(new_state)
    XDG_STATE_HOME=$g "$GO" patch "$fx" --no-open --json >"$g/out" 2>&1 || true
    gs=$(sess_of "$g")
    if [ -n "${gs:-}" ] && cmp -s "$gs/diff.json" "$(dirname "$fx")/diff.json"; then
        ok "reparse $name"
    else
        bad "reparse $name  ($g)"
    fi
done

# Refresh: a comment anchored by content, then an edit that pushes every hunk
# down. The new diff and the re-anchored comment both have to land where the
# goldens say.
python3 - "$SESS" <<'PY'
import json, sys
json.dump([{"id": "c1", "target": "line", "file": "a.go", "hunk": 0, "side": "new", "line": 4,
            "snippet": "\treturn 11", "body": "why 11?", "status": "open", "stale": False,
            "created": "2026-01-01T00:00:00+08:00", "replies": []}],
          open(sys.argv[1] + "/comments.json", "w"), ensure_ascii=False, indent=1)
PY
cd "$FIX/repo"
{ printf '// a new line pushes every hunk down\n'; sed 's/return 22$/return 222/' a.go; } >a.tmp && mv a.tmp a.go
git add -A
cd "$ROOT"
hud "$STATE" session refresh >"$WORK/refresh.out" 2>&1 || true
gold refresh-diff.json     json "$SESS/diff.json"
gold refresh-comments.json json "$SESS/comments.json"

# Untracked files are ours (spec §11.8); the fixture stays free of them until
# here so the byte-for-byte checks above are unaffected.
printf 'brand new\nsecond line\n' >"$FIX/repo/untracked.txt"
printf 'ignored/\n' >"$FIX/repo/.gitignore"
mkdir -p "$FIX/repo/ignored" && printf 'hidden\n' >"$FIX/repo/ignored/x.txt"
u=$(new_state)
( cd "$FIX/repo" && XDG_STATE_HOME=$u "$GO" --no-open --new --json ) >"$u/out" 2>&1 || true
us=$(sess_of "$u")
if [ -n "${us:-}" ] && python3 -c 'import json,sys
files = {f["path"]: f["status"] for f in json.load(open(sys.argv[1]))["files"]}
assert files.get("untracked.txt") == "A", files
assert ".gitignore" in files, files
assert "ignored/x.txt" not in files, files' "$us/diff.json" 2>"$u/err"; then
    ok "untracked file shows as added, .gitignore honoured"
else
    bad "untracked  ($u/out $u/err)"
fi

# A PR is live data — someone can push to it between two runs — so this is an
# invariant check rather than a golden, and the diff never enters the repo.
if [ -n "${PARITY_PR:-}" ]; then
    pr=$(new_state)
    ( cd "${PARITY_PR_REPO:-$ROOT}" && XDG_STATE_HOME=$pr "$GO" pr "$PARITY_PR" --no-open --json ) >"$pr/out" 2>&1 || true
    prs=$(sess_of "$pr")
    if [ -n "${prs:-}" ] && python3 -c 'import json,sys
s = json.load(open(sys.argv[1] + "/session.json"))
assert s["kind"] == "pr", s["kind"]
assert s["meta"].get("url", "").endswith("/pull/" + sys.argv[2]), s["meta"]
assert s["meta"].get("head") and s["meta"].get("base"), s["meta"]
assert s["title"].startswith("#" + sys.argv[2]), s["title"]
assert s["refresh"] == ["pr", sys.argv[2]], s["refresh"]' "$prs" "$PARITY_PR" 2>"$pr/err1"; then
        ok "pr $PARITY_PR builds a session with url / head / base and a refresh recipe"
    else
        bad "pr $PARITY_PR session  ($pr/out $pr/err1)"
    fi
    if [ -n "${prs:-}" ] && python3 -c 'import json,sys
d = json.load(open(sys.argv[1] + "/diff.json"))
assert len(d["files"]) > 1, len(d["files"])
assert all(f["status"] in "AMDRT" for f in d["files"]), d["files"][:3]
assert sum(len(f["hunks"]) for f in d["files"]) > 1' "$prs" 2>"$pr/err2"; then
        ok "pr $PARITY_PR diff parses ($(python3 -c 'import json,sys; print(len(json.load(open(sys.argv[1]+"/diff.json"))["files"]))' "$prs") files)"
    else
        bad "pr $PARITY_PR diff  ($pr/err2)"
    fi
fi
