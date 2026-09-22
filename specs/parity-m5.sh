#!/bin/sh
# M5 half of the parity gate: config, keymap, hooks, skill install, --ascii.
# These are ours alone — the oracle has no config file, no keymap and no hooks —
# so they are assertions about the Go build rather than comparisons.
#
# Sourced by parity.sh; uses ok/bad/$WORK/$ROOT/$GO/$FGS.
set -eu

CFG=$(mktemp -d)
remember "$CFG/state"   # the daemon writes its server.json under XDG_STATE_HOME

hud() { XDG_CONFIG_HOME=$CFG XDG_STATE_HOME=$CFG/state "$GO" "$@"; }

# ── config: no file means defaults, and `config init` writes those defaults ──
hud config show >"$CFG/default.json" 2>&1 || true
hud config init >/dev/null 2>&1 || true
hud config show >"$CFG/after-init.json" 2>&1 || true
if python3 -c 'import json,sys
a, b = (json.load(open(p)) for p in sys.argv[1:3])
assert a == b, "config init changed the effective config"
assert a["viewer"] == "web" and a["web"]["open"] is True and a["diagram"]["sketch"] is False, a
assert a["ui"] == {"theme": "auto", "language": "en"}, a' "$CFG/default.json" "$CFG/after-init.json" 2>"$CFG/cfg.err"; then
    ok "config defaults survive config init"
else
    bad "config defaults  ($CFG/cfg.err)"
fi

# ── keymap: config.keymap overrides one key, /api/config serves the merge ──
python3 -c 'import json,sys
p = sys.argv[1]
c = json.load(open(p))
c["keymap"] = {"send": "S", "nonsense": "z"}
json.dump(c, open(p, "w"), indent=2)' "$CFG/hudiff/config.json"
mkdir -p "$CFG/state/hudiff/sessions"
cp -R "$FGS" "$CFG/state/hudiff/sessions/"
hud daemon start >/dev/null 2>&1 || true
KP=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["port"])' "$CFG/state/hudiff/server.json")
curl -s "http://127.0.0.1:$KP/api/config" >"$CFG/api-config.json"
if python3 -c 'import json,sys
k = json.load(open(sys.argv[1]))["keymap"]
assert k["send"] == "S", k          # overridden
assert k["next_hunk"] == "j", k     # default kept
assert "nonsense" not in k, k       # unknown action ignored
assert len(k) == 19, len(k)' "$CFG/api-config.json" 2>"$CFG/km.err"; then
    ok "keymap override merges into /api/config"
else
    bad "keymap merge  ($CFG/api-config.json $CFG/km.err)"
fi

# ── hooks: on_submit runs with the documented environment ──
hud daemon stop >/dev/null 2>&1 || true
python3 -c 'import json,sys
p, out = sys.argv[1], sys.argv[2]
c = json.load(open(p))
c["hooks"] = {"on_submit": "printf %s\\\\n \"$HUDIFF_SESSION|$HUDIFF_URL|$HUDIFF_REPO|$HUDIFF_COUNT\" > " + out,
              "on_review_request": ""}
json.dump(c, open(p, "w"), indent=2)' "$CFG/hudiff/config.json" "$CFG/hook.out"
hud daemon start >/dev/null 2>&1 || true
KP=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["port"])' "$CFG/state/hudiff/server.json")
MSID=$(basename "$FGS")   # not SID: that is parity.sh's fixture, used by restore()
curl -s -o /dev/null -X POST -H 'Content-Type: application/json' \
    -d '{"target":"general","body":"hook probe"}' "http://127.0.0.1:$KP/s/$MSID/api/comments"
curl -s -o /dev/null -X POST -H 'Content-Type: application/json' -d '{}' "http://127.0.0.1:$KP/s/$MSID/api/submit"
for _ in 1 2 3 4 5 6 7 8 9 10; do [ -s "$CFG/hook.out" ] && break; sleep 0.2; done
if [ -s "$CFG/hook.out" ] && grep -q "^$MSID|http://127.0.0.1:$KP/s/$MSID/|.*|[0-9]" "$CFG/hook.out"; then
    ok "hooks.on_submit ran with HUDIFF_SESSION / URL / REPO / COUNT"
else
    bad "hooks.on_submit  (got: $(cat "$CFG/hook.out" 2>/dev/null || echo nothing))"
fi
hud daemon stop >/dev/null 2>&1 || true

# ── skill install: a symlink into the agent's skills dir, inside a temp HOME ──
HOME_T=$(mktemp -d)
if HOME=$HOME_T XDG_DATA_HOME=$HOME_T/share "$GO" skill install --for claude >"$HOME_T/out" 2>&1 &&
   [ -L "$HOME_T/.claude/skills/hudiff" ] &&
   [ -f "$HOME_T/.claude/skills/hudiff/SKILL.md" ]; then
    ok "skill install --for claude links $(readlink "$HOME_T/.claude/skills/hudiff")"
else
    bad "skill install  ($HOME_T/out)"
fi
rm -rf "$HOME_T"

# ── diagram --ascii: readable in a terminal, and it keeps the node labels ──
hud -s "$MSID" diagram auto-map --ascii >"$CFG/auto.txt" 2>"$CFG/ascii.err" || true
hud -s "$MSID" diagram auto-map --src >"$CFG/auto.d2" 2>/dev/null || true
if [ -s "$CFG/auto.txt" ] && python3 "$ROOT/specs/parity-labels.py" "$CFG/auto.txt" "$CFG/auto.d2" 2>>"$CFG/ascii.err"; then
    ok "diagram --ascii keeps every node label ($(wc -l <"$CFG/auto.txt" | tr -d ' ') lines)"
else
    bad "diagram --ascii  ($CFG/auto.txt $CFG/ascii.err)"
fi

# ── the UI is English: no CJK left in anything we ship ──
# Chinese left in anything a user sees. internal/review/review.go's priority
# alias table is on purpose — it accepts what an agent might write — and Go
# comments are not user-visible.
if leak=$(grep -rn '[一-鿿]' "$ROOT/web" "$ROOT/skills" "$ROOT/internal" --exclude-dir=vendor 2>/dev/null |
          grep -v 'internal/review/review.go' | grep -v ':[0-9]*:[[:space:]]*//'); then
    bad "CJK left where a user can see it: $leak"
else
    ok "no CJK in web/, skills/ or Go output strings"
fi

# ── archive / rm: the home page buttons and the two subcommands ─────────────
M5_AR=$(mktemp -d)
remember "$M5_AR/state"
mkdir -p "$M5_AR/state/hudiff/sessions"
cp -R "$FGS" "$M5_AR/state/hudiff/sessions/"
arh() { XDG_STATE_HOME=$M5_AR/state "$GO" "$@"; }

arh session archive "$MSID" >/dev/null 2>&1 || true
hidden=$(arh session list 2>/dev/null | grep -c "$MSID" || true)
shown=$(arh session list --all 2>/dev/null | grep -c "$MSID" || true)
if [ "$hidden" = 0 ] && [ "$shown" -ge 1 ]; then
    ok "session archive hides it from list, --all still shows it"
else
    bad "session archive: default listed $hidden, --all listed $shown"
fi
if arh session list --all 2>/dev/null | grep -q '\[archived\]'; then
    ok "archived sessions are marked in the list"
else
    bad "archived marker missing from session list --all"
fi

# the home page's checkbox reads the same flag off /api/sessions
arh daemon start >/dev/null 2>&1 || true
M5_AP=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["port"])' "$M5_AR/state/hudiff/server.json")
if curl -s "http://127.0.0.1:$M5_AP/api/sessions" |
   python3 -c 'import json,sys; assert json.load(sys.stdin)["sessions"][0]["archived"] is True' 2>/dev/null; then
    ok "/api/sessions reports the session as archived"
else
    bad "/api/sessions did not report archived"
fi
curl -s -o /dev/null -X PATCH -H 'Content-Type: application/json' -d '{"archived":false}' \
    "http://127.0.0.1:$M5_AP/api/s/$MSID"
if arh session list 2>/dev/null | grep -q "$MSID"; then
    ok "PATCH /api/s/{id} unarchives it (the page's button)"
else
    bad "PATCH /api/s/{id} did not unarchive"
fi
curl -s -o /dev/null -X DELETE "http://127.0.0.1:$M5_AP/api/s/$MSID"
if [ ! -d "$M5_AR/state/hudiff/sessions/$MSID" ]; then
    ok "DELETE /api/s/{id} removes the session directory"
else
    bad "DELETE /api/s/{id} left the directory behind"
fi
arh daemon stop >/dev/null 2>&1 || true

# and the CLI path, on a second copy
cp -R "$FGS" "$M5_AR/state/hudiff/sessions/" 2>/dev/null || true
arh session rm "$MSID" >/dev/null 2>&1 || true
if [ ! -d "$M5_AR/state/hudiff/sessions/$MSID" ]; then
    ok "session rm removes the session directory"
else
    bad "session rm left the directory behind"
fi
