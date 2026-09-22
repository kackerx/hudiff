#!/usr/bin/env python3
"""Collapse the wordings that are allowed to differ between the Go build and
the Python oracle, so `diff` still catches everything else.

The Go CLI is English (spec §3) while the oracle is Chinese, and the page UI
stays Chinese until M5. Rather than dropping those lines, each allowance maps
both wordings to the same token: a line that goes missing, moves, or carries
different data still shows up as a difference.

    parity-normalize.py read <file>     hudiff read / hudiff show
    parity-normalize.py list <file>     hudiff session list / hudiff ls
    parity-normalize.py body <file>     a --dump-dom capture of a page
"""
import re
import sys

# read: `#`/`##`/`###` lines are the agent-facing header and the review-request
# section. Everything below them — `===` file lines, `--- [path#n]` hunk tags,
# the two line-number columns — must still match byte for byte.
HDR = re.compile(r"^#{1,3} ")
RENAME = re.compile(r"^(=== \S+) \((?:原 |was )(.+)\)(  \[)")
NO_NEW_TEXT = re.compile(r"^    \((?:取不到新版全文, 只能看 diff 自带的上下文"
                         r"|cannot read the new file; only the diff's own context is shown)\)$")

# list: one session per line, `<mark> <id>  [<kind>] <title padded to 40>  <tail>`.
# The tail's counts, archived flag and repo are data; the words around them are
# labels ("条评论" / "comments").
ROW = re.compile(r"^([ *]) (\S+)  \[([^\]]+)\] (.{40})  (.*)$")
SERVER_UP = re.compile(r"^(?:server|daemon): ● (\S+)$")
SERVER_DOWN = re.compile(r"^(?:server|daemon): ○ ")
EMPTY = re.compile(r"^\((?:没有会话|no sessions)\)$")
HIDDEN = re.compile(r"^\((?:另有 (\d+) 个已归档.*|(\d+) more archived.*)\)$")


def norm_read(line):
    if HDR.match(line):
        return "<HDR>"
    line = RENAME.sub(r"\1 (REN \2)\3", line)
    if NO_NEW_TEXT.match(line):
        return "    <NO-NEW-TEXT>"
    return line


def norm_list(line):
    m = SERVER_UP.match(line)
    if m:
        return f"<DAEMON-UP> {m.group(1)}"
    if SERVER_DOWN.match(line):
        return "<DAEMON-DOWN>"
    if EMPTY.match(line):
        return "<NO-SESSIONS>"
    m = HIDDEN.match(line)
    if m:
        return f"<HIDDEN> {m.group(1) or m.group(2)}"
    m = ROW.match(line)
    if not m:
        return line
    mark, sid, kind, title, tail = m.groups()
    parts = tail.rsplit(None, 1)
    body, repo = (parts[0], parts[1]) if len(parts) == 2 else ("", tail)
    counts = " ".join(re.findall(r"\d+", body))
    archived = "archived" if "[" in body else "-"
    return f"{mark} {sid} [{kind}] {title!r} counts=[{counts}] {archived} repo={repo}"


# body: the rendered <body> is what the page actually shows and is identical on
# both sides. The <head> is not: M1-fix moved the CSS and JS out of the HTML,
# and the oracle still links highlight.js from the CDN. Inline <style>/<script>
# go too, for the same reason.
BODY_ASSETS = re.compile(r"<style>.*?</style>|<script[^>]*>.*?</script>", re.S)


def norm_body(path):
    html = open(path).read()
    return BODY_ASSETS.sub("", html[html.index("<body"):])


def main():
    mode, path = sys.argv[1], sys.argv[2]
    if mode == "body":
        sys.stdout.write(norm_body(path))
        return
    fn = {"read": norm_read, "list": norm_list}[mode]
    with open(path) as fh:
        for line in fh:
            print(fn(line.rstrip("\n")))


if __name__ == "__main__":
    main()
