# hudiff v1 (Go 重写) —— 技术选型与架构规格

状态: **定稿, 2026-09-21**(§11 六项已拍板), 进入 M1。
上一版(Python 单文件 `hudiff`, 3595 行)是本规格的 oracle: 行为以它为准, 除非本文明确说改。

## 0. 一句话

hudiff = diffs for humans。一个本地工具, 让 agent 讲解一份 diff(讲解 / 图解 / 交叉审查), 人在浏览器里像
GitHub PR 一样逐行评论, 评论回流给 agent。**面向所有能跑 shell 的 agent**(Claude Code / pi / opencode /
codex…)和所有平台(macOS / Linux / Windows 尽力), 单二进制分发。

## 1. 目标与非目标

目标
- 单二进制, 零运行时依赖(可选外部工具: `git` 必需, `gh` 看 PR 时, `d2` 选 d2 引擎时)
- 页面 HTML/CSS/JS 是真文件(`web/`), embed 进二进制; 开发时可从磁盘热读
- CLI 表面现代、统一(参考 `hunk`), `hudiff -h` 就是 agent 的说明书
- 不绑定任何终端复用器(tmux / ccmux 全部去掉), 用户级"通知"走可配置的 shell hook
- 图解引擎可配(d2 / mermaid)
- UI 文案、CLI 输出、SKILL.md 全英文
- 给未来的 TUI 留口子: 核心不感知"怎么看", 查看器只是 daemon API 的客户端

非目标(v1 不做)
- TUI 本体(只留 `--tui` 和 `viewer` 配置位, 选了报 "not implemented yet")
- 多用户 / 远程访问 / 鉴权(只监听 127.0.0.1)
- WebSocket / SSE(5 秒轮询 + `/api/wait` 长轮询够用)
- sqlite(磁盘 JSON 是 agent 可直接读的契约)
- 非 git 的 VCS

## 2. 技术选型(已定)

| 项 | 选择 | 理由 / 否掉的 |
|---|---|---|
| 语言 | Go, `go 1.25`(本机 1.26.3) | 单二进制、embed、维护者更熟 |
| module | `github.com/kackerx/hudiff` | 个人号; 根目录就是 `package main`, `go install …@latest` 直接可用 |
| CLI | **kong** v1.16(唯一第三方依赖) | 18 子命令 / 47 参数 / 两层子命令 / 可重复 flag / flag 可在位置参数后 —— 标准库 `flag` 全要手写, 且 `-h` 质量差 |
| HTTP | 标准库 `net/http`, Go 1.22 mux 模式(`GET /s/{sid}/api/session`) | 十几条路由, 框架无收益 |
| 前端 | 保留现有原生 JS + JSON API, 拆成文件 | **否掉 htmx**: 页面是重客户端状态(5k 行表 + content-visibility + IntersectionObserver 判 hunk + 拖栏 + localStorage + 客户端 hljs); htmx 要重写我们想原样保留的 60% 代码, 且 agent 走的就是同一套 JSON API |
| 资源 | `//go:embed web skills` | `HUDIFF_DEV=1` 时改为每次请求读磁盘 |
| 图解 | d2(server 侧二进制)/ mermaid(浏览器侧 mermaid.js) | 见 §7 |
| 高亮 | 浏览器侧 highlight.js(CDN) | 不引 chroma; 页面离线时退化为纯文本 |
| 分发 | goreleaser → GitHub Releases(darwin/linux/windows × arm64/amd64)+ homebrew tap; `go install` | 版本号 `-ldflags -X main.version` |

## 3. CLI 表面(v1)

设计原则: **人用的动词开路径, agent 用的动词操作会话**; 全局 `-s/--session <id>` 默认最近一次, 可放任何位置;
flag 与位置参数可交错(kong 默认)。

```
hudiff [--staged|--unstaged] [-- <pathspec>...]  【默认, 无动词】工作区 vs HEAD(含已暂存, 含未跟踪)
hudiff commit [<rev>]                             单个 commit(默认 HEAD)与首个父提交比; merge 取 first-parent
hudiff range <a>..<b> | <a>...<b> | <a>           区间; 只给 <a> = 工作区 vs <a>
hudiff pr <n|#n|url>                              GitHub PR(gh pr diff / gh pr view)
hudiff patch [<file>|-]                           补丁文件 / stdin
hudiff open [<session-id|prefix>]                 用查看器重开已有会话(默认最近一次)

  kong 实现提示: 根命令无子命令时落到 `workspace` 子命令(`cmd:"" default:"withargs"`), 它挂 --staged/--unstaged
  和 `-- pathspec`; 这样 `hudiff --staged` 和 `hudiff -p "…"` 都不用写动词。

  Review options(workspace / commit / range / pr / patch 共用):
  --web | --tui                 查看器覆盖(默认 config.viewer; v1 里 --tui 报 not implemented)
  --no-open                     只建/刷新会话并打印 id, 不开查看器
  --json                        打印会话信息 JSON(隐含 --no-open)—— agent 走这条
  --new                         不复用同 source key 的已有会话
  -p, --prompt <text|->         审查要求, 原文给 agent
  -c, --context <file>          给 agent 的上下文文件, 可重复
  --diagram <d2|mermaid>        覆盖 config.diagram.engine

hudiff session list [--all] | info [--json] | url | refresh | archive [--undo] | rm <id>...

hudiff read    [--file <path>] [-W] [-U <n>]     带 [file#hunk] 标记和双列行号的 diff —— agent 读这个写讲解(原 show)
hudiff context [--file <path>]                   仓库级上下文: 文件近史 / 改了哪些符号 / 谁在引用 / 相关测试(原 ctx)
hudiff notes   <file|->                          写讲解(校验 schema; hunk 写 null = 无需说明; 图源按引擎编译校验)
hudiff diagram [list|<id>] [--src] [-o <svg>]
hudiff review  [show]                            看报告
hudiff review  set <file|-> [--replace] [--author <name>]      默认【并入】
hudiff review  done|wontfix|reopen <id> [<text>]
hudiff review  note <id> <text>
hudiff review  md
hudiff comments [--new] [--json]
hudiff wait    [--timeout <sec>]                 阻塞到用户点 Send, 打印新评论; 超时 exit 124
hudiff reply   <id> <text|->
hudiff resolve <id> [--reopen]                   只打「resolved by agent, awaiting confirmation」, 不关评论

hudiff daemon start|stop|status                  status 打印 URL / 端口 / pid; 内部前台入口 `daemon serve`
hudiff skill   path | install [--for claude|pi|opencode|codex|all]
hudiff config  path | show | init
hudiff version
```

与 Python 版的**故意变更**(2026-09-21 已拍板): 工作区 diff 不要动词, `hudiff` 裸跑就是; 单 commit 用 `commit`
(不是 hunk 的 `show`, 名词和 `pr`/`patch` 一致); 区间用 `range`; `open` 从"创建"变成"重开"; `show`(agent 读 diff)
改名 `read`; `ctx` → `context`; `ls/url/stop/serve/rm/archive/refresh/info` 收进 `session` / `daemon`。
其余子命令名与 flag 保持。

退出码: 0 成功 / 1 用法或校验错 / 2 环境错(不在 git 里、gh 没装、daemon 拉不起)/ 124 wait 超时。
所有子命令 `--json` 时 stdout 只有 JSON, 人类可读的提示走 stderr。

## 4. 配置

路径 `${XDG_CONFIG_HOME:-~/.config}/hudiff/config.json`(XDG 惯例, 目录名**不带点**), 不存在则全默认。

```jsonc
{
  "viewer": "web",                        // web | tui(预留)
  "web":     { "port": 0,                 // 0 = 沿用上次(server.json), 首次随机
               "open": true,              // 建会话后自动开浏览器
               "browser": "" },           // 覆盖打开命令, 空 = 系统默认(open / xdg-open / start)
  "diagram": { "engine": "d2",            // d2 | mermaid
               "d2_bin": "d2" },
  "data_dir": "",                         // 会话数据目录, 空 = ${XDG_STATE_HOME:-~/.local/state}/hudiff
  "prompts_dir": "",                      // 预设审查要求, 空 = <config dir>/prompts/{default,<repo>}.md
  "hooks": {                              // 替代 tmux 投递: shell 命令, 环境变量 HUDIFF_SESSION / HUDIFF_URL / HUDIFF_REPO / HUDIFF_COUNT
    "on_submit": "",                      //   用户点 Send 且【没有 agent 在 wait】时执行
    "on_review_request": "" },            //   用户点 Request review 时执行
  "keymap": { /* 见 §6, 只写要改的键 */ },
  "ui": { "theme": "auto", "language": "en" }
}
```

**会话数据放哪**(已拍板: XDG state): 默认 `~/.local/state/hudiff/`, 因为
① XDG 把 state 和 config 分开就是为了这个; ② kackerx 的 `~/.config` 是跨机器同步的 git 仓库, 会话数据是
单机的、会长大(现 2.3MB)、含私有代码, 不该进去。要放一起就设 `data_dir`。

## 5. 架构

```
hudiff/
  main.go                 kong 入口(package main)
  internal/
    cli/                  kong 结构体 + Run(); 只做参数 → 调核心 → 输出
    config/               加载 / 默认值 / `config path|show|init`
    session/              data_dir 布局, JSON 原子读写(temp+rename), source_key 幂等, archive, 列表
    diff/                 unified diff → files/hunks/lines; hunk 编号必须与 Python 逐字一致(讲解按它对位)
    source/               git worktree/ref/range、commit、gh pr、patch → diff 文本 + meta
    repoctx/              git log / 符号提取 / 引用查找(原 collect_ctx); 不叫 context/ 是为了不遮蔽标准库
    review/               validate(file:line 在 diff 里)+ merge(并入规则, 见 §8.4)
    diagram/              d2 渲染缓存、auto-map 生成(两种方言各一个 emitter)
    daemon/               net/http server, 长轮询 waiters, server.json, 拉起/停止
    viewer/               interface Viewer { Open(sess) }; web/ 实现 = 确保 daemon 在 + 开浏览器; tui/ 占位
    browser/              跨平台 open URL
  web/                    index.html / session.html / dlg.html / favicon.svg(embed)
                          M1 从 Python 常量原样拆出, M5 英文化时再分出 app.css / app.js
  skills/hudiff/SKILL.md  (embed; `skill path` 会把它落到 $XDG_DATA_HOME/hudiff/skill/ 再打印路径)
  legacy/hudiff.py        Python 版 oracle, M6 删
  hudiff                  M1–M5 是跑 legacy 的 /bin/sh shim(会解析软链), M6 换成 go build 产物
  bin/hudiff              go build 输出, .gitignore(M6 之前根目录被 shim 占着)
  specs/                  本文 + handoff
```

关键机制(沿用 Python 版, 只换实现)
- **单 daemon 管所有会话**。任何子命令发现 daemon 不在就 `exec.Command(os.Executable(), "daemon", "serve")`
  + `Setsid`(unix)/ `DETACHED_PROCESS`(windows)拉起, 轮询 `server.json` 最多 3 秒。
- **daemon 是 comments / review 的唯一写者**; CLI 的 `reply/resolve/review set` 走 HTTP; daemon 不在才直接落盘。
  每会话一把 `sync.RWMutex`; 文件写入 temp + rename。
- **会话即收件箱**: 页面 Send 只做 `submit_seq++` 并 close 该会话的 broadcast channel;
  `/api/wait?since=N` 挂在 channel 上, `r.Context()` 处理断开。没人在 wait 时触发 `hooks.on_submit`。
- **TUI 口子**: TUI 将来只是 daemon API 的另一个客户端(轮询 `/api/state`、POST comments、`/api/wait`),
  核心零改动。所以 daemon API 必须是完整的: 页面能做的每件事都有 API。
- **dev 模式**: `HUDIFF_DEV=1` 时 `web/` 从磁盘读(cwd 或 `HUDIFF_WEB_DIR`), 不再有"进程里是老页面"的错位。
- **平台**: agent 发现(ps/lsof)整段删除, 不再需要 build tag; daemon 拉起有 `_unix.go` / `_windows.go`。

## 6. 页面快捷键

内置默认(草案, 具体键位后续再定), 全部可在 `config.keymap` 覆盖(`{"action": "key"}`), 页面 `?` 显示当前表:

| action | 默认 | 说明 |
|---|---|---|
| `next_hunk` / `prev_hunk` | `j` / `k` | 代码 tab 里跳 hunk |
| `next_file` / `prev_file` | `J` / `K` | |
| `next_comment` / `prev_comment` | `n` / `p` | |
| `tab_code` / `tab_diagrams` / `tab_review` | `1` / `2` / `3` | |
| `comment` | `c` | 对当前 hunk 开评论框; 有选区时评论选区 |
| `comment_overall` | `C` | 整体评论 |
| `send` | `s` | 发给 agent |
| `toggle_resolved` | `r` | |
| `filter_files` | `f` | 聚焦文件过滤框 |
| `toggle_nav` / `toggle_aside` | `[` / `]` | 折叠左 / 右栏 |
| `theme` | `t` | auto → light → dark |
| `help` | `?` | |
| `escape` | `Esc` | 关表单 / 弹窗 |

daemon 把合并后的 keymap 放进 `/api/config`, 页面启动时取; 焦点在输入框时不响应单键。

## 7. 图解引擎

- `d2`: daemon 侧调 `d2_bin`, 按 `hash(src, theme)` 缓存 SVG(沿用); `notes` 写入时用 d2 真编译校验。
- `mermaid`: 浏览器侧 mermaid.js(ESM, jsdelivr)渲染, daemon 不需要 node; `notes` 写入时只做结构校验
  (非空、首行是合法图类型), 渲染错误页面显示在图卡片里。
- **自动改动地图**按当前引擎方言生成(`diagram/automap_d2.go` / `automap_mermaid.go`)。
- `notes.json.diagrams[].kind` ∈ {d2, mermaid}(字段已有), 会话建立时记 `session.meta.diagram_engine`,
  `hudiff session info` 暴露出来, SKILL.md 规定 agent 先看它再画。
- 引擎在会话内固定; `--diagram` 只在建会话时生效。

## 8. 数据契约(冻结, 与 Python 版兼容; 现有会话零迁移可打开)

目录 `<data_dir>/sessions/<id>/`, id = `YYYYMMDD-HHMMSS-xxxx`。

1. `session.json`: `id, kind(worktree|commit|range|pr|patch), title, repo, created, meta{…}, key, submit_seq,
   last_submit, refresh, archived, opener{cwd, agent_session}`。**删掉** `bound`(tmux 绑定)。
2. `diff.patch` 原文; `diff.json`: `files[]{path, old_path, status, binary, add, del, hunks[]{header, section,
   old_start, old_count, new_start, new_count, lines[]{t, o, n, s}}}`。
3. `notes.json`: `title, summary, diagrams[]{id, title, kind, caption, hunks[], src}, files{path: {summary,
   hunks{"<n>": {title, body, lines, tags[]} | null}}}`。
4. `comments.json`: `[]{id, target(line|hunk|overall), file, hunk, line, side, snippet, stale, body, status
   (open|sent|replied|resolved), created, replies[]{author, body, created}, **author**}`。
   新增 `author`(默认 `"user"`; agent 回复 `"agent"`; reviewer 行内评论 = 它的 `--author`), 老数据缺省视为 user。
5. `review.json`: `author, updated, summary, strengths[], rounds[]{round, author, at}, items[]{id, round,
   priority, title, file, line, side, hunk, snippet, body, suggestion, status(open|done|wontfix), resolution,
   resolved_by, resolved_at, notes[]{round, author, verdict(ok|disagree|info), note}}`。
   `set` 默认并入: 同 id 同位置更新文字; 同 id 不同位置换 id; 同位置同标题视为同一项; done/wontfix 状态保留;
   `{"id","note","verdict"}` 追加到老项 notes; `--replace` 才整份覆盖。
6. `server.json`(data_dir 根): `port, pid, started`。

HTTP API 前缀 `/s/{sid}/api/`: `session(GET)`, `state`, `wait`, `submit(POST)`, `comments(POST/PATCH/DELETE)`,
`notes(POST)`, `review(GET/POST)`, `review/{id}(PATCH)`, `review/{id}/note(POST)`, `diagram?i=&theme=`,
`refresh(POST)`, `archive(PATCH)`; 根级 `/api/sessions`, `/api/config`, `/favicon.svg`, `/s/{sid}/`, `/`。
**删掉** `/api/agents`、`/api/notify`、`/api/review-request` 的投递部分(改为返回要粘贴的命令 + 触发 hook)。

`hudiff read` 的文本格式逐字保留(`=== path [M] +a -b` / `--- [path#n] ✓ @@…` / 双列行号), 只把中文注释行换成英文。

## 9. 前端改动清单(在拆文件之外)

- 全部文案英文, 字符串收进 `app.js` 顶部一个 `T` 对象(为将来 `ui.language` 留位)
- 删: agent 选择器弹层、绑定状态、"在 tmux 里" 分支; Send 在无 waiter 时提示 `Run: hudiff wait` + 复制按钮;
  Request review 弹出 `/hudiff review <id>` 命令 + 复制按钮(hook 若配置则同时触发)
- 加: keymap 模块(§6)、mermaid 渐进加载、评论 `author` 头像/颜色 + 按作者筛选、首页评论数按作者分
- 保: content-visibility、scrollCodeTo 定位、sash 拖宽、自绘 dlg、tab 独立滚动

## 10. 移植策略与里程碑

冻结四个契约(§8 数据、§3 CLI、§8 HTTP、`read` 文本), 现有 3 个会话是夹具, Python 版是 oracle。
`specs/parity.sh`: 对同一会话跑 `read`(含 `-W` / `-U`)/ `session info --json` / `session list` /
`GET /api/session` / `GET /api/sessions` / 每张图的 SVG(两个主题)/ 冷缓存重渲染 / 页面截图, 全部相等才算过。
两个 daemon 共用 server.json, 所以脚本让它们轮流上: 先抓 Python 侧, 换 Go daemon 再抓一遍, 退出前把
Python daemon 放回去。图解 tab 的截图不比像素 —— 那张 d2 `steps` 图是动画, 每次截到的帧不同, 真正的
对照是上面逐字节比的 SVG。

- **M0** 本文定稿(§11 拍板)
- **M1** ✅ 2026-09-21 骨架: `go.mod` / kong / config / embed / daemon / `session list|info|url` / `read` / `open`,
  Python 写好的会话在 Go daemon 下页面完整可用 → 契约正确的证明。`git mv hudiff legacy/hudiff.py`,
  根目录 `hudiff` 暂为 shim(`exec python3 legacy/hudiff.py "$@"`), `~/.config/bin/hudiff` 软链不用动
- **M2** `diff/show/pr/patch` + source key 幂等 + `refresh` 评论对位
- **M3** 写路径: `notes / comments / submit / wait / reply / resolve`
- **M4** `review`(并入)/ `diagram`(双引擎)/ `context`
- **M5** config / keymap / hooks / `skill install` / 英文化 / author 字段
- **M6** parity 全过 → 删 `legacy/` → shim 换成 `go build` 产物(`.gitignore /hudiff`)→ goreleaser + tap

## 11. 拍板记录(2026-09-21, kackerx)

1. 会话数据默认位置 → **XDG state**(`~/.local/state/hudiff/`), `data_dir` 可配
2. CLI → **不要 `diff` 动词**, 裸 `hudiff` = 工作区; `hudiff commit <rev>` / `hudiff pr <x>` / `hudiff range a..b` / `hudiff patch`;
   `show→read`、`ctx→context`、`open`=重开 接受
3. 快捷键 → 先按 §6 草案做, 后续再调
4. Windows → v1 保证能编译、无 daemon 路径能跑; daemon 拉起用 DETACHED_PROCESS 尽力, 不承诺
5. License → **MIT**
6. README → **英文**, 顶部放 `uvx pyfiglet -w 200 -f ansi_shadow hudiff` 生成的 banner, 然后 logo, 结构按常见开源项目
   (Features / Install / Quickstart / Agent skill / Configuration / How it works / Development / License)
