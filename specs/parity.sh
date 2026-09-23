#!/bin/sh
# The gate. Every surface the Go build exposes — `read`, `session list|info`,
# the HTTP API, the write path, review, diagrams and the rendered page — is
# captured from a fixture this script builds from scratch and diffed against a
# frozen golden file under specs/golden/. Anything that drifts fails: wording,
# field names, element order, counts.
#
#   specs/parity.sh [--no-page] [--bless] [--self-check]
#
#     --bless       rewrite the golden files from this run instead of comparing.
#                   Read the diff before committing it — a blessed regression is
#                   a regression with a green light.
#     --self-check  break the Go build on purpose and assert the matching checks
#                   go red, so a golden that stopped meaning anything is caught.
#     --no-page     skip the headless-Chrome part.
#
# Until M6 this compared everything against legacy/hudiff.py. The oracle is gone;
# the goldens were frozen from the last run where the two still agreed. Nothing
# here may read legacy/ again — `gate is oracle-free` asserts that.
#
# The user's own sessions are never used as a fixture (they hold private code and
# would not reproduce on another machine); the gate runs entirely inside mktemp
# state dirs and never touches the daemon on the user's port.
set -eu

cd "$(dirname "$0")/.."
ROOT=$(pwd)
GO="$ROOT/bin/hudiff"
GOLD="$ROOT/specs/golden"
PAGE=1
BLESS=0
SELF=0
CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

while [ $# -gt 0 ]; do
    case $1 in
        --no-page) PAGE=0; shift ;;
        --bless) BLESS=1; shift ;;
        --self-check) SELF=1; shift ;;
        *) echo "usage: $0 [--no-page] [--bless] [--self-check]" >&2; exit 1 ;;
    esac
done

# Sessions record who opened them (session.json opener.agent_session, from
# CLAUDE_SESSION_ID / PI_COMPANION_SESSION_ID). Left alone, the goldens would
# carry the UUID of whoever ran --bless and fail for everyone else.
CLAUDE_SESSION_ID=parity
PI_COMPANION_SESSION_ID=parity
export CLAUDE_SESSION_ID PI_COMPANION_SESSION_ID
# Same for the fixture's commit dates: `read`, context and commit sessions
# print them, and they would follow the calendar.
GIT_AUTHOR_DATE='2026-09-22T12:00:00+08:00'
GIT_COMMITTER_DATE=$GIT_AUTHOR_DATE
export GIT_AUTHOR_DATE GIT_COMMITTER_DATE

WORK=$(mktemp -d)
FIX=$(mktemp -d)
STATES=$(mktemp)
FAILED=0

ok()  { printf '  \033[32mok\033[0m    %s\n' "$1"; }
bad() { printf '  \033[31mFAIL\033[0m  %s\n' "$1"; FAILED=$((FAILED + 1)); }

# Every session the gate builds gets its own XDG_STATE_HOME, and every one of
# them leaves a daemon behind (`--json` needs a URL). Stop them before the dirs go.
remember() { printf '%s\n' "$@" >>"$STATES"; }
new_state() { d=$(mktemp -d); remember "$d"; printf '%s' "$d"; }

sweep_states() {
    [ -f "$STATES" ] || return 0
    while read -r d; do
        [ -n "$d" ] && XDG_STATE_HOME=$d "$GO" daemon stop >/dev/null 2>&1
        [ "$FAILED" = 0 ] && rm -rf "$d"
    done <"$STATES"
    rm -f "$STATES"
}
cleanup() { sweep_states; [ "$FAILED" = 0 ] && rm -rf "$FIX"; return 0; }
trap cleanup EXIT

hud()     { st=$1; shift; XDG_STATE_HOME=$st "$GO" "$@"; }
port_of() { python3 -c 'import json,sys; print(json.load(open(sys.argv[1] + "/hudiff/server.json"))["port"])' "$1"; }
sess_of() { ls -d "$1"/hudiff/sessions/* 2>/dev/null | head -1; }

same() {   # same <label> <expected> <actual>
    if diff -u "$2" "$3" >"$WORK/$1.diff" 2>&1; then
        ok "$1"
    else
        bad "$1  ($WORK/$1.diff, $(wc -l <"$WORK/$1.diff" | tr -d ' ') lines)"
    fi
}

# golden <label> <file> — compare against specs/golden/<label>, or write it.
golden() {
    g="$GOLD/$1"
    if [ "$BLESS" = 1 ]; then
        mkdir -p "$(dirname "$g")"
        cp "$2" "$g"
        ok "blessed $1 ($(wc -l <"$g" | tr -d ' ') lines)"
        return 0
    fi
    if [ ! -s "$g" ]; then
        bad "$1  (no golden at $g — run --bless)"
        return 0
    fi
    if [ ! -s "$2" ]; then
        bad "$1  (captured nothing)"
        return 0
    fi
    same "$1" "$g" "$2"
}

# gold <label> <mask|json|structure> <file> — normalise, then compare.
gold() {
    if ! python3 "$ROOT/specs/parity-normalize.py" "$2" "$3" >"$WORK/$1.n" 2>"$WORK/$1.err"; then
        bad "$1  (normalise failed: $(head -c 200 "$WORK/$1.err"))"
        return 0
    fi
    golden "$1" "$WORK/$1.n"
}

# ── --self-check: a golden nobody can break is not a check ───────────────────
# Each mutation is a plausible regression on a different surface; the labels
# after the pipe are the checks that must go red because of it.
MUTATIONS='internal/cli/read.go|# Mark after the number:|# Mark:|read read-W read-U3 read-file read-brief
internal/cli/session.go|"hunks_total"|"hunks_count"|info
web/session.html|<section id="mid">|<section data-probe="1" id="mid">|page-code page-diagrams page-review
internal/diagram/automap.go|"M": "modified"|"M": "edited"|auto-map-src diagram-ascii'

if [ "$SELF" = 1 ]; then
    echo "self-check: breaking the Go build on purpose…"
    restore_src() {
        [ -d "$WORK/orig" ] || return 0
        (cd "$WORK/orig" && find . -type f) | while read -r f; do
            cp "$WORK/orig/$f" "$ROOT/$f"
        done
        rm -rf "$WORK/orig"
        go build -o "$GO" . >/dev/null 2>&1 || true
    }
    trap 'restore_src; cleanup' EXIT
    printf '%s\n' "$MUTATIONS" | while IFS='|' read -r f from to labels; do
        mkdir -p "$WORK/orig/$(dirname "$f")"
        cp "$ROOT/$f" "$WORK/orig/$f"
        python3 - "$ROOT/$f" "$from" "$to" <<'PY'
import sys
p, a, b = sys.argv[1:4]
s = open(p).read()
assert a in s, f"self-check: {a!r} not found in {p}"
open(p, "w").write(s.replace(a, b, 1))
PY
    done
    go build -o "$GO" . || { echo "self-check: the mutated tree does not build" >&2; exit 1; }
    sh "$ROOT/specs/parity.sh" >"$WORK/self.out" 2>&1 || true
    restore_src
    trap cleanup EXIT
    echo "self-check: which checks noticed"
    printf '%s\n' "$MUTATIONS" | while IFS='|' read -r f from to labels; do
        for lb in $labels; do
            if grep -q "FAIL.*  $lb  " "$WORK/self.out" || grep -q "FAIL.*  $lb\$" "$WORK/self.out"; then
                printf '  \033[32mok\033[0m    %s catches %s\n' "$lb" "$f"
            else
                printf '  \033[31mFAIL\033[0m  %s stayed green after %s was broken\n' "$lb" "$f"
                echo fail >>"$WORK/self.failed"
            fi
        done
    done
    [ -s "$WORK/self.failed" ] && { echo "self-check failed; full run in $WORK/self.out" >&2; exit 1; }
    echo "self-check ok (full run: $WORK/self.out)"
    exit 0
fi

# ── build ────────────────────────────────────────────────────────────────────
echo "building Go binary…"
mkdir -p "$ROOT/bin"
go build -o "$GO" .

# ── the fixture repo ─────────────────────────────────────────────────────────
# One commit of history plus a staged change set that covers every file status
# `read` prints differently: modified, added, deleted (so -W has to say it cannot
# read the new file) and renamed.
git init -q "$FIX/repo"
cd "$FIX/repo"
git config user.email p@p && git config user.name p
cat >a.go <<'EOF'
package main

func alpha() int {
	return 1
}

// the spacer keeps alpha and beta in separate hunks, so `read` has a hunk that
// is explicitly marked "needs no note" next to one that carries a note
// 3
// 4
// 5
// 6
// 7

func beta() int {
	return 2
}
EOF
printf 'package main\n\nfunc gone() string {\n\treturn "removed"\n}\n' >gone.go
printf 'package main\n\nfunc moved() int {\n\treturn 7\n}\n' >old_name.go
printf '# fixture\n' >README.md
# untouched by the change set, so the auto map and `context` have somewhere to
# point: a caller outside the diff, and a red edge for the symbol that is gone.
cat >user.go <<'EOF'
package main

func useAll() int {
	return alpha() + beta() + moved() + len(gone())
}
EOF
git add -A && git commit -qm base
printf '# fixture\n\nsecond commit, so commit and range sessions have something to show.\n' >README.md
git commit -qam second
sed -e 's/return 1$/return 11/' -e 's/return 2$/return 22/' \
    -e 's/^func beta() int {$/func beta(n int) int {/' a.go >a.tmp && mv a.tmp a.go
git rm -q gone.go
git mv old_name.go new_name.go
printf 'package main\n\nfunc moved() int {\n\treturn 8\n}\n' >new_name.go
printf 'package main\n\nfunc fresh() bool {\n\treturn true\n}\n' >added.go
git add -A
cd "$ROOT"

# ── the session every later section works on ─────────────────────────────────
STATE=$(new_state)
( cd "$FIX/repo" && XDG_STATE_HOME=$STATE "$GO" --no-open --json ) >"$WORK/create.out" 2>&1 || true
SESS=$(sess_of "$STATE")
if [ -z "${SESS:-}" ]; then
    echo "could not create the fixture session; see $WORK/create.out" >&2
    cat "$WORK/create.out" >&2
    exit 1
fi
SID=$(basename "$SESS")

# Seed notes so the hunk marks and the counts are not all empty.
python3 - "$SESS" <<'PY'
import json, sys
json.dump({"title": "fixture", "summary": "one file of every status",
           "files": {"a.go": {"summary": "two constants bumped",
                              "hunks": {"0": {"title": "alpha", "body": "why 11"}, "1": None}}}},
          open(sys.argv[1] + "/notes.json", "w"), ensure_ascii=False, indent=1)
PY

P=$(port_of "$STATE")
echo "fixture session $SID  (repo $FIX/repo, port $P)"

# ── CLI and API surfaces ─────────────────────────────────────────────────────
hud "$STATE" -s "$SID" read              >"$WORK/read"    2>&1 || true
hud "$STATE" -s "$SID" read -W           >"$WORK/read-W"  2>&1 || true
hud "$STATE" -s "$SID" read -U 3         >"$WORK/read-U3" 2>&1 || true
hud "$STATE" -s "$SID" read --file a.go  >"$WORK/read-file" 2>&1 || true
hud "$STATE" session list                >"$WORK/list"    2>&1 || true
hud "$STATE" -s "$SID" session info --json >"$WORK/info"  2>&1 || true
curl -s "http://127.0.0.1:$P/s/$SID/api/session" >"$WORK/api-session"
curl -s "http://127.0.0.1:$P/api/sessions"       >"$WORK/api-sessions"
COLUMNS=80 "$GO" --help                  >"$WORK/help"    2>&1 || true

gold read         mask "$WORK/read"
gold read-W       mask "$WORK/read-W"
gold read-U3      mask "$WORK/read-U3"
gold read-file    mask "$WORK/read-file"
gold session-list mask "$WORK/list"
gold info         json "$WORK/info"
gold api-session  json "$WORK/api-session"
gold api-sessions json "$WORK/api-sessions"
# `hudiff -h` is the agent's manual (spec §3): freezing it catches a flag that
# quietly changed name, moved group or lost its help text.
gold help         mask "$WORK/help"

# ── exit codes (spec §3) ─────────────────────────────────────────────────────
rc=0; hud "$STATE" -s "$SID" read -U nonsense >/dev/null 2>&1 || rc=$?
[ "$rc" = 1 ] && ok "a usage error exits 1" || bad "usage error exit code: $rc (want 1)"
rc=0; ( cd "$FIX" && XDG_STATE_HOME=$STATE "$GO" --no-open --json ) >/dev/null 2>&1 || rc=$?
[ "$rc" = 2 ] && ok "outside a git repo exits 2" || bad "non-repo exit code: $rc (want 2)"

# Restarting has to come back on the same port with a new pid: the page keeps
# its URL, and the old process really is gone (that is the point after a build).
RS=$(new_state)
XDG_STATE_HOME=$RS "$GO" daemon start >/dev/null 2>&1 || true
rs_port=$(port_of "$RS") rs_pid=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1] + "/hudiff/server.json"))["pid"])' "$RS")
XDG_STATE_HOME=$RS "$GO" daemon restart >/dev/null 2>&1 || true
rs_port2=$(port_of "$RS") rs_pid2=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1] + "/hudiff/server.json"))["pid"])' "$RS")
if [ "$rs_port" = "$rs_port2" ] && [ "$rs_pid" != "$rs_pid2" ] &&
   [ "$(curl -s -o /dev/null -w '%{http_code}' --max-time 3 "http://127.0.0.1:$rs_port2/")" = 200 ]; then
    ok "daemon restart keeps the port ($rs_port2), on a new pid"
else
    bad "daemon restart: port $rs_port → $rs_port2, pid $rs_pid → $rs_pid2"
fi

echo "M2: session creation"
. "$ROOT/specs/parity-m2.sh"

echo "M3: the write path"
. "$ROOT/specs/parity-m3.sh"

echo "M4: review / context / diagram"
. "$ROOT/specs/parity-m4.sh"

echo "M5: config / keymap / hooks / skill"
. "$ROOT/specs/parity-m5.sh"

echo "agents: the Send / Request review picker"
. "$ROOT/specs/parity-agents.sh"

# ── the page ─────────────────────────────────────────────────────────────────
# Compared as rendered DOM rather than pixels: the DOM is reproducible, while
# screenshots differ by a sub-pixel, and the diagrams tab animates.
# --virtual-time-budget is needed because the page fetches its own data.
dump() {   # dump <url> <out>
    "$CHROME" --headless --disable-gpu --window-size=1600,1000 \
        --virtual-time-budget=8000 --dump-dom "$1" >"$2" 2>/dev/null
}

if [ "$PAGE" = 1 ]; then
    for v in code diagrams review; do
        dump "http://127.0.0.1:$P/s/$SID/?view=$v" "$WORK/page.$v.html"
        dump "http://127.0.0.1:$P/s/$SID/?view=$v" "$WORK/page2.$v.html"
        if [ ! -s "$WORK/page.$v.html" ]; then
            bad "page-$v  (no DOM capture; is Chrome at $CHROME?)"
            continue
        fi
        gold "page-$v" structure "$WORK/page.$v.html"
        python3 "$ROOT/specs/parity-normalize.py" structure "$WORK/page2.$v.html" >"$WORK/page2.$v.skel"
        if cmp -s "$WORK/page-$v.n" "$WORK/page2.$v.skel"; then
            ok "page-$v renders the same way twice"
        else
            bad "page-$v  two captures of the same page differ"
        fi
    done
    # The home page is its own render path; a JS error there once showed up as an
    # empty list while every session-page check stayed green.
    dump "http://127.0.0.1:$P/" "$WORK/index.html"
    # The browser is told to keep a diagram forever, so the URL has to carry the
    # build: upgrading hudiff changed how they render and the old picture stayed.
    ver=$(hud "$STATE" version)
    build=$(curl -s "http://127.0.0.1:$P/api/config" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("build",""))')
    if [ -n "$ver" ] && [ "${build#"$ver"}" != "$build" ] && [ "${build%-r*}" = "$ver" ] &&
       grep -q "api/diagram?i=[^\"]*v=$build" "$WORK/page.diagrams.html"; then
        ok "diagram URLs carry the build stamp ($build)"
    else
        bad "build stamp: version=$ver build=$build, not in the page's diagram URLs"
    fi
    # Zoom has to open the board that was clicked, so each one is tagged and the
    # lightbox can walk between them; it used to grab whichever img came first.
    if [ "$(grep -o 'class="board" data-board="[0-9]*"' "$WORK/page.diagrams.html" | wc -l | tr -d ' ')" -ge 2 ] &&
       grep -q 'id="lb-prev"' "$WORK/page.diagrams.html" && grep -q 'id="lb-next"' "$WORK/page.diagrams.html"; then
        ok "every board is tagged and the lightbox can step between them"
    else
        bad "board tagging / lightbox nav  ($WORK/page.diagrams.html)"
    fi
    if grep -q 'id="agentpop"' "$WORK/page.code.html"; then
        ok "the page carries the agent picker"
    else
        bad "no agent picker in the page  ($WORK/page.code.html)"
    fi
    if grep -q 'class="lr' "$WORK/index.html"; then
        ok "home page lists sessions"
    else
        bad "home page rendered no session rows  ($WORK/index.html)"
    fi
fi

# The DOM cannot tell whether the stylesheets and bundles actually arrived.
for a in app.css app.js index.css index.js vendor/highlight.min.js \
         vendor/hljs-github.min.css vendor/hljs-github-dark.min.css vendor/hljs-lang-lua.min.js \
         vendor/JetBrainsMono.ttf vendor/JetBrainsMono-OFL.txt; do
    code=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:$P/$a")
    [ "$code" = 200 ] && ok "asset /$a" || bad "asset /$a  (HTTP $code)"
done
# a font served as application/octet-stream is a font the browser will not use
ct=$(curl -s -o /dev/null -w "%{content_type}" "http://127.0.0.1:$P/vendor/JetBrainsMono.ttf")
case $ct in
    font/*) ok "the embedded font is served as $ct" ;;
    *) bad "font content-type is $ct, want font/ttf" ;;
esac

# ── the gate stands on its own ───────────────────────────────────────────────
# The pattern is a regex, so this line does not match itself; comments do not
# count, only a line that would run it.
grep -rn 'hudiff\.py' "$ROOT/specs" --include='*.sh' --include='*.py' 2>/dev/null |
    grep -v ':[0-9]*:[[:space:]]*#' >"$WORK/oracle.refs" || true
if [ -s "$WORK/oracle.refs" ]; then
    bad "the gate still runs the Python oracle: $(tr '\n' ' ' <"$WORK/oracle.refs")"
else
    ok "gate stands on its own (no specs/ script runs the Python oracle)"
fi

if [ "$FAILED" = 0 ]; then
    [ "$BLESS" = 1 ] && echo "goldens written to specs/golden/" || echo "parity ok"
    rm -rf "$WORK"
else
    echo "$FAILED check(s) failed; artifacts in $WORK" >&2
fi
exit "$FAILED"
