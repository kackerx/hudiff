#!/bin/sh
# Parity gate for the Go rewrite: the Python oracle (legacy/hudiff.py) and the
# Go build must agree on `read`, `session info --json`, the session HTTP API
# and the rendered page. Any difference is a bug in the Go side.
#
#   specs/parity.sh [-s <session-id>] [--no-shots] [--keep-go]
#
# The two daemons share server.json, so they take turns: the script captures
# the Python side, swaps in the Go daemon, captures again, and puts the Python
# daemon back before it exits (--keep-go leaves the Go one running).
set -eu

cd "$(dirname "$0")/.."
ROOT=$(pwd)
PY="python3 $ROOT/legacy/hudiff.py"
GO="$ROOT/bin/hudiff"
SID=${HUDIFF_PARITY_SESSION:-20260920-155017-4d71}
SHOTS=1
KEEP_GO=0
CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

while [ $# -gt 0 ]; do
    case $1 in
        -s) SID=$2; shift 2 ;;
        --no-shots) SHOTS=0; shift ;;
        --keep-go) KEEP_GO=1; shift ;;
        *) echo "usage: $0 [-s <session-id>] [--no-shots] [--keep-go]" >&2; exit 1 ;;
    esac
done

WORK=$(mktemp -d)
FAILED=0

restore() {
    [ "$KEEP_GO" = 1 ] && return 0
    "$GO" daemon stop >/dev/null 2>&1 || true
    $PY -s "$SID" url >/dev/null 2>&1 || true
}
trap restore EXIT

ok()   { printf '  \033[32mok\033[0m    %s\n' "$1"; }
bad()  { printf '  \033[31mFAIL\033[0m  %s\n' "$1"; FAILED=$((FAILED + 1)); }

same() {   # same <label> <a> <b>
    if diff -u "$2" "$3" >"$WORK/$1.diff" 2>&1; then
        ok "$1"
    else
        bad "$1  ($WORK/$1.diff, $(wc -l <"$WORK/$1.diff" | tr -d ' ') lines)"
    fi
}

same_json() {
    for f in "$2" "$3"; do
        if ! python3 -c 'import json,sys; print(json.dumps(json.load(open(sys.argv[1])), sort_keys=True, indent=1, ensure_ascii=False))' \
                "$f" >"$f.norm" 2>"$f.err"; then
            bad "$1  (not JSON: $f)"
            return 0
        fi
    done
    same "$1" "$2.norm" "$3.norm"
}

shot() {   # shot <url> <out>
    "$CHROME" --headless --disable-gpu --hide-scrollbars --window-size=1600,1000 \
        --virtual-time-budget=8000 --screenshot="$2" "$1" >/dev/null 2>&1
}

port() { python3 -c 'import json,os; print(json.load(open(os.path.expanduser("~/.local/state/hudiff/server.json")))["port"])'; }

# Every diagram of the session, so the d2 pipeline (auto-map source included)
# is compared as bytes rather than through the animated page.
diagram_ids() {
    python3 -c 'import json,sys; print(" ".join(d["id"] for d in json.load(open(sys.argv[1]))["diagrams"]))' "$1"
}

grab() {   # grab <side> <port>
    for v in code diagrams review; do
        [ "$SHOTS" = 1 ] && shot "http://127.0.0.1:$2/s/$SID/?view=$v" "$WORK/$1.$v.png"
    done
    for id in $(diagram_ids "$WORK/$1.api-session"); do
        for th in light dark; do
            curl -s "http://127.0.0.1:$2/s/$SID/api/diagram?i=$id&theme=$th" >"$WORK/$1.dg-$id-$th.svg"
        done
    done
}

echo "building Go binary…"
mkdir -p "$ROOT/bin"
go build -o "$GO" .

# ── Python side ──────────────────────────────────────────────────────────────
echo "capturing the oracle…"
$PY -s "$SID" url >/dev/null
P=$(port)
$PY -s "$SID" show            >"$WORK/py.read"
$PY -s "$SID" show -W         >"$WORK/py.read-W"
$PY -s "$SID" show -U 3       >"$WORK/py.read-U3"
$PY -s "$SID" info            >"$WORK/py.info"
$PY ls                        >"$WORK/py.ls"
curl -s "http://127.0.0.1:$P/s/$SID/api/session"  >"$WORK/py.api-session"
curl -s "http://127.0.0.1:$P/api/sessions"        >"$WORK/py.api-sessions"
grab py "$P"
$PY stop >/dev/null

# ── Go side ──────────────────────────────────────────────────────────────────
echo "capturing the Go build…"
"$GO" daemon start >/dev/null
P=$(port)
"$GO" -s "$SID" read          >"$WORK/go.read"
"$GO" -s "$SID" read -W       >"$WORK/go.read-W"
"$GO" -s "$SID" read -U 3     >"$WORK/go.read-U3"
"$GO" -s "$SID" session info --json >"$WORK/go.info"
"$GO" session list            >"$WORK/go.ls"
curl -s "http://127.0.0.1:$P/s/$SID/api/session"  >"$WORK/go.api-session"
curl -s "http://127.0.0.1:$P/api/sessions"        >"$WORK/go.api-sessions"
grab go "$P"

# ── Compare ──────────────────────────────────────────────────────────────────
echo "session $SID"
same      read            "$WORK/py.read"          "$WORK/go.read"
same      read-W          "$WORK/py.read-W"        "$WORK/go.read-W"
same      read-U3         "$WORK/py.read-U3"       "$WORK/go.read-U3"
same      session-list    "$WORK/py.ls"            "$WORK/go.ls"
same_json info            "$WORK/py.info"          "$WORK/go.info"
same_json api-session     "$WORK/py.api-session"   "$WORK/go.api-session"
same_json api-sessions    "$WORK/py.api-sessions"  "$WORK/go.api-sessions"

for id in $(diagram_ids "$WORK/py.api-session"); do
    for th in light dark; do
        if cmp -s "$WORK/py.dg-$id-$th.svg" "$WORK/go.dg-$id-$th.svg"; then
            ok "diagram-$id-$th"
        else
            bad "diagram-$id-$th  ($WORK/py.dg-$id-$th.svg vs $WORK/go.dg-$id-$th.svg)"
        fi
    done
done

# The checks above all hit the shared SVG cache, so they only prove the d2
# source matches. This one renders from cold in an isolated state dir, which
# also compares the d2 invocation itself.
COLD=$(mktemp -d)
mkdir -p "$COLD/hudiff/sessions"
cp -R "$HOME/.local/state/hudiff/sessions/$SID" "$COLD/hudiff/sessions/"
rm -rf "$COLD/hudiff/sessions/$SID/diagrams"
XDG_STATE_HOME=$COLD "$GO" daemon start >/dev/null
CP=$(python3 -c "import json;print(json.load(open('$COLD/hudiff/server.json'))['port'])")
for id in $(diagram_ids "$WORK/py.api-session"); do
    for th in light dark; do
        curl -s "http://127.0.0.1:$CP/s/$SID/api/diagram?i=$id&theme=$th" >/dev/null
    done
done
XDG_STATE_HOME=$COLD "$GO" daemon stop >/dev/null
for f in "$COLD/hudiff/sessions/$SID/diagrams/"*.svg; do
    b=$(basename "$f")
    if cmp -s "$f" "$HOME/.local/state/hudiff/sessions/$SID/diagrams/$b"; then
        ok "d2-cold-render $b"
    else
        bad "d2-cold-render $b  ($f)"
    fi
done
[ "$FAILED" = 0 ] && rm -rf "$COLD"

if [ "$SHOTS" = 1 ]; then
    for v in code review; do
        if [ ! -s "$WORK/go.$v.png" ]; then
            bad "page-$v  (no screenshot; is Chrome at $CHROME?)"
        elif cmp -s "$WORK/py.$v.png" "$WORK/go.$v.png"; then
            ok "page-$v"
        else
            bad "page-$v  ($WORK/py.$v.png vs $WORK/go.$v.png)"
        fi
    done
    # The diagrams tab holds a d2 `steps` animation, so its pixels move between
    # runs; the SVGs above are the real check, this only proves it renders.
    if [ -s "$WORK/go.diagrams.png" ]; then
        ok "page-diagrams (rendered; pixels not compared, d2 steps animation)"
    else
        bad "page-diagrams  (no screenshot; is Chrome at $CHROME?)"
    fi
fi

if [ "$FAILED" = 0 ]; then
    echo "parity ok"
    rm -rf "$WORK"
else
    echo "$FAILED check(s) failed; artifacts in $WORK" >&2
fi
exit "$FAILED"
