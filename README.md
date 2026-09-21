<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/logo-dark.svg">
    <img src="assets/logo.svg" alt="hudiff — diffs for humans" width="300">
  </picture>
</p>

<p align="center"><b>hudiff</b> — diffs for humans. 让 agent 讲解一份 diff,人在浏览器里逐行评论,评论回流给 agent。</p>

---

## 这是什么

你让 agent(Claude Code / pi / opencode / codex …)改了一堆代码,或者要 review 一个 PR。
`git diff` 只告诉你**改了什么**,不告诉你**为什么改、影响哪里、哪里有风险**。

hudiff 起一个本地页面,像 GitHub PR 一样看 diff,但多了三样东西,全部由 agent 填进来:

| tab | 内容 | 谁写 |
|---|---|---|
| **代码** | diff + 每个 hunk 的讲解(review 视角:为什么改 / 影响面 / 风险;平凡改动不废话) | 讲解 agent |
| **图解** | d2 画的改动地图(文件 → 改动的符号 → 外部引用方,自动生成)+ agent 补的流程/时序图 | 自动 + agent |
| **审查** | 第二个 agent 的交叉审查报告:总评 / 优点 / 高中低待处理项,多轮复审并入,不覆盖你打过的钩 | 审查 agent |

你在任何一行、任何一段、或整体上写评论,点「发给 agent」,正在 `hudiff wait` 的 agent 立刻收到,
逐条回复或直接改代码;工作区改了之后 `hudiff refresh`,评论按行内容重新对位。

一个 server 进程管所有会话,URL 按会话 id,会话数据留在 `~/.local/state/hudiff/`,随时重放、归档、删除。

## 安装

零依赖,单文件 Python 3(标准库),外加:

```bash
git clone <this repo> ~/proj/hudiff
ln -s ~/proj/hudiff/hudiff ~/.local/bin/hudiff     # 或任何在 PATH 里的目录
brew install d2                                     # 可选: 图解 tab 需要; gh CLI 用于 PR
```

给 agent 装 skill(把 `skills/hudiff` 链到你的 agent 读 skill 的目录):

```bash
ln -s ~/proj/hudiff/skills/hudiff ~/.claude/skills/hudiff       # Claude Code
ln -s ~/proj/hudiff/skills/hudiff ~/.pi/agent/skills/hudiff     # pi
# opencode / codex: 放到各自的 skills 目录, SKILL.md 格式通用
```

## 用法

```bash
hudiff open                     # 工作区 vs HEAD(含已暂存)
hudiff open --staged            # 只看已暂存
hudiff open HEAD~1              # 某个 commit
hudiff open a1b2c3..HEAD        # 区间
hudiff open pr 562              # GitHub PR(也接受 #562 / URL, 走 gh pr diff)
hudiff open --patch x.patch     # 任意补丁('-' 读 stdin)
hudiff open -p "重点看并发和错误处理"   # 带审查要求, 原文给 agent
```

`open` 是幂等的:同一个目标复用已有会话,工作区内容变了原地 refresh;`--new` 强制新建。

在 agent 里直接说 `/hudiff <目标>`,它会 `open` → `show` 读 diff → 写讲解 → `wait` 等你评论。
另开一个 agent 说 `/hudiff review <会话id>`,它去做交叉审查。

agent 侧的完整命令(`hudiff -h`):

```
show / ctx        读带 hunk 标记的 diff; 仓库级上下文(文件近史 / 改了哪些符号 / 谁在引用 / 相关测试)
notes             写讲解(JSON, schema 见 skill; 平凡 hunk 写 null)
diagram           列出 / 看 d2 源码 / 渲染
review set|done|wontfix|note|md    交叉审查报告
comments / wait / reply / resolve  评论回流
info / refresh / archive / rm / ls / url / stop
```

## 设计要点

- **CLI + skill 两件套**。CLI 是产品本体,任何能跑 shell 的 agent 读 `hudiff -h` 就会用;`skills/hudiff/SKILL.md` 是给 agent 的说明书(流程 A 讲解 / B 处理评论 / C 交叉审查)。
- **server 是评论的唯一写者**,agent 走 HTTP;**会话即收件箱**,「发给 agent」不发给进程,谁在这个会话上 `wait` 谁被叫醒。
- **agent 不能替你关评论**:它的 `resolve` 只打「已处理,待确认」标记。
- **审查报告是并入不是覆盖**:第二轮复审保留你已打钩/拒绝的项,只追加复核说明。
- 页面零依赖(highlight.js 走 CDN),5k+ 行 diff 用 `content-visibility` 保持滚动流畅。

## 环境变量

| 变量 | 作用 |
|---|---|
| `HUDIFF_PORT` | server 端口(否则沿用上次,首次随机) |
| `HUDIFF_PROMPTS` | 预设审查要求目录,默认 `$XDG_CONFIG_HOME/hudiff`,里面放 `default.md` / `<仓库目录名>.md` |
| `XDG_STATE_HOME` | 会话数据位置,默认 `~/.local/state/hudiff` |

## 状态

个人在用,准备开源。待做:reviewer 的行内评论进评论线程、评论 `author` 字段、Linux 验证、拆文件。
