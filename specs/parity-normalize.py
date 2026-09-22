#!/usr/bin/env python3
"""Translate the Python oracle's fixed output phrases into the exact English the
Go build prints, so the two can be diffed strictly.

The mapping runs on the ORACLE side only. The Go side is never loosened: if its
wording drifts from what is written here, the diff fails, which is the point.
Only fixed phrases are listed — paths, ids, counts, titles and diff bodies are
data and are compared as they are.

    parity-normalize.py <read|list|comments|context|md> <file>  # oracle text in
    parity-normalize.py structure <file>                      # a --dump-dom capture
"""
import re
import sys

READ = [
    (r'^# hudiff 会话 (\S+): (.*)  \[(\w+)\]$', r'# hudiff session \1: \2  [\3]'),
    (r'^# 每个 hunk 标着 .*$',
     '# Every hunk is tagged [path#n]; use n as the key in notes.json. The two number columns are old | new.'),
    (r'^# 序号后的标记: .*$',
     '# Mark after the number: ✓ noted  · explicitly needs no note (null)  blank = not written yet'),
    (r'^# 行首 ~ 的是补充上下文.*$',
     '# Lines starting with ~ are extra context from the new file, not part of the diff; they only show the enclosing block.'),
    (r'^## 审查要求.*$', '## Review request (from the user; write the notes to match)'),
    (r'^### 预设 ', '### Preset '),
    (r'^### 本次$', '### This time'),
    (r'^### 上下文文件\(先读\)$', '### Context files (read these first)'),
    (r'^(=== \S+) \(原 (.+)\)(  \[)', r'\1 (was \2)\3'),
    (r'^    \(取不到新版全文, 只能看 diff 自带的上下文\)$',
     "    (cannot read the new file; only the diff's own context is shown)"),
]

LIST = [
    (r'^server: ● ', 'daemon: ● '),
    (r'^server: ○ .*$', 'daemon: ○ not running (any subcommand starts it)'),
    (r'^\(没有会话\)$', '(no sessions)'),
    (r'^\(另有 (\d+) 个已归档, --all 查看\)$', r'(\1 more archived; --all to see them)'),
    (r'(\d+) 条评论', r'\1 comments'),
    (r'(\d+) 条待处理', r'\1 pending'),
    (r'\[已归档\]', '[archived]'),
]

# Both sides build their own fixture session, so the id differs by construction,
# and the default session title is intentionally English on our side (spec §9).
IDENT = [
    (r'\b\d{8}-\d{6}-[0-9a-f]{4}\b', '<SESSION-ID>'),
    (r'(工作区未提交的改动|Uncommitted changes)', '<DEFAULT-TITLE>'),
]

COMMENTS = [
    (r'^# 会话 (\S+): (.*)  共 (\d+) 条, 显示 (\d+) 条$', r'# session \1: \2  \3 total, showing \4'),
    (r'^# 用户发来 (\d+) 条评论$', r'# the user sent \1 comments'),
    (r'^# 已有 (\d+) 条待处理评论\(用户之前就点过发送\)$',
     r'# \1 comments were already pending (the user hit Send earlier)'),
    (r'^\(没有评论\)$', '(no comments)'),
    (r'^\[(\w+)\] 未发送', r'[\1] not sent'),
    (r'^\[(\w+)\] 待处理', r'[\1] pending'),
    (r'^\[(\w+)\] 已回复', r'[\1] replied'),
    (r'^\[(\w+)\] 已解决', r'[\1] resolved'),
    (r'  新版 L', '  new L'),
    (r'  旧版 L', '  old L'),
    (r'  整段 hunk ', '  whole hunk '),
    (r'  整体意见$', '  overall'),
    (r' ⚠ 行内容已变, 位置可能不准', " ⚠ the line's content changed, the position may be off"),
    (r'  ✔ agent 已处理, 待用户确认', '  ✔ handled by the agent, awaiting the user'),
    (r'^    我\(新\): ', '    me (new): '),
    (r'^    我: ', '    me: '),
    (r'^         ', '    '),  # the oracle pads a continuation line to len("我")+1
]

CONTEXT = [
    (r'^# hudiff 会话 (\S+): 仓库级上下文  \((.*)\)$', r'# hudiff session \1: repo-level context  (\2)'),
    (r'^# 用法: 写说明前先看这份.*$',
     '# Read this before writing notes: callers tell you the blast radius, recent history is often the reason for the change, tests tell you whether it is covered.'),
    (r'^近史:$', 'recent history:'),
    (r'^符号: \(这种文件类型没有符号规则, 跳过\)$', 'symbols: (no symbol rules for this file type, skipped)'),
    (r'^符号: ', 'symbols: '),
    (r'^引用\(改动文件之外\):$', 'references (outside the changed files):'),
    (r'^  (\S+): (\d+) 处, 名字太泛, 不列\(要看就 git grep -nw (\S+)\)$',
     r'  \1: \2 hits, too common a name to list (git grep -nw \3)'),
    (r'  \(已删除, 别处也没引用, 干净\)$', '  (removed, nothing else referenced it, clean)'),
    (r'  \(新符号, 还没人用\)$', '  (new symbol, no callers yet)'),
    (r'  \(仓库里没有别处引用\)$', '  (nothing else in the repo references it)'),
    (r'^  (\S+): (\d+) 处', r'  \1: \2 hits'),
    (r'其中 (\d+) 处在本次改动的其它文件里', r'\1 of them in other files this change touches'),
    (r'(\d+) 处在测试里', r'\1 in tests'),
    (r'⚠ 符号被删了但别处还在引用', '⚠ the symbol was removed but is still referenced'),
    (r'^      … 还有 (\d+) 处$', r'      … \1 more'),
    (r'^相关测试: 没找到  ⚠ .*$',
     'related tests: none found  ⚠ this change has no test coverage (or the tests are named unusually)'),
    (r'^相关测试: ', 'related tests: '),
    (r'   \(本次没动任何测试\)$', '   (none of them changed here)'),
    (r'   \(本次改了其中 (\d+) 个\)$', r'   (\1 of them changed here)'),
]

MD = [
    (r'^## 总体评价$', '## Overall'),
    (r'^## 优点$', '## Strengths'),
    (r'^- \(无\)$', '- (none)'),
    (r'^## 待处理项$', '## Items'),
    (r'^### \[高优\]$', '### [High]'),
    (r'^### \[中优\]$', '### [Medium]'),
    (r'^### \[低优\]$', '### [Low]'),
    (r'^  \*\*修改建议\*\*: ', '  **Suggested fix**: '),
    (r'^  ✔ 处理结果\((.*)\): ', r'  ✔ resolution (\1): '),
    (r'^  - (.) 复核\((.*), 第 (\S+) 轮\): ', r'  - \1 re-review (\2, round \3): '),
]

TABLES = {"read": READ, "list": LIST, "comments": COMMENTS, "context": CONTEXT, "md": MD}

# Modes where the session id and the default title have to be masked on both
# sides: the two fixtures are different sessions and the title is translated.
MASKED = {"comments", "context"}

# structure: the rendered page's tag / id / class skeleton. Text is dropped —
# the Go page is English and the oracle's is Chinese by design — so what this
# compares is that both render the same elements in the same order with the same
# classes. A logic difference shows up here; a translation does not.
BODY_ASSETS = re.compile(r"<style>.*?</style>|<script[^>]*>.*?</script>", re.S)
TAG = re.compile(r'<(/?)([a-zA-Z][-\w]*)([^>]*)>')
KEEP = re.compile(r'\b(id|class|data-\w+)="([^"]*)"')


def skeleton(html):
    body = BODY_ASSETS.sub("", html[html.index("<body"):])
    out = []
    for m in TAG.finditer(body):
        close, tag, attrs = m.groups()
        if close:
            out.append(f"</{tag}>")
            continue
        kept = " ".join(f'{k}="{v}"' for k, v in KEEP.findall(attrs))
        out.append(f"<{tag}{' ' + kept if kept else ''}>")
    return "\n".join(out) + "\n"


def main():
    mode, path = sys.argv[1], sys.argv[2]
    if mode == "structure":
        sys.stdout.write(skeleton(open(path).read()))
        return
    rules = [(re.compile(p), r) for p, r in TABLES[mode]]
    masks = [(re.compile(p), r) for p, r in IDENT] if mode in MASKED else []
    oracle = "--oracle" in sys.argv
    with open(path) as fh:
        for line in fh:
            line = line.rstrip("\n")
            if oracle:
                for rx, rep in rules:
                    line = rx.sub(rep, line)
            for rx, rep in masks:
                line = rx.sub(rep, line)
            print(line)


if __name__ == "__main__":
    main()
