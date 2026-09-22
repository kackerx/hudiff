#!/usr/bin/env python3
"""Normalise a captured output so it can be diffed against a frozen golden file.

Everything that changes from run to run — session ids, ports, pids, mktemp
paths, wall-clock timestamps, git hashes, $HOME — is replaced by a placeholder.
Nothing else is touched: wording, field names, counts, paths inside the fixture
repo and diff bodies are compared exactly, which is the point of the gate.

    parity-normalize.py mask <file>        # text captured from the CLI
    parity-normalize.py json <file>        # JSON: canonicalised, then masked
    parity-normalize.py structure <file>   # a --dump-dom capture: tag/id/class skeleton
"""
import json
import os
import re
import sys

MASK = [
    (r'\b\d{8}-\d{6}-[0-9a-f]{4}\b', '<SID>'),
    (r'\b[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}\b', '<UUID>'),
    (r'(?:/private)?/var/folders/[^\s"\',]*?/T/tmp\.[A-Za-z0-9]+', '<TMP>'),
    (r'/tmp/tmp\.[A-Za-z0-9]+', '<TMP>'),
    (r'127\.0\.0\.1:\d+', '127.0.0.1:<PORT>'),
    (r'"(port|pid|ppid)": ?\d+', r'"\1": <N>'),
    (r'\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:[+-]\d{2}:\d{2}|Z)', '<TS>'),
    # a git hash, i.e. hex with at least one digit — never a word like "decade"
    (r'\b(?=[0-9a-f]*\d)[0-9a-f]{7,40}\b', '<SHA>'),
    (re.escape(os.path.expanduser("~")), '<HOME>'),
]
RULES = [(re.compile(p), r) for p, r in MASK]


def mask(text):
    for rx, rep in RULES:
        text = rx.sub(rep, text)
    return text


# structure: the rendered page's tag / id / class skeleton. Text is dropped, so
# what this compares is that the page renders the same elements in the same
# order with the same classes — a logic change shows up, a reworded label does
# not (those are covered by the CLI goldens).
BODY_ASSETS = re.compile(r"<style>.*?</style>|<script[^>]*>.*?</script>", re.S)
TAG = re.compile(r'<(/?)([a-zA-Z][-\w]*)([^>]*)>')
KEEP = re.compile(r'\b(id|class|data-\w+)="([^"]*)"')


# Which diagram card is "active" depends on where the IntersectionObserver
# happened to be when the page was captured — view state, not structure.
def drop_active(key, value):
    if key != "class" or not {"dg", "dgcard"} & set(value.split()):
        return value
    return " ".join(t for t in value.split() if t != "active")


def skeleton(html):
    body = BODY_ASSETS.sub("", html[html.index("<body"):])
    out = []
    for m in TAG.finditer(body):
        close, tag, attrs = m.groups()
        if close:
            out.append(f"</{tag}>")
            continue
        kept = " ".join(f'{k}="{drop_active(k, v)}"' for k, v in KEEP.findall(attrs))
        out.append(f"<{tag}{' ' + kept if kept else ''}>")
    return "\n".join(out) + "\n"


def main():
    mode, path = sys.argv[1], sys.argv[2]
    raw = open(path).read()
    if mode == "structure":
        out = skeleton(raw)
    elif mode == "json":
        out = json.dumps(json.loads(raw), sort_keys=True, indent=1, ensure_ascii=False) + "\n"
    else:
        out = raw
    sys.stdout.write(mask(out))


if __name__ == "__main__":
    main()
