#!/bin/sh
# M4 half of the parity gate: review, context, diagram, and the two budgets
# (binary size, dependency count).
#
# Sourced by parity.sh m4_after parity-m3.sh, which leaves both fixture sessions
# in $FPS / $FGS and their state dirs in $FP / $FG.
set -eu

# ── review: two rounds, with the user ticking things off in between ──────────
review_round() {   # review_round <state> <session dir> <json file> <runner...>
    state=$1 sdir=$2 file=$3
    shift 3
    XDG_STATE_HOME=$state "$@" -s "$(basename "$sdir")" review set "$file" --author reviewer-A \
        >"$state/review.$(basename "$file").out" 2>&1 || true
}

line=$(python3 -c 'import json,sys
h = json.load(open(sys.argv[1]))["files"][0]["hunks"][0]
print(next(l["n"] for l in h["lines"] if l["t"] == "+"))' "$FPS/diff.json")

cat >"$WORK/r1.json" <<JSON
{"summary":"first pass","strengths":["clear naming"],
 "items":[{"id":"r1","priority":"high","title":"magic number","file":"a.go","line":$line,"body":"why 11","suggestion":"name it"},
          {"id":"r2","priority":"medium","title":"no test","file":"a.go","body":"add one"},
          {"id":"r3","priority":"low","title":"comment wording","file":"a.go","line":$line,"body":"nit"}]}
JSON
# round 2: r1 keeps its place (text update), r3 comes back under a new id at the
# same place with the same title, and r2 gets a re-review note.
cat >"$WORK/r2.json" <<JSON
{"summary":"second pass","strengths":["clear naming","tests added"],
 "items":[{"id":"r1","priority":"high","title":"magic number","file":"a.go","line":$line,"body":"still magic","suggestion":"name it properly"},
          {"id":"r9","priority":"low","title":"comment wording","file":"a.go","line":$line,"body":"same spot, renumbered"},
          {"id":"r2","note":"checked, the test landed","verdict":"ok"},
          {"id":"r4","priority":"medium","title":"brand new finding","file":"a.go","body":"round two"}]}
JSON

review_flow() {   # review_flow <state> <session dir> <runner...>
    state=$1 sdir=$2
    shift 2
    sid=$(basename "$sdir")
    review_round "$state" "$sdir" "$WORK/r1.json" "$@"
    XDG_STATE_HOME=$state "$@" -s "$sid" review done r1 "renamed the constant" --by user >/dev/null 2>&1 || true
    XDG_STATE_HOME=$state "$@" -s "$sid" review wontfix r3 "style only" --by user >/dev/null 2>&1 || true
    review_round "$state" "$sdir" "$WORK/r2.json" "$@"
    XDG_STATE_HOME=$state "$@" -s "$sid" review md >"$state/review.md" 2>&1 || true
}

review_flow "$FP" "$FPS" $PY_ORACLE
review_flow "$FG" "$FGS" $GO_BIN

strip_review() {
    python3 -c 'import json,sys
rv = json.load(open(sys.argv[1]))
rv.pop("updated", None)
rv.pop("_merge", None)
for r in rv.get("rounds", []):
    r.pop("at", None)
for it in rv.get("items", []):
    it.pop("resolved_at", None)
    for n in it.get("notes", []):
        n.pop("ts", None)
print(json.dumps(rv, sort_keys=True, indent=1, ensure_ascii=False))' "$1"
}
strip_review "$FPS/review.json" >"$FP/review.norm"
strip_review "$FGS/review.json" >"$FG/review.norm"
same "review.json m4_after two rounds" "$FP/review.norm" "$FG/review.norm"
same_norm "review md" md "$FP/review.md" "$FG/review.md"

# --replace throws the old report away, including the user's ticks.
XDG_STATE_HOME=$FP $PY_ORACLE -s "$(basename "$FPS")" review set "$WORK/r1.json" --replace --author reviewer-A >/dev/null 2>&1 || true
XDG_STATE_HOME=$FG $GO_BIN    -s "$(basename "$FGS")" review set "$WORK/r1.json" --replace --author reviewer-A >/dev/null 2>&1 || true
strip_review "$FPS/review.json" >"$FP/replace.norm"
strip_review "$FGS/review.json" >"$FG/replace.norm"
same "review --replace" "$FP/replace.norm" "$FG/replace.norm"

# ── context ─────────────────────────────────────────────────────────────────
XDG_STATE_HOME=$FP $PY_ORACLE -s "$(basename "$FPS")" ctx     >"$FP/ctx.out" 2>&1 || true
XDG_STATE_HOME=$FG $GO_BIN    -s "$(basename "$FGS")" context >"$FG/ctx.out" 2>&1 || true
same_norm "context" context "$FP/ctx.out" "$FG/ctx.out"

# ── diagram ─────────────────────────────────────────────────────────────────
# The oracle lays out with dagre and we lay out with TALA, so the SVGs cannot
# match byte for byte. Both must compile, and ours must carry every node label.
XDG_STATE_HOME=$FP $PY_ORACLE -s "$(basename "$FPS")" diagram auto-map >"$FP/auto.svg" 2>"$FP/auto.err" || true
XDG_STATE_HOME=$FG $GO_BIN    -s "$(basename "$FGS")" diagram auto-map >"$FG/auto.svg" 2>"$FG/auto.err" || true
XDG_STATE_HOME=$FG $GO_BIN    -s "$(basename "$FGS")" diagram auto-map --src >"$FG/auto.d2" 2>/dev/null || true
for side in "$FP" "$FG"; do
    if head -c 5 "$side/auto.svg" | grep -q "<" && [ "$(wc -c <"$side/auto.svg")" -gt 500 ]; then
        ok "auto-map renders ($(basename "$side"), $(wc -c <"$side/auto.svg" | tr -d ' ') bytes)"
    else
        bad "auto-map render ($side/auto.svg, $side/auto.err)"
    fi
done
if python3 "$ROOT/specs/parity-labels.py" "$FG/auto.svg" "$FG/auto.d2" 2>"$FG/label.err"; then
    ok "auto-map SVG carries every node label"
else
    bad "auto-map labels  ($FG/label.err)"
fi

# Second render must come from the cache, not the compiler.
m4_before=$(ls "$FGS/diagrams" | wc -l | tr -d ' ')
XDG_STATE_HOME=$FG $GO_BIN -s "$(basename "$FGS")" diagram auto-map >/dev/null 2>&1 || true
m4_after=$(ls "$FGS/diagrams" | wc -l | tr -d ' ')
if [ "$m4_before" = "$m4_after" ] && [ "$m4_before" -gt 0 ]; then
    ok "diagram cache hit on the second render ($m4_before file(s))"
else
    bad "diagram cache: $m4_before → $m4_after files"
fi

# ── notes rejects a d2 source that does not compile ─────────────────────────
printf '{"diagrams":[{"id":"bad","title":"broken","kind":"d2","src":"a -> : {{{"}]}' >"$WORK/bad.json"
if ! XDG_STATE_HOME=$FG $GO_BIN -s "$(basename "$FGS")" notes "$WORK/bad.json" >"$FG/notes.out" 2>"$FG/notes.err"; then
    if grep -q "d2 failed to compile" "$FG/notes.err" && [ "$(wc -c <"$FG/notes.err")" -gt 40 ]; then
        ok "notes rejects a broken d2 source with the compiler's message"
    else
        bad "notes d2 rejection message  ($FG/notes.err)"
    fi
else
    bad "notes accepted a broken d2 source"
fi

# ── budgets ─────────────────────────────────────────────────────────────────
go build -ldflags "-s -w" -o "$WORK/stripped" "$ROOT"
bytes=$(wc -c <"$WORK/stripped" | tr -d ' ')
mods=$(grep -c ' v[0-9]' "$ROOT/go.mod" || true)
if [ "$bytes" -le 33554432 ]; then
    ok "binary $bytes bytes stripped (budget 33554432), go.mod modules $mods"
else
    bad "binary $bytes bytes stripped, over the 32MB budget; go.mod modules $mods"
fi

# d2 must stay inside internal/diagram.
leak=$(cd "$ROOT" && for p in $(go list ./internal/...); do
    [ "$p" = "github.com/kackerx/hudiff/internal/diagram" ] && continue
    if go list -deps "$p" 2>/dev/null | grep -q '^github.com/d2lang/d2$'; then echo "$p"; fi
done || true)
if [ -z "$leak" ]; then
    ok "d2 is imported only by internal/diagram"
else
    bad "d2 leaked into: $leak"
fi
