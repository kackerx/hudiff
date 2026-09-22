#!/bin/sh
# Agent discovery: what the Send / Request review picker lists. The fakes are
# processes carrying an agent's name with their cwd in the fixture repo, which
# is exactly what discovery matches on — no agent has to be installed.
#
# It runs against its own copy of the session so that binding an agent does not
# leave a bound-but-dead agent in the header the page golden captures later.
#
# Sourced by parity.sh; uses ok/bad/new_state/port_of and $FIX/$SESS/$GO.
set -eu

AG_STATE=$(new_state)
mkdir -p "$AG_STATE/hudiff/sessions"
cp -R "$SESS" "$AG_STATE/hudiff/sessions/"
AG_SID=$(basename "$SESS")
XDG_STATE_HOME=$AG_STATE "$GO" daemon start >/dev/null 2>&1 || true
AG_P=$(port_of "$AG_STATE")

# `exec -a` renames the process without installing anything; sleep keeps it alive.
# The redirect matters: without it the background process holds the pipe that
# $(fake …) reads, and the command substitution waits for it to exit.
fake() { ( cd "$1" && exec sh -c 'exec -a claude sleep 120' ) >/dev/null 2>&1 & echo $!; }
agents_json() { curl -s "http://127.0.0.1:$AG_P/s/$AG_SID/api/agents"; }

A1=$(fake "$FIX/repo")
sleep 0.4
if agents_json | python3 -c 'import json,sys
d = json.load(sys.stdin)
a = [x for x in d["agents"] if x["pid"] == int(sys.argv[1])]
assert a, d["agents"]
assert a[0]["kind"] == "claude", a
assert a[0]["name"] == "repo", a          # no name on the command line → the directory
assert a[0]["cwd"].endswith("/repo"), a
assert a[0]["started"], a' "$A1" 2>"$WORK/ag1.err"; then
    ok "an agent running in the repo shows up, named after its directory"
else
    bad "agent discovery  ($WORK/ag1.err, $(agents_json | head -c 300))"
fi

# An agent outside the repo is somebody else's business.
OUT=$(fake "$FIX")
sleep 0.4
if agents_json | python3 -c 'import json,sys
pids = [a["pid"] for a in json.load(sys.stdin)["agents"]]
assert int(sys.argv[1]) not in pids, pids' "$OUT" 2>"$WORK/ag2.err"; then
    ok "an agent outside the repo is not listed"
else
    bad "agent outside the repo leaked into the list  ($WORK/ag2.err)"
fi

# Two in the same directory: numbered by which started first. ps reports whole
# seconds, so the second one has to start at least a second later.
sleep 1.5
A2=$(fake "$FIX/repo")
sleep 0.4
if agents_json | python3 -c 'import json,sys
by = {a["pid"]: a["name"] for a in json.load(sys.stdin)["agents"]}
first, second = int(sys.argv[1]), int(sys.argv[2])
assert by.get(first) == "repo:1", by
assert by.get(second) == "repo:2", by' "$A1" "$A2" 2>"$WORK/ag3.err"; then
    ok "two agents in one directory are numbered by start order"
else
    bad "agent numbering  ($WORK/ag3.err, $(agents_json | head -c 300))"
fi

# Picking one in the page writes it into session.json and the header goes live.
curl -s -o /dev/null -X POST -H 'Content-Type: application/json' -d "{\"pid\":$A2}" \
    "http://127.0.0.1:$AG_P/s/$AG_SID/api/submit"
if python3 -c 'import json,sys
b = json.load(open(sys.argv[1]))["bound"]
assert b and b["pid"] == int(sys.argv[2]), b
assert b["kind"] == "claude" and b["name"] == "repo:2", b' \
   "$AG_STATE/hudiff/sessions/$AG_SID/session.json" "$A2" 2>"$WORK/ag4.err"; then
    ok "Send with an agent picked records it in session.json"
else
    bad "bound not written  ($WORK/ag4.err)"
fi
if curl -s "http://127.0.0.1:$AG_P/s/$AG_SID/api/state" |
   python3 -c 'import json,sys; assert json.load(sys.stdin)["bound_alive"] is True' 2>/dev/null; then
    ok "bound_alive is true while the agent is running"
else
    bad "bound_alive should be true"
fi

# ── delivery ────────────────────────────────────────────────────────────────
# The matchers for tmux / kitty / cmux run against fixture output, so they are
# checked on a machine that has none of those installed.
if go test ./internal/agents/ >"$WORK/agents-test.out" 2>&1; then
    ok "delivery matchers (tmux tty, kitty window, ancestor walk)"
else
    bad "internal/agents tests  ($WORK/agents-test.out)"
fi

# A fake with no controlling terminal matches no adapter, and that has to come
# back as copy rather than as silence.
if curl -s -X POST -H 'Content-Type: application/json' -d "{\"pid\":$A1,\"text\":\"hello\"}" \
   "http://127.0.0.1:$AG_P/s/$AG_SID/api/deliver" |
   python3 -c 'import json,sys
d = json.load(sys.stdin)
assert d["via"] == "copy", d
assert d["detail"], d' 2>"$WORK/ag5.err"; then
    ok "an agent with no terminal falls back to copy, with a reason"
else
    bad "deliver fallback  ($WORK/ag5.err)"
fi

# tmux for real: its own server, an agent-looking process in the fixture repo,
# and the command has to show up in the pane.
TSOCK=hudiff-parity
tmux -L "$TSOCK" kill-server 2>/dev/null || true   # a run that died before its cleanup
if tmux -L "$TSOCK" new -d -c "$FIX/repo" 'exec -a claude cat' 2>/dev/null; then
    sleep 0.6
    TPID=$(tmux -L "$TSOCK" list-panes -a -F '#{pane_pid}')
    TPANE=$(tmux -L "$TSOCK" list-panes -a -F '#{pane_id}')
    if agents_json | python3 -c 'import json,sys
pids = [a["pid"] for a in json.load(sys.stdin)["agents"]]
assert int(sys.argv[1]) in pids, pids' "$TPID" 2>"$WORK/ag6.err"; then
        ok "an agent inside a tmux pane is discovered"
    else
        bad "tmux pane agent not discovered  ($WORK/ag6.err)"
    fi
    via=$(curl -s -X POST -H 'Content-Type: application/json' \
        -d "{\"pid\":$TPID,\"text\":\"hudiff -s $AG_SID comments --new\"}" \
        "http://127.0.0.1:$AG_P/s/$AG_SID/api/deliver" |
        python3 -c 'import json,sys; print(json.load(sys.stdin)["via"])')
    sleep 0.5
    if [ "$via" = tmux ] &&
       tmux -L "$TSOCK" capture-pane -p -t "$TPANE" | grep -q "hudiff -s $AG_SID comments --new"; then
        ok "deliver types the command into the agent's tmux pane"
    else
        bad "tmux delivery: via=$via, pane did not receive the command"
    fi
    tmux -L "$TSOCK" kill-server 2>/dev/null || true
else
    bad "could not start a tmux server for the delivery check (is tmux installed?)"
fi

# the copy path's wording, where the CLI says it out loud
XDG_STATE_HOME=$AG_STATE "$GO" -s "$AG_SID" agents deliver 0 'hudiff -s x comments --new' \
    >"$WORK/deliver-copy.out" 2>&1 || true
gold agents-deliver-copy mask "$WORK/deliver-copy.out"

kill "$A1" "$A2" "$OUT" 2>/dev/null || true
sleep 0.3
if curl -s "http://127.0.0.1:$AG_P/s/$AG_SID/api/state" |
   python3 -c 'import json,sys; assert json.load(sys.stdin)["bound_alive"] is False' 2>/dev/null; then
    ok "bound_alive goes false once the agent exits"
else
    bad "bound_alive should be false after the agent is gone"
fi
if [ "$(agents_json | python3 -c 'import json,sys; print(len(json.load(sys.stdin)["agents"]))')" = 0 ]; then
    ok "the list empties when the agents are gone"
else
    bad "dead agents still listed  ($(agents_json | head -c 200))"
fi
XDG_STATE_HOME=$AG_STATE "$GO" daemon stop >/dev/null 2>&1 || true
