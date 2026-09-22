# hudiff TUI —— 后续版本的设计备忘

状态: **不进 v1**(2026-09-22 拍板)。web 版已经做得很好, TUI 对这种交互密集的 UI 收益不如成本; 等 Go 版发布、有真实需求再做。
本文只记结论, 免得到时候重新讨论。

## 0. 定位

TUI 是 daemon HTTP API 的**第二个客户端**, 与浏览器同级(spec §5「TUI 口子」)。不走进程内直调领域包, 原因是业务规则
"daemon 是评论的唯一写者": 写评论、被「发给 agent」叫醒, 走 HTTP 全是现成的(`/api/wait` 长轮询就是事件源), 走进程内要和
daemon 抢写权、再实现一遍通知。所以 v1 期间守住一条: **daemon API 必须覆盖页面能做的每件事**, 页面不许有 API 没有的私有能力。

## 1. 前置: 先抽 `internal/view`(做 TUI 的第一个 M)

现在的视图模型住在 `web/app.js` 里: `renderTree / noteRange / rvCounts / dgForHunk / cmtLoc / renderReviewOutline` 这批
"哪个文件有几条未解决评论、hunk 对应哪几张图、审查项按优先级怎么分组、评论落在哪一行"的**派生逻辑**是 JS 算的, API 只给原始
session / diff / notes / comments。TUI 要用 Go 再算一遍 —— 六边形不管两个驱动适配器之间的重复, 这要靠下沉解决。

做法: 抽一个纯函数包 `internal/view`(输入 session/diff/notes/comments/review, 输出 tree / counts / hunk↔diagram 映射 /
comment 定位 / review outline), daemon 加 `GET /s/{sid}/api/view` 给浏览器, TUI 进程内直接调。约 300 行, 把 TUI 的重复部分
砍掉一半, 顺手让 app.js 变薄。**v1 期间的纪律**: 新写的"数据整形"逻辑进 Go 领域包, 不进 app.js; app.js 只做 DOM。

## 2. 选型: Bubble Tea v2 全家桶 + chroma

| 库 | 谁在用(2026-09 查本机二进制) | 结论 |
|---|---|---|
| gocui(lazygit 自己 fork) | lazygit | ❌ 事实上是 lazygit 私有库, 零生态 |
| tview v0.42 + tcell | k9s(lock 在 tview v0.8.5) | ⚠️ 出仪表盘最快, 但自绘带行内评论的 diff 视图要绕过它的 widget 抽象 |
| **bubbletea/v2 v2.0.9 + lipgloss/v2 v2.0.6 + bubbles** | opencode / glow / another Go CLI(bubbles v0.21 + huh + lipgloss v1) | ✅ |
| chroma/v2 v2.27 | 语法高亮(Pygments 的 Go 移植, 作者和 kong 同一人) | ✅ TUI 侧唯一新增的"新"依赖 |

选 Bubble Tea 的理由: TUI 的全部状态来自轮询 `/api/state` + 长轮询 `/api/wait` 的快照, Elm 架构(`Model / Update(msg) / View()`)
里这就是 `tea.Cmd` 发 HTTP → `Msg` → `Update` → 纯函数 `View`, 和 web 端"5 秒轮询 + 重渲染"同一套心智; 维护者已会; diff 视图就是
bubbles 的 `viewport`(只渲染可见行, 5k 行零压力 —— web 端靠 content-visibility 做的事终端天然免费); 评论表单用 `textarea`。
lazygit 用 gocui 是 2018 年的历史原因, 不是推荐。`hunk`(用户参考过其 CLI 设计)是 Node 脚本, 实现不可借。

## 3. 图: ASCII 化, 不用终端图形协议

- **d2 图免费**: d2 v0.9 同一个 Go 模块里有 `d2renderers/d2ascii`(CLI 即 `d2 x.d2 x.txt` / `--stdout-format ascii`), 同一份源码、同一
  布局(TALA)直接出 Unicode 框线图。内嵌后不走 CLI flag, 而是拿 `d2lib.Compile` 已经算好布局的 `*d2target.Diagram` 直接
  `d2ascii.Render(diagram, &d2ascii.RenderOpts{Charset: charset.Unicode | charset.ASCII, Scale})` —— 布局算一次, SVG 和文本是同一张图的
  两种画法。`Charset` 对应 CLI 的 `--ascii-mode`: `extended`(Unicode 框线, 默认, TUI 用这个)/ `standard`(纯 7-bit ASCII, 给
  `hudiff diagram <id> --ascii` 这类要贴进 PR 评论 / 聊天的场景)。`Scale` 用来适配终端宽度。2026-09-22 实测两种模式都能出图, 边标签
  的落点比 SVG 粗糙(标签会挤在拐角), 可接受。2026-09-22 用 4d71 的 auto-map 实测能出图; **已知缺陷**: CJK 宽字符会把框线撑歪("其 它 27 个"
  被拆成单字加空格), 英文标签正常 —— UI 全英文化后 auto-map 没问题, agent 写的中文标签会歪, 到时看 d2ascii 有没有修 wcwidth。
- **mermaid 图没有 ASCII 路径**: TUI 里只列标题 + caption + 关联 hunk, 按键用系统浏览器打开该图的 `/api/diagram` URL; 不引 graph-easy /
  plantuml 之类的外部转换。
- 否掉 kitty graphics protocol(把 PNG 画进终端): 只覆盖 kitty / ghostty / WezTerm, tmux 要开 passthrough, 用户明确要 ASCII。

## 4. 难点排序(从难到易)与体量

1. 评论表单的交互密度: 光标在行上按 `c` 弹 textarea, 提交后回到原位; 键位直接复用 spec §6 的 keymap
2. 三栏布局与折叠: lipgloss `JoinHorizontal` + 自己算宽度, resize 重算
3. 代码 / 审查 tab 的渲染: 翻译 app.js, 体力活(抽了 `internal/view` 之后只剩 DOM→字符串的部分)
4. 图解 tab: d2ascii 直出 + mermaid 退化为列表
5. 与 daemon 通信: 最简单, `net/http` + JSON

估算: MVP(代码 tab + 评论 + 发送/等待状态)1.5–2.5k 行 Go; 全功能对齐 web 三 tab 3–4k 行(前提是 `internal/view` 已抽)。

## 5. 接入点(已在 v1 预留)

- `config.viewer: web|tui`, `--tui` flag(v1 报 not implemented)
- `hudiff open --tui <id>` 是入口; 建会话命令的 `--tui` 等价于建完直接进 TUI
- 不新增 daemon 能力; 若 TUI 需要某个 API 而页面没有 —— 说明页面在偷用私有能力, 先修页面
