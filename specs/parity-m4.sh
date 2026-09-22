#!/bin/sh
# M4 section: review (two rounds with the user ticking things off in between),
# context, the embedded d2 engine, and the two budgets (binary size, deps).
#
# Sourced by parity.sh, which leaves the fixture session in $SESS / $SID and its
# state dir in $STATE.
set -eu

line=$(python3 -c 'import json,sys
h = json.load(open(sys.argv[1] + "/diff.json"))["files"][0]["hunks"][0]
print(next(l for l in h["lines"] if l["t"] == "+")["n"])' "$SESS")

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

hud "$STATE" -s "$SID" review set "$WORK/r1.json" --author reviewer-A >/dev/null 2>&1 || true
hud "$STATE" -s "$SID" review done r1 "renamed the constant" --by user >/dev/null 2>&1 || true
hud "$STATE" -s "$SID" review wontfix r3 "style only" --by user >/dev/null 2>&1 || true
hud "$STATE" -s "$SID" review set "$WORK/r2.json" --author reviewer-A >/dev/null 2>&1 || true
hud "$STATE" -s "$SID" review md   >"$WORK/review.md"   2>&1 || true
hud "$STATE" -s "$SID" review show >"$WORK/review.show" 2>&1 || true

gold review.json  json "$SESS/review.json"
gold review-md    mask "$WORK/review.md"
gold review-show  mask "$WORK/review.show"

# --replace throws the old report away, including the user's ticks.
hud "$STATE" -s "$SID" review set "$WORK/r1.json" --replace --author reviewer-A >/dev/null 2>&1 || true
gold review-replace.json json "$SESS/review.json"

# ── context ─────────────────────────────────────────────────────────────────
hud "$STATE" -s "$SID" context >"$WORK/context.out" 2>&1 || true
gold context mask "$WORK/context.out"

# ── diagrams ────────────────────────────────────────────────────────────────
# The auto map is generated from repoctx, so its d2 source is the real output of
# that pipeline: freezing the source catches a change no SVG byte count would.
hud "$STATE" -s "$SID" diagram auto-map --src >"$WORK/auto.d2" 2>/dev/null || true
gold auto-map-src mask "$WORK/auto.d2"

for th in light dark; do
    hud "$STATE" -s "$SID" diagram auto-map --theme "$th" >"$WORK/auto-$th.svg" 2>"$WORK/auto-$th.err" || true
    if [ "$(wc -c <"$WORK/auto-$th.svg" | tr -d ' ')" -gt 500 ] && head -c 200 "$WORK/auto-$th.svg" | grep -q "<svg"; then
        ok "auto-map renders ($th, $(wc -c <"$WORK/auto-$th.svg" | tr -d ' ') bytes)"
    else
        bad "auto-map render $th  ($WORK/auto-$th.svg $WORK/auto-$th.err)"
    fi
done
if python3 "$ROOT/specs/parity-labels.py" "$WORK/auto-light.svg" "$WORK/auto.d2" 2>"$WORK/label.err"; then
    ok "auto-map SVG carries every node label"
else
    bad "auto-map labels  ($WORK/label.err)"
fi

# Second render must come from the cache, not the compiler.
before=$(ls "$SESS/diagrams" 2>/dev/null | wc -l | tr -d ' ')
hud "$STATE" -s "$SID" diagram auto-map >/dev/null 2>&1 || true
after=$(ls "$SESS/diagrams" 2>/dev/null | wc -l | tr -d ' ')
if [ "$before" = "$after" ] && [ "$before" -gt 0 ]; then
    ok "diagram cache hit on the second render ($before file(s))"
else
    bad "diagram cache: $before → $after files"
fi

# ── notes: the agent's own diagrams, one of them multi-board ────────────────
cat >"$WORK/notes.json" <<'JSON'
{"title":"fixture","summary":"one file of every status",
 "diagrams":[
   {"id":"flow","title":"call flow","kind":"d2","caption":"who calls what","hunks":["a.go#0"],
    "src":"alpha: {label: \"alpha() = 11\"; class: added}\nbeta: {label: \"beta() = 222\"; class: ext}\nalpha -> beta: after\n"},
   {"id":"steps","title":"request path","kind":"d2","caption":"two boards","hunks":[],
    "src":"steps: {\n  1: {\n    client -> server: request\n  }\n  2: {\n    server -> db: query\n  }\n}\n"}],
 "files":{"a.go":{"summary":"two constants bumped","hunks":{"0":{"title":"alpha","body":"why 11"}}}}}
JSON
if hud "$STATE" -s "$SID" notes "$WORK/notes.json" >"$WORK/notes.out" 2>&1; then
    ok "notes accepts the agent's diagrams and compiles them"
else
    bad "notes rejected a valid file  ($WORK/notes.out)"
fi
hud "$STATE" -s "$SID" diagram list >"$WORK/diagram-list" 2>&1 || true
gold diagram-list mask "$WORK/diagram-list"

for id in flow steps; do
    for th in light dark; do
        hud "$STATE" -s "$SID" diagram "$id" --theme "$th" >"$WORK/dg-$id-$th.svg" 2>"$WORK/dg-$id-$th.err" || true
        if [ "$(wc -c <"$WORK/dg-$id-$th.svg" | tr -d ' ')" -gt 500 ] && head -c 200 "$WORK/dg-$id-$th.svg" | grep -q "<svg"; then
            ok "diagram $id renders ($th, $(wc -c <"$WORK/dg-$id-$th.svg" | tr -d ' ') bytes)"
        else
            bad "diagram $id $th  ($WORK/dg-$id-$th.svg $WORK/dg-$id-$th.err)"
        fi
    done
done
if grep -q 'alpha() = 11' "$WORK/dg-flow-light.svg" && grep -q 'beta() = 222' "$WORK/dg-flow-light.svg"; then
    ok "the agent's labels reach the SVG"
else
    bad "labels missing from $WORK/dg-flow-light.svg"
fi
# The palette lives in diagram.Preamble, so an agent's own `class: added` has to
# style the node — it used to be defined only inside the generated change map,
# where d2 silently ignored it everywhere else.
if grep -q '#dafbe1' "$WORK/dg-flow-light.svg" && grep -q '#1a7f37' "$WORK/dg-flow-light.svg"; then
    ok "the shared palette styles an agent's own diagram (class: added)"
else
    bad "class: added had no effect  ($WORK/dg-flow-light.svg)"
fi
# A steps diagram is several boards stitched into one animated SVG (spec §7).
if [ "$(grep -c '<svg' "$WORK/dg-steps-light.svg")" -gt 1 ] || grep -q 'animate\|@keyframes' "$WORK/dg-steps-light.svg"; then
    ok "a steps diagram renders as an animated multi-board SVG"
else
    bad "steps rendered as a single static board  ($WORK/dg-steps-light.svg)"
fi

# The page asks for every diagram at once, and a shared d2 ruler turned that
# into "fatal error: concurrent map writes" — which takes the whole daemon down,
# not just one request. Sequential CLI renders can never see it.
rm -rf "$SESS/diagrams"
for id in auto-map flow steps; do
    for th in light dark; do
        curl -s -o "$WORK/burst-$id-$th.svg" \
            "http://127.0.0.1:$P/s/$SID/api/diagram?i=$id&theme=$th" &
    done
done
wait
burst=$(grep -l '<svg' "$WORK"/burst-*.svg 2>/dev/null | wc -l | tr -d ' ')
alive=$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 "http://127.0.0.1:$P/api/sessions")
if [ "$burst" = 6 ] && [ "$alive" = 200 ]; then
    ok "six diagrams rendered in parallel, the daemon survived"
else
    bad "parallel render: $burst/6 SVGs, daemon HTTP $alive  ($WORK/burst-*.svg)"
fi

# ── notes is a trust boundary: bad input is rejected, with the reason ────────
printf '{"diagrams":[{"id":"bad","title":"broken","kind":"d2","src":"a -> : {{{"}]}' >"$WORK/bad.json"
if ! hud "$STATE" -s "$SID" notes "$WORK/bad.json" >"$WORK/notes-bad.out" 2>"$WORK/notes-bad.err"; then
    # and the line number is the agent's own, not one shifted by the preamble
    if grep -q "d2 failed to compile" "$WORK/notes-bad.err" &&
       grep -qE '(^|[^0-9])1:[0-9]+:' "$WORK/notes-bad.err"; then
        ok "notes rejects a broken d2 source, pointing at the agent's own line"
    else
        bad "notes d2 rejection message  ($WORK/notes-bad.err)"
    fi
else
    bad "notes accepted a broken d2 source"
fi
printf '{"files":{"a.go":{"hunks":{"99":{"title":"nowhere"}}}}}' >"$WORK/schema.json"
if ! hud "$STATE" -s "$SID" notes "$WORK/schema.json" >"$WORK/notes-schema.out" 2>"$WORK/notes-schema.err"; then
    ok "notes rejects a hunk number the diff does not have"
else
    bad "notes accepted a note on a hunk that does not exist"
fi

# ── budgets ─────────────────────────────────────────────────────────────────
go build -trimpath -ldflags "-s -w" -o "$WORK/stripped" "$ROOT"
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
