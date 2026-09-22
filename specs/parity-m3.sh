#!/bin/sh
# M3 half of the parity gate: the write path. Both implementations run the same
# round trip on their own copy of the fixture session — the page posts a line
# comment and an overall one, hits Send, the agent waits, replies and resolves —
# and must end up with the same comments.
#
# Sourced by parity.sh after parity-m2.sh, which leaves the two sessions in
# $FPS / $FGS and their state dirs in $FP / $FG.
set -eu

port_of() {
    python3 -c 'import json,sys; print(json.load(open(sys.argv[1] + "/hudiff/server.json"))["port"])' "$1"
}

# first_added picks a real "+" line out of hunk 0 so the comment anchors.
first_added() {
    python3 -c 'import json,sys
h = json.load(open(sys.argv[1] + "/diff.json"))["files"][0]["hunks"][0]
ln = next(l for l in h["lines"] if l["t"] == "+")
print(ln["n"])' "$1"
}

post() {   # post <port> <session> <path> <json>
    curl -s -o /dev/null -X POST -H 'Content-Type: application/json' \
        -d "$4" "http://127.0.0.1:$1/s/$2$3"
}

# round_trip <state dir> <session dir> <runner...>
round_trip() {
    state=$1 sdir=$2
    shift 2
    sid=$(basename "$sdir")
    p=$(port_of "$state")
    line=$(first_added "$sdir")
    post "$p" "$sid" /api/comments \
        "{\"target\":\"line\",\"file\":\"a.go\",\"hunk\":0,\"side\":\"new\",\"line\":$line,\"body\":\"why this value?\"}"
    post "$p" "$sid" /api/comments '{"target":"general","body":"overall this reads fine"}'
    post "$p" "$sid" /api/submit '{}'
    XDG_STATE_HOME=$state "$@" -s "$sid" wait --timeout 5 >"$state/wait.out" 2>&1 || true
    XDG_STATE_HOME=$state "$@" -s "$sid" reply c2 "anchored to the new value" >/dev/null 2>&1 || true
    XDG_STATE_HOME=$state "$@" -s "$sid" resolve c2 >/dev/null 2>&1 || true
    XDG_STATE_HOME=$state "$@" -s "$sid" comments --new >"$state/new.out" 2>&1 || true
    XDG_STATE_HOME=$state "$@" -s "$sid" comments --all >"$state/all.out" 2>&1 || true
}

round_trip "$FP" "$FPS" $PY_ORACLE
round_trip "$FG" "$FGS" $GO_BIN

# created / ts are wall clock, and author is ours (spec §8.4); everything else
# about the round trip has to match.
strip_comments() {
    python3 -c 'import json,sys
cs = json.load(open(sys.argv[1]))
for c in cs:
    c.pop("created", None)
    c.pop("author", None)
    for r in c.get("replies", []):
        r.pop("ts", None)
print(json.dumps(cs, sort_keys=True, indent=1, ensure_ascii=False))' "$1"
}
strip_comments "$FPS/comments.json" >"$FP/comments.norm"
strip_comments "$FGS/comments.json" >"$FG/comments.norm"
same "write comments.json" "$FP/comments.norm" "$FG/comments.norm"
same_norm "wait output"     comments "$FP/wait.out" "$FG/wait.out"
same_norm "comments --new"  comments "$FP/new.out"  "$FG/new.out"
same_norm "comments --all"  comments "$FP/all.out"  "$FG/all.out"

# author is ours: spec §8.4 adds it, the oracle has no such field.
if python3 -c 'import json,sys
cs = json.load(open(sys.argv[1]))
assert all(c.get("author") == "user" for c in cs if c["id"] != "c1"), cs' "$FGS/comments.json" 2>/dev/null; then
    ok "comments[].author defaults to user"
else
    bad "comments[].author  ($FGS/comments.json)"
fi

# 124 on timeout, with nothing pending left.
XDG_STATE_HOME=$FG $GO_BIN -s "$(basename "$FGS")" resolve c1 c3 --by user >/dev/null 2>&1 || true
rc=0
XDG_STATE_HOME=$FG $GO_BIN -s "$(basename "$FGS")" wait --timeout 2 >/dev/null 2>&1 || rc=$?
if [ "$rc" = 124 ]; then
    ok "wait exits 124 on timeout"
else
    bad "wait timeout exit code: $rc (want 124)"
fi
