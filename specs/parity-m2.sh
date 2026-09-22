#!/bin/sh
# M2 half of the parity gate: session creation. For each source kind the Go
# build and the Python oracle each build a session in a throwaway state dir;
# diff.json must match byte for byte and session.json must match apart from
# id / created / opener (opener changed shape in spec §8.1).
#
# Sourced by parity.sh, which provides ok(), bad() and $ROOT.
set -eu

PY_ORACLE="python3 $ROOT/legacy/hudiff.py"
GO_BIN="$ROOT/bin/hudiff"

# Both implementations start a daemon just to put a URL in --json output, so
# every throwaway state dir leaves one behind. Stop it before dropping the dir.
STATES=$(mktemp)

remember() { printf '%s\n' "$@" >>"$STATES"; }

stop_state() {
    XDG_STATE_HOME=$1 $PY_ORACLE stop >/dev/null 2>&1 || true
    XDG_STATE_HOME=$1 $GO_BIN daemon stop >/dev/null 2>&1 || true
}

sweep_states() {
    [ -f "$STATES" ] || return 0
    while read -r d; do
        [ -n "$d" ] && stop_state "$d"
        [ "${FAILED:-0}" = 0 ] && rm -rf "$d"
    done <"$STATES"
    rm -f "$STATES"
}

# Allowed to differ, with reasons:
#   id / created   new session each run
#   opener         spec §8.1 dropped tmux_pane + ppid, renamed claude_session
#   diagram_engine spec §7 (2026-09-22) defaults to mermaid; the oracle predates
#                  the field and writes nothing
strip_session() {
    python3 -c 'import json,sys
d = json.load(open(sys.argv[1]))
for k in ("id", "created", "opener"):
    d.pop(k, None)
d.get("meta", {}).pop("diagram_engine", None)
print(json.dumps(d, sort_keys=True, indent=1, ensure_ascii=False))' "$1"
}

as_json() {
    python3 -c 'import json,sys; print(json.dumps(json.load(open(sys.argv[1])), sort_keys=True, indent=1, ensure_ascii=False))' "$1"
}

# create <label> <repo> <py args…> -- <go args…>
create() {
    label=$1 repo=$2
    shift 2
    pyargs="" goargs="" side=py
    for a in "$@"; do
        if [ "$a" = "--" ]; then side=go; continue; fi
        if [ "$side" = py ]; then pyargs="$pyargs $a"; else goargs="$goargs $a"; fi
    done
    p=$(mktemp -d) g=$(mktemp -d)
    remember "$p" "$g"
    ( cd "$repo" && XDG_STATE_HOME=$p $PY_ORACLE open --no-browser --json $pyargs ) >"$p/out" 2>&1 || true
    ( cd "$repo" && XDG_STATE_HOME=$g $GO_BIN $goargs --no-open --json ) >"$g/out" 2>&1 || true
    ps=$(ls -d "$p"/hudiff/sessions/* 2>/dev/null | head -1) || true
    gs=$(ls -d "$g"/hudiff/sessions/* 2>/dev/null | head -1) || true
    if [ -z "${ps:-}" ] || [ -z "${gs:-}" ]; then
        bad "$label  (no session: py=${ps:-none} go=${gs:-none}; see $p/out $g/out)"
        return 0
    fi
    if cmp -s "$ps/diff.json" "$gs/diff.json"; then
        ok "$label diff.json ($(wc -c <"$ps/diff.json" | tr -d ' ') bytes)"
    else
        bad "$label diff.json  ($ps/diff.json vs $gs/diff.json)"
        return 0
    fi
    strip_session "$ps/session.json" >"$p/session.norm"
    strip_session "$gs/session.json" >"$g/session.norm"
    if diff -u "$p/session.norm" "$g/session.norm" >"$g/sdiff"; then
        ok "$label session.json"
    else
        bad "$label session.json  ($g/sdiff)"
    fi
    PY_SESSION=$ps GO_SESSION=$gs PY_STATE=$p GO_STATE=$g
}

# A throwaway repo with one uncommitted edit, so the working-tree cases do not
# depend on $ROOT happening to be dirty. It is also the refresh fixture below.
FIX=$(mktemp -d)
git init -q "$FIX/repo"
cd "$FIX/repo"
git config user.email p@p && git config user.name p
printf 'package main\n\nfunc alpha() int {\n\treturn 1\n}\n\nfunc beta() int {\n\treturn 2\n}\n' >a.go
git add -A && git commit -qm base
printf 'package main\n\nfunc alpha() int {\n\treturn 11\n}\n\nfunc beta() int {\n\treturn 22\n}\n' >a.go
cd "$ROOT"

create "create-workspace" "$FIX/repo" -- 
FP=$PY_STATE FG=$GO_STATE FPS=$PY_SESSION FGS=$GO_SESSION

# -p / -c land in session.brief. The context file exists on purpose: the
# "file missing" marker is one of the strings that turned English.
create "create-with-brief" "$FIX/repo" -p check-errors -c a.go -- -p check-errors -c a.go

create "create-commit"    "$ROOT" HEAD~1 -- commit HEAD~1
create "create-range"     "$ROOT" HEAD~2..HEAD -- range HEAD~2..HEAD

# A patch session re-parses an existing diff.patch, so every fixture session is
# also a parser check: the result must equal the diff.json already on disk.
for fx in "$HOME"/.local/state/hudiff/sessions/*/diff.patch; do
    [ -f "$fx" ] || continue
    name=$(basename "$(dirname "$fx")")
    g=$(mktemp -d)
    remember "$g"
    XDG_STATE_HOME=$g $GO_BIN patch "$fx" --no-open --json >"$g/out" 2>&1 || true
    gs=$(ls -d "$g"/hudiff/sessions/* 2>/dev/null | head -1) || true
    if [ -n "${gs:-}" ] && cmp -s "$gs/diff.json" "$(dirname "$fx")/diff.json"; then
        ok "reparse $name"
    else
        bad "reparse $name  ($g)"
    fi
done

# Refresh: same session on both sides, same comment, same edit — the new diff
# and the re-anchored comments must agree.
python3 - "$FPS" "$FGS" <<'PY'
import json, sys
# One line comment anchored by content, so refresh has to find it again.
c = [{"id": "c1", "target": "line", "file": "a.go", "hunk": 0, "side": "new", "line": 4,
      "snippet": "\treturn 11", "body": "why 11?", "status": "open", "stale": False,
      "created": "2026-01-01T00:00:00+08:00", "replies": []}]
for d in sys.argv[1:3]:
    json.dump(c, open(d + "/comments.json", "w"), ensure_ascii=False, indent=1)
PY

cd "$FIX/repo"
printf 'package main\n\n// a new line pushes every hunk down\n\nfunc alpha() int {\n\treturn 11\n}\n\nfunc beta() int {\n\treturn 222\n}\n' >a.go
XDG_STATE_HOME=$FP $PY_ORACLE refresh >"$FP/refresh.out" 2>&1 || true
XDG_STATE_HOME=$FG $GO_BIN session refresh >"$FG/refresh.out" 2>&1 || true
cd "$ROOT"

if cmp -s "$FPS/diff.json" "$FGS/diff.json"; then
    ok "refresh diff.json"
else
    bad "refresh diff.json  ($FPS/diff.json vs $FGS/diff.json)"
fi
as_json "$FPS/comments.json" >"$FP/comments.norm"
as_json "$FGS/comments.json" >"$FG/comments.norm"
if diff -u "$FP/comments.norm" "$FG/comments.norm" >"$FG/cdiff"; then
    ok "refresh reanchor  ($(python3 -c 'import json,sys
c = json.load(open(sys.argv[1]))[0]
print("c1 -> hunk", c["hunk"], "line", c["line"], "stale", c["stale"])' "$FGS/comments.json"))"
else
    bad "refresh reanchor  ($FG/cdiff)"
fi
rm -rf "$FIX"

if [ -n "${PARITY_PR:-}" ]; then
    create "create-pr-$PARITY_PR" "${PARITY_PR_REPO:-$ROOT}" pr "$PARITY_PR" -- pr "$PARITY_PR"
fi

sweep_states
