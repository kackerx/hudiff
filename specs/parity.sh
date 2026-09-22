#!/bin/sh
# Parity gate for the Go rewrite: the Python oracle (legacy/hudiff.py) and the
# Go build must agree on `read`, `session info --json`, the session HTTP API
# and the rendered page. Any difference is a bug in the Go side.
#
#   specs/parity.sh [-s <session-id>] [--no-page] [--keep-go]
#
# The Go CLI is English and the oracle is Chinese, so `read` and `session list`
# go through specs/parity-normalize.py first. It collapses exactly the header
# lines and label words to shared tokens — the diff body, hunk tags, line
# numbers, counts, ids and repos are still compared byte for byte.
#
# The two daemons share server.json, so they take turns: the script captures
# the Python side, swaps in the Go daemon, captures again, and puts the Python
# daemon back before it exits (--keep-go leaves the Go one running).
# --no-page skips the headless-Chrome part.
set -eu

cd "$(dirname "$0")/.."
ROOT=$(pwd)
PY="python3 $ROOT/legacy/hudiff.py"
GO="$ROOT/bin/hudiff"
SID=${HUDIFF_PARITY_SESSION:-20260920-155017-4d71}
PAGE=1
KEEP_GO=0
CHROME="/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

while [ $# -gt 0 ]; do
    case $1 in
        -s) SID=$2; shift 2 ;;
        --no-page) PAGE=0; shift ;;
        --keep-go) KEEP_GO=1; shift ;;
        *) echo "usage: $0 [-s <session-id>] [--no-page] [--keep-go]" >&2; exit 1 ;;
    esac
done

WORK=$(mktemp -d)
FAILED=0

restore() {
    command -v sweep_states >/dev/null 2>&1 && sweep_states
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

# same_norm <label> <mode> <a> <b> — compare after collapsing the wordings that
# are allowed to differ (see specs/parity-normalize.py; nothing else is let through).
same_norm() {
    python3 "$ROOT/specs/parity-normalize.py" "$2" "$3" >"$3.norm"
    python3 "$ROOT/specs/parity-normalize.py" "$2" "$4" >"$4.norm"
    same "$1" "$3.norm" "$4.norm"
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

# The page is compared as rendered DOM, not pixels: the DOM is identical and
# reproducible on both sides, while screenshots differ by a sub-pixel now and
# then. It also covers the diagrams tab, whose d2 `steps` animation made pixels
# useless. --virtual-time-budget is still needed; the page fetches its data.
dump() {   # dump <url> <out>
    "$CHROME" --headless --disable-gpu --window-size=1600,1000 \
        --virtual-time-budget=8000 --dump-dom "$1" >"$2" 2>/dev/null
}

port() { python3 -c 'import json,os; print(json.load(open(os.path.expanduser("~/.local/state/hudiff/server.json")))["port"])'; }

# Every diagram of the session, so the d2 pipeline (auto-map source included)
# is compared as bytes rather than through the animated page.
diagram_ids() {
    python3 -c 'import json,sys; print(" ".join(d["id"] for d in json.load(open(sys.argv[1]))["diagrams"]))' "$1"
}

grab() {   # grab <side> <port>
    for v in code diagrams review; do
        [ "$PAGE" = 1 ] && dump "http://127.0.0.1:$2/s/$SID/?view=$v" "$WORK/$1.$v.html"
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
same_norm read         read "$WORK/py.read"          "$WORK/go.read"
same_norm read-W       read "$WORK/py.read-W"        "$WORK/go.read-W"
same_norm read-U3      read "$WORK/py.read-U3"       "$WORK/go.read-U3"
same_norm session-list list "$WORK/py.ls"            "$WORK/go.ls"
same_json info            "$WORK/py.info"          "$WORK/go.info"
same_json api-session     "$WORK/py.api-session"   "$WORK/go.api-session"
same_json api-sessions    "$WORK/py.api-sessions"  "$WORK/go.api-sessions"

# The oracle shells out to d2 with dagre; we compile d2 in with TALA (spec §7),
# so the SVG bytes cannot match. What must hold: both sides render, ours carries
# every node label from its own source, and a second render comes from cache.
for id in $(diagram_ids "$WORK/py.api-session"); do
    for th in light dark; do
        for side in py go; do
            f="$WORK/$side.dg-$id-$th.svg"
            if [ "$(wc -c <"$f" | tr -d ' ')" -gt 500 ] && head -c 200 "$f" | grep -q "<svg"; then
                ok "diagram-$id-$th renders ($side, $(wc -c <"$f" | tr -d ' ') bytes)"
            else
                bad "diagram-$id-$th ($side: $f)"
            fi
        done
    done
    "$GO" -s "$SID" diagram "$id" --src >"$WORK/go.dg-$id.src" 2>/dev/null || true
    if python3 "$ROOT/specs/parity-labels.py" "$WORK/go.dg-$id-light.svg" "$WORK/go.dg-$id.src" 2>"$WORK/label-$id.err"; then
        ok "diagram-$id SVG carries every node label"
    else
        bad "diagram-$id labels ($WORK/label-$id.err)"
    fi
done

before=$(ls "$HOME/.local/state/hudiff/sessions/$SID/diagrams" 2>/dev/null | wc -l | tr -d ' ')
"$GO" -s "$SID" diagram auto-map >/dev/null 2>&1 || true
after=$(ls "$HOME/.local/state/hudiff/sessions/$SID/diagrams" 2>/dev/null | wc -l | tr -d ' ')
if [ "$before" = "$after" ]; then
    ok "diagram cache hit on the second render ($before files)"
else
    bad "diagram cache: $before → $after files"
fi

if [ "$PAGE" = 1 ]; then
    for v in code diagrams review; do
        if [ ! -s "$WORK/go.$v.html" ]; then
            bad "page-$v  (no DOM capture; is Chrome at $CHROME?)"
        else
            same_norm "page-$v" body "$WORK/py.$v.html" "$WORK/go.$v.html"
        fi
    done
fi

# The DOM cannot tell whether the stylesheets and bundles actually arrived.
for a in app.css app.js index.css index.js vendor/highlight.min.js \
         vendor/hljs-github.min.css vendor/hljs-github-dark.min.css vendor/hljs-lang-lua.min.js; do
    code=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:$P/$a")
    if [ "$code" = 200 ]; then
        ok "asset /$a"
    else
        bad "asset /$a  (HTTP $code)"
    fi
done

echo "M2: session creation"
. "$ROOT/specs/parity-m2.sh"

echo "M3: the write path"
. "$ROOT/specs/parity-m3.sh"
echo "M4: review / context / diagram"
. "$ROOT/specs/parity-m4.sh"
rm -rf "$FIX"

if [ "$FAILED" = 0 ]; then
    echo "parity ok"
    rm -rf "$WORK"
else
    echo "$FAILED check(s) failed; artifacts in $WORK" >&2
fi
exit "$FAILED"
