#!/bin/sh
# M3 section: the write path. The page posts a line comment and an overall one
# and hits Send; the agent waits, replies and resolves. comments.json and the
# three CLI renderings of it are frozen.
#
# Sourced by parity.sh, which leaves the fixture session in $SESS / $SID, its
# state dir in $STATE and its daemon on $P.
set -eu

# a real "+" line out of hunk 0, so the comment anchors to content
line=$(python3 -c 'import json,sys
h = json.load(open(sys.argv[1] + "/diff.json"))["files"][0]["hunks"][0]
print(next(l for l in h["lines"] if l["t"] == "+")["n"])' "$SESS")

post() {   # post <path> <json>
    curl -s -o /dev/null -X POST -H 'Content-Type: application/json' \
        -d "$2" "http://127.0.0.1:$P/s/$SID$1"
}

post /api/comments \
    "{\"target\":\"line\",\"file\":\"a.go\",\"hunk\":0,\"side\":\"new\",\"line\":$line,\"body\":\"why this value?\"}"
post /api/comments '{"target":"general","body":"overall this reads fine"}'
post /api/submit '{}'

hud "$STATE" -s "$SID" wait --timeout 5 >"$WORK/wait.out" 2>&1 || true
hud "$STATE" -s "$SID" reply c2 "anchored to the new value" >/dev/null 2>&1 || true
hud "$STATE" -s "$SID" resolve c2 >/dev/null 2>&1 || true
hud "$STATE" -s "$SID" comments --new >"$WORK/comments-new.out" 2>&1 || true
hud "$STATE" -s "$SID" comments --all >"$WORK/comments-all.out" 2>&1 || true

gold write-comments.json json "$SESS/comments.json"
gold wait-output   mask "$WORK/wait.out"
gold comments-new  mask "$WORK/comments-new.out"
gold comments-all  mask "$WORK/comments-all.out"

# author is ours (spec §8.4): the page writes "user", `reply` writes "agent".
if python3 -c 'import json,sys
cs = json.load(open(sys.argv[1]))
assert all(c.get("author") == "user" for c in cs if c["id"] != "c1"), cs
# a reply is stored as {who, body, ts}: the shape on disk, frozen by spec 8
assert any(r.get("who") == "agent" for c in cs for r in c.get("replies", [])), cs' \
   "$SESS/comments.json" 2>"$WORK/author.err"; then
    ok "comments[].author: the page writes user, reply writes agent"
else
    bad "comments[].author  ($SESS/comments.json $WORK/author.err)"
fi

# 124 on timeout, with nothing pending left.
hud "$STATE" -s "$SID" resolve c1 c3 --by user >/dev/null 2>&1 || true
rc=0
hud "$STATE" -s "$SID" wait --timeout 2 >/dev/null 2>&1 || rc=$?
if [ "$rc" = 124 ]; then
    ok "wait exits 124 on timeout"
else
    bad "wait timeout exit code: $rc (want 124)"
fi
