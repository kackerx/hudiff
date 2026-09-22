#!/bin/sh
# M5 half of the parity gate: config, keymap, hooks, skill install, --ascii.
# These are ours alone — the oracle has no config file, no keymap and no hooks —
# so they are assertions about the Go build rather than comparisons.
#
# Sourced by parity.sh; uses ok/bad/$WORK/$ROOT/$GO/$FGS.
set -eu

CFG=$(mktemp -d)
remember "$CFG"

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
SID=$(basename "$FGS")
curl -s -o /dev/null -X POST -H 'Content-Type: application/json' \
    -d '{"target":"general","body":"hook probe"}' "http://127.0.0.1:$KP/s/$SID/api/comments"
curl -s -o /dev/null -X POST -H 'Content-Type: application/json' -d '{}' "http://127.0.0.1:$KP/s/$SID/api/submit"
for _ in 1 2 3 4 5 6 7 8 9 10; do [ -s "$CFG/hook.out" ] && break; sleep 0.2; done
if [ -s "$CFG/hook.out" ] && grep -q "^$SID|http://127.0.0.1:$KP/s/$SID/|.*|[0-9]" "$CFG/hook.out"; then
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
hud -s "$SID" diagram auto-map --ascii >"$CFG/auto.txt" 2>"$CFG/ascii.err" || true
hud -s "$SID" diagram auto-map --src >"$CFG/auto.d2" 2>/dev/null || true
if [ -s "$CFG/auto.txt" ] && python3 "$ROOT/specs/parity-labels.py" "$CFG/auto.txt" "$CFG/auto.d2" 2>>"$CFG/ascii.err"; then
    ok "diagram --ascii keeps every node label ($(wc -l <"$CFG/auto.txt" | tr -d ' ') lines)"
else
    bad "diagram --ascii  ($CFG/auto.txt $CFG/ascii.err)"
fi

# ── the UI is English: no CJK left in anything we ship ──
if leak=$(grep -rl '[一-鿿]' "$ROOT/web" "$ROOT/skills" --exclude-dir=vendor 2>/dev/null); then
    bad "CJK left in shipped assets: $leak"
else
    ok "no CJK left in web/ or skills/"
fi
