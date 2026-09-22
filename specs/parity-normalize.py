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
    parity-normalize.py comments <file> hudiff comments / hudiff wait
    parity-normalize.py context <file>  hudiff context / hudiff ctx
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


# comments: status words, the side word, the reply author label and the two
# headers are labels. Ids, file, hunk, line, the quoted source and every body
# line are data and are compared as they are. Session ids differ by definition.
STATUS = {"not sent": "OPEN", "未发送": "OPEN", "pending": "SENT", "待处理": "SENT",
          "replied": "REPLIED", "已回复": "REPLIED", "resolved": "RESOLVED", "已解决": "RESOLVED"}
SIDE = {"new": "NEW", "新版": "NEW", "old": "OLD", "旧版": "OLD"}
WHO = {"agent": "AGENT", "me": "USER", "我": "USER", "me (new)": "USER-NEW", "我(新)": "USER-NEW"}

C_HEAD = re.compile(r"^# (?:session|会话) \S+: (.*?)\s+(?:(\d+) total, showing (\d+)|共 (\d+) 条, 显示 (\d+) 条)$")
C_ROW = re.compile(r"^\[(\w+)\] (\S+(?: \(new\))?)  (.*)$")
C_REF = re.compile(r"^(\S+)  (\S+) L(\d+)  \(hunk (\d+)\)$")
C_REPLY = re.compile(r"^    (agent|me \(new\)|me|我\(新\)|我): (.*)$")
C_WAIT = [(re.compile(r"^# (?:the user sent (\d+) comments|用户发来 (\d+) 条评论)$"), "<SENT>"),
          (re.compile(r"^# (?:(\d+) comments were already pending.*|已有 (\d+) 条待处理评论.*)$"), "<ALREADY>")]


def norm_comments(line):
    if line in ("(no comments)", "(没有评论)"):
        return "<NONE>"
    for rx, tag in C_WAIT:
        m = rx.match(line)
        if m:
            return f"{tag} {m.group(1) or m.group(2)}"
    m = C_HEAD.match(line)
    if m:
        return f"<HDR> {m.group(1)} {m.group(2) or m.group(4)} {m.group(3) or m.group(5)}"
    m = C_REPLY.match(line)
    if m:
        return f"    {WHO[m.group(1)]}: {m.group(2)}"
    m = C_ROW.match(line)
    if m:
        cid, status, rest = m.groups()
        return f"[{cid}] {STATUS.get(status, status)} {norm_ref(rest)}"
    return line


FLAGS = [(re.compile(r"\s*⚠ (?:行内容已变.*|the line's content changed.*?)(?=\s*✔|$)"), " <STALE>"),
         (re.compile(r"\s*✔ (?:agent 已处理.*|handled by the agent.*)$"), " <AGENT-DONE>")]


def norm_ref(rest):
    flags = ""
    for rx, tag in FLAGS:
        rest, n = rx.subn("", rest)
        if n:
            flags += tag
    rest = rest.strip()
    if rest in ("overall", "整体意见"):
        return "<OVERALL>" + flags
    m = C_REF.match(rest)
    if m:
        return f"{m.group(1)} {SIDE.get(m.group(2), m.group(2))} L{m.group(3)} hunk {m.group(4)}{flags}"
    return rest.replace("整段 hunk ", "whole hunk ") + flags


# context: the section labels and the canned remarks are labels; paths, symbol
# names, hit counts, git log lines and every reference line are data.
CTX_LABELS = [
    (re.compile(r"^# (?:hudiff session|hudiff 会话) \S+: (?:repo-level context|仓库级上下文)  \((.*)\)$"), r"<CTX-HDR> \1"),
    (re.compile(r"^# (?:Read this before writing notes|用法: 写说明前先看这份).*$"), "<CTX-HOWTO>"),
    (re.compile(r"^(?:recent history|近史):$"), "<HIST>"),
    (re.compile(r"^(?:symbols|符号): \((?:no symbol rules.*|这种文件类型没有符号规则.*)\)$"), "<NO-SYMS>"),
    (re.compile(r"^(?:symbols|符号): (.*)$"), r"<SYMS> \1"),
    (re.compile(r"^(?:references \(outside the changed files\)|引用\(改动文件之外\)):$"), "<REFS>"),
    (re.compile(r"^  (\S+): (\d+) (?:hits, too common a name to list|处, 名字太泛, 不列)\(git grep -nw (\S+)\)$"), r"  \1: \2 <NOISY> \3"),
    (re.compile(r"^      … (?:(\d+) more|还有 (\d+) 处)$"), r"      <MORE> \1\2"),
    (re.compile(r"^(?:related tests|相关测试): (?:none found|没找到)  ⚠ .*$"), "<NO-TESTS>"),
]
CTX_ZERO = re.compile(r"^  (\S+): 0  \((?:removed, nothing else referenced it, clean|已删除, 别处也没引用, 干净)\)$")
CTX_ZERO_NEW = re.compile(r"^  (\S+): 0  \((?:new symbol, no callers yet|新符号, 还没人用)\)$")
CTX_ZERO_NONE = re.compile(r"^  (\S+): 0  \((?:nothing else in the repo references it|仓库里没有别处引用)\)$")
CTX_HITS = re.compile(r"^  (\S+): (\d+) (?:hits|处)(.*)$")
CTX_EXTRA = [(re.compile(r"(?:(\d+) of them in other files this change touches|其中 (\d+) 处在本次改动的其它文件里)"), r"<IN-CHANGED>\1\2"),
             (re.compile(r"(?:(\d+) in tests|(\d+) 处在测试里)"), r"<IN-TESTS>\1\2"),
             (re.compile(r"⚠ (?:the symbol was removed but is still referenced|符号被删了但别处还在引用)"), "<DANGER>")]
CTX_TESTS = re.compile(r"^(?:related tests|相关测试): (.*?)(?:   \((?:none of them changed here|本次没动任何测试)\)|   \((?:(\d+) of them changed here|本次改了其中 (\d+) 个)\))$")


def norm_context(line):
    for rx, tag in CTX_LABELS:
        m = rx.match(line)
        if m:
            return rx.sub(tag, line)
    for rx, tag in ((CTX_ZERO, "<ZERO-REMOVED>"), (CTX_ZERO_NEW, "<ZERO-NEW>"), (CTX_ZERO_NONE, "<ZERO>")):
        m = rx.match(line)
        if m:
            return f"  {m.group(1)}: 0 {tag}"
    m = CTX_TESTS.match(line)
    if m:
        n = m.group(2) or m.group(3) or "0"
        return f"<TESTS> {m.group(1)} changed={n}"
    m = CTX_HITS.match(line)
    if m:
        extra = m.group(3)
        for rx, tag in CTX_EXTRA:
            extra = rx.sub(tag, extra)
        return f"  {m.group(1)}: {m.group(2)} hits{extra}"
    return line


def main():
    mode, path = sys.argv[1], sys.argv[2]
    if mode == "body":
        sys.stdout.write(norm_body(path))
        return
    fn = {"read": norm_read, "list": norm_list, "comments": norm_comments, "context": norm_context}[mode]
    with open(path) as fh:
        for line in fh:
            print(fn(line.rstrip("\n")))


if __name__ == "__main__":
    main()
