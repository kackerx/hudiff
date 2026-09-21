---
name: hudiff
description: "带逐行评论的 diff 讲解。用户想【理解】一个 PR / commit / 工作区改动时用: 起本地页面, 我按 hunk 写说明, 用户在浏览器里像 GitHub PR 那样逐行评论, 评论通过 `hudiff wait` / `hudiff comments` 回流给我, 我逐条回复或改代码。用户说 /hudiff、'讲讲这个 diff'、'看看这个 PR'、'解释一下这次改动'、'我的评论看了吗' 时用。"
user_invocable: true
triggers:
  - 讲讲这个 diff
  - 解释这次改动
  - 看看这个 PR
  - 看我的评论
---

# hudiff —— 讲解 diff, 收用户的逐行意见

CLI 是 `hudiff`(要在 PATH 里), `hudiff -h` 看全部子命令。
分工: **CLI 管取 diff / 起页面 / 存评论, 我管写说明和回应评论。** 页面会自己轮询,
我写完说明它就刷新, 不用告诉用户「刷新一下」。

只有一个本地 server(默认 `http://127.0.0.1:7788/`, 环境变量 `HUDIFF_PORT` 改),
首页列所有会话, 每个会话在 `/s/<id>/`。任何子命令发现 server 不在都会自动拉起。

用法: `/hudiff [目标] [-- 审查要求]`

    /hudiff                  工作区未提交的改动(含已暂存)
    /hudiff --staged         只看已暂存
    /hudiff HEAD~1           某个 commit
    /hudiff a1b2c3..HEAD     区间
    /hudiff pr 562           GitHub PR(也接受 #562 / PR URL)
    /hudiff comments         跳过讲解, 直接处理用户已写的评论
    /hudiff HEAD~1 -- 重点看并发和错误处理, 样式改动不用讲, 对照 docs/spec.md
    /hudiff review <会话id>   交叉审查: 我是【另一个】agent, 重审别人讲解过的会话, 出报告(流程 C)

`--` 后面的话是**本次审查要求**, 原文传给 `open -p`; 里面提到的文件用 `-c <路径>` 传
(可重复)。没写要求也可能有预设: `~/.config/hudiff/default.md` 和
`~/.config/hudiff/<仓库目录名>.md`, 存在就自动带上。要求会出现在 `show` 的头部
和页面头部, 写说明时**以它为准**, 它说不用讲的就写 `null` 跳过。

## 流程 A: 讲解一段 diff

1. **开会话**: `hudiff open <目标> --json [-p "<要求>"] [-c <文件>]...`。输出里有
   `url`、`dir`、`brief`(审查要求)、文件列表。浏览器会自动打开。先把 url 告诉用户一句,
   然后立刻开始写说明(用户在等)。`brief.context` 里的文件先读。
   `open` 是**幂等**的: 同一个 commit / PR / 区间 / 工作区已有会话就复用, 输出里 `reused: true`;
   工作区内容变了会顺带 refresh(`refreshed: true`, `notes_dropped` 列出说明作废需重写的文件)。
   复用时 `has_notes` 为真说明之前已经讲解过, 只补作废的部分, 别从头重写。
   **要查会话信息(目录、地址、计数)用 `hudiff -s <id> info`, 不要再跑 open。**
2. **读 diff, 带上仓库上下文** —— 只看 diff 文本写出来的一定是复述代码, 这一步决定质量:
   - `hudiff show` 先通读一遍拿到全貌; 每个 hunk 标着 `[路径#序号]`, 这个序号就是 notes 的
     key。行号两列 = 旧版 | 新版。
   - `hudiff ctx` **必跑**: 每个文件的近史(最近 4 次提交, 动机常常就写在里面)、本次改了哪些
     符号(+新增 / -删除 / ~改动)、改动文件之外谁在引用它们(影响面)、相关测试有没有一起改。
     带 ⚠ 的行(符号删了但别处还在引用 / 没有测试覆盖)**必须**在对应卡片里点出来。
   - 非平凡的 hunk 用 `hudiff show -W --file <路径>` 看所在的整个函数/块(行首 `~` 是补出来
     的新版原文), 或 `-U 20` 前后各补 20 行。判断「这个改动对不对」要看它所在的函数, 不是那 3 行。
   - `ctx` 列出的引用方、`brief.context` 里的文件、commit message / PR 描述, 该读的直接读, 别猜。
   - 大 diff 按文件分批: `show --file` / `ctx --file`。
3. **写说明**: 生成 JSON 写到临时文件, 然后 `hudiff notes <文件>`。它会校验
   文件名 / hunk 序号是否存在, 并打印还没覆盖的 hunk。schema:

   ```json
   {
     "title": "一句话标题, 15 字内, 页头用",
     "summary": "整体说明, 2–5 句 markdown: 为什么改、改了哪几块、要注意什么",
     "diagrams": [
       {"id": "flow", "title": "图的标题", "kind": "d2", "caption": "一两句: 这张图回答什么问题",
        "hunks": ["internal/cli/config.go#0", "internal/cli/memory.go#1"],
        "src": "direction: right\n..."}
     ],
     "files": {
       "internal/cli/config.go": {
         "summary": "可选, 这个文件在本次改动里的角色, 一句话",
         "hunks": {
           "0": {"title": "短标题", "body": "markdown 正文", "lines": "new:42-47", "tags": ["需求", "#562"]},
           "1": null
         }
       }
     }
   }
   ```
   `lines` 可选, 指向 hunk 内最关键的几行(`new:42-47` / `old:10-12`), 页面 hover 卡片
   时会高亮这几行; 不写就默认整个 hunk。`tags` 可选, 用来把 hunk 归到改动线
   (如「AGENTS.md 单名」「nvim tab 直达」)。**`null` = 这个 hunk 无需说明**: 页面不出卡,
   hunk 头标「无需说明」, 校验也算覆盖。import 增删、重命名扫尾、空行、生成文件、
   token 回写这类全用 `null`, 别硬写一张「机械改动」的卡凑数。
4. **写说明的要求**(这是用户用这个工具的核心目的):
   - **讲为什么, 不复述代码。** 用户看得见 diff, 他缺的是意图、取舍、和别处的关联。
     「新增 ContextWindow 字段」是废话;「context_window 与 --context-window 共享同一个
     解析器, 避免 K/M 后缀两处实现不一致」才是要点。
   - 每张卡尽量回答四件事, 能省则省但别全省: **为什么改**(近史 / brief / commit message
     里找依据) → **关键点**(所在函数里它改变了什么行为) → **影响面**(`ctx` 里的引用方: 谁会
     受影响, 有没有一起改) → **风险与验证**(边界、并发、错误路径; 测试有没有覆盖)。
   - 跨文件的关联要写出来: A 文件新增的函数被 B 文件调用, 就在两张卡里互相点名
     (`` `bufferline.lua#3` 用了这里的 `bound_paths()` ``), 用户顺着能跳。
   - 每个非平凡 hunk 一张卡; 平凡的写 `null`。**每个 hunk 的 key 都要有**(卡或 null),
     免得用户分不清「没必要讲」和「漏了」。一个文件全是 null 时页面会自动显示
     「都是机械性改动」。真值得提一句的机械改动(比如 token 被工具回写混进了提交)
     放进 summary 里一句话带过。
   - 用户给了审查要求(`brief`)就以它为纲: 它点名的关注点要在相关卡里正面回答, 它说
     不用讲的一律 null。没有 brief 时按下面的默认标准。
   - 有风险、有疑点的地方**明说**: 「这里 nil 检查没了, 调用方是否保证非空需要确认」。
     用户要的是 review 视角, 不是宣传稿。
   - 中文, 术语和标识符保留原文并用反引号。
4b. **图解**(`diagrams`, 可选但强烈建议): 大段文字对人的解释效果远不如图, 页面中间栏有
   「图解」tab, 右栏对应变成图列表(缩略图)。「改动地图」(文件 → 改动的符号 → 引用方)是
   工具从 `ctx` 数据**自动画的**, 不用你管; 你负责画机器画不出来的**语义**:
   - **什么时候画**: 改动涉及 ≥3 个文件的调用关系 → 时序图; 改了分支/状态机/重试逻辑 →
     流程图; 改了类型、接口、结构体字段 → 类图; 迁移/重构「老形状 → 新形状」→ `steps`
     动画(同一布局两帧, 最直观)。一次改动通常 1–3 张, 不为画而画; 纯机械改动一张都不要。
   - **`kind: "d2"`** 优先(server 侧渲染, 深浅色各一份); 没装 d2 的机器才用 `mermaid`。
   - **`hunks`** 必填: 图讲的是哪几个 hunk(`路径#序号`), 页面上点标签能跳回代码, 说明卡上
     反过来有「图解」chip。缺锚点的图是孤岛。
   - **d2 要点**: 含 `.` 的 id 必须加引号(`"CLAUDE.md"`), 不然 `.` 被当成层级; 时序图开头
     `shape: sequence_diagram`; 类图 `x.shape: class` 再列成员; 前后对比用 `steps: { 1: {...}
     2: {...} }`; 连线标签写「为什么」(`a -> b: 拿 token, 失败则回退`), 不要只画箭头; 节点数
     控制在 15 以内, 多了就拆两张。`hudiff notes` 会用 d2 编译校验, 语法错直接报, 改好再写。
   - 图和卡分工: 图给全局关系, 卡给局部理由; 卡里别再用文字复述图已经画出来的调用链。
5. **等评论**: 说明写完后, 告诉用户「说明写好了, 在页面里评论, 写完点右上角『发给 agent』」,
   然后**在后台**跑 `hudiff wait`(Bash 的 `run_in_background`, timeout 给满 600000)。
   它会阻塞到用户点「发给 agent」, 然后把新评论打印出来。默认等 30 分钟(1800 秒);
   后台命令**不受** Bash 工具 10 分钟上限的影响(2026-09-21 实测 sleep 660 正常跑完), 所以
   一次 `wait` 就够, 别分段续挂。退出码 124 = 等满 30 分钟没人来, 停下告诉用户
   「想让我看评论时说一声」, 不要无限续。同时可以继续干别的事, 不用干等。

## 流程 B: 处理评论

评论到了(`wait` 返回, 或用户说「看评论」时跑 `hudiff comments --new`)之后:

1. 每条评论都带 `[id]`、文件、新版/旧版行号、被评论的那行代码、正文, 以及之前的往返。
   按 id 逐条处理, **不要合并回复**, 用户在页面上是按条看的。
2. 三种情况:
   - 是问题 → 答, `hudiff reply <id> "<回答>"`(多行用 `-` 读 stdin)。
   - 要改代码 → 先改, 改完 `hudiff reply <id> --resolve "已改: <怎么改的>"`。这只会打上
     「agent 已处理, 待你确认」的标记, **评论不会关闭** —— 解决与否是用户在页面上点的,
     agent 永远不要替用户关评论(`hudiff resolve` 同理, 默认也只是标记)。
     如果是工作区会话, 改完跑 `hudiff refresh` 让页面显示新 diff(评论会按行内容自动
     重新对位, 找不到的会标 stale, 输出里会说哪些说明作废需要重写 → 重写那些 hunk)。
   - 不同意 / 需要用户决定 → `reply` 说清理由, 不要打「已处理」标记, 让用户在页面上接着回。
3. 回完在对话里用一两句总结「回了 N 条, 改了 M 处」, 然后再次后台 `hudiff wait`
   等下一轮。用户可能在页面上对我的回复再追问, 会作为同一条评论的新回复出现。

## 流程 C: 交叉审查(`/hudiff review <会话id>`)

我是审稿人, 不是作者: 用全新的眼光重审, 产出一份**审查报告**, 页面中间栏「审查」tab 会渲染它,
每一条待处理项是一个可勾的 todo, 讲解的 agent(或人)处理完打钩并写处理结果。

1. 读材料, 一样都别漏:
   - 改动本身: `hudiff -s <id> show`、`ctx`、`show -W` 看关键函数。
   - 作者的讲解: `show` 里带 ✓ 的 hunk 在 `<会话目录>/notes.json` 有说明(`hudiff -s <id>
     open --json` 或 `ls` 能看到目录), 图解用 `hudiff -s <id> diagram` 列出、
     `diagram <图id> --src` 读 d2 源码 —— 图的结构和连线标签就是作者对调用关系的主张。
     这些是作者的观点, **要验证, 不要照抄**; 讲解里说错的也算审查发现。
   - 人的意见: `hudiff -s <id> comments --all` —— 用户在页面上写的逐行评论和作者的回复,
     里面常有「为什么这样改」的真实动机和已经达成的共识, 报告不要和已解决的讨论打架, 但用户
     提出过、作者没回应好的疑问要接着追。
   - **上一轮审查**: `hudiff -s <id> review`(不带 --open, 要看全部)。已有报告说明这是第二轮
     以上: 用户已经把一些项标成「已处理」或「拒绝」并写了理由。对**每一条**已处理 / 已拒绝的项,
     去读现在的代码, 对照处理结果或拒绝理由, 在报告里写一条**复核说明**(见下面 schema 的复核项):
     `verdict: ok` = 改法正确 / 拒绝理由成立; `disagree` = 没改对、改漏了、或拒绝理由站不住, 说清
     依据(文件:行); `info` = 补充信息。复核说明会显示在那一项的处理结果下面。不要把同一个问题换个
     说法当新项再提一遍 —— 有异议就写在复核说明里。新报告 = 复核说明 + 新发现。
     `review set` 默认是**并入**: 老项和状态全部保留, 新项追加成新一轮; **绝不要**用 `--replace`。
   - `brief`(用户的审查要求)优先级最高; 仓库里有后端宪法 / lint 约定的话对照着看。
2. 写报告 JSON, `hudiff -s <id> review set <文件> --author <我的标识>`(并入, 不覆盖)。校验会核对每条的
   `file`/`line` 在 diff 里(行号用 `show` 的**新版列**, 删除行用 `"side":"old"`), 自动补
   hunk 和 snippet。写入后会叫醒在 `wait` 的作者 agent。schema:

   ```json
   {
     "summary": "总体评价: 简评整体质量, 2–4 句 markdown",
     "strengths": ["值得称赞的点, 一条一句"],
     "items": [
       {"priority": "high", "title": "问题一句话", "file": "a/b.go", "line": 42,
        "body": "问题描述: 为什么是问题, 影响什么", 
        "suggestion": "具体改法, 带对比示例:\n```diff\n-old\n+new\n```"},
       {"id": "r1", "note": "复核: workflows.go:146 已改成历史注释, 与上文一致", "verdict": "ok"},
       {"id": "r3", "note": "拒绝理由是「有意为之」, 但 settings.json 的 model 值在上个 commit 才换回 opus, 建议再确认", "verdict": "disagree"}
     ]
   }
   ```
   只带 `id` + `note` 的是**复核项**, 追加到上一轮同 id 那一项下面, 不算新问题; 也可以事后单条加:
   `hudiff -s <id> review note r1 "…" --verdict ok`。
   `priority`: `high` = 不能发布必须改(严重违反宪法 / 静态检查问题 / 明确 bug);
   `medium` = 建议改但不影响发布(优化、可 hotfix 的小 bug、边界 case、暂可不管的性能);
   `low` = 风格与可读性。也接受 高优/中优/低优。没有具体位置的整体性问题可以不带 file。
3. 报告要求: 每一项都要能**独立执行** —— 有位置、有问题代码、有改法; 「建议重构」「注意
   一下」这种没有动作的不要写。优点要具体到哪一处好, 不写客套。宁少勿滥: 一次 3–10 项。
4. 写完就结束, **不要**自己去改代码, 也不要 `wait`。要二审由用户再发起。

作者 agent 收到(`wait` 会提示「审查报告已更新」, 或用户说「处理审查项」):
`hudiff review --open` 逐项看 → 改代码 → `hudiff review done <id> "做了什么"`;
不同意的 `hudiff review wontfix <id> "理由"`, **有分歧的留给用户裁决, 别硬 done**。
工作区会话改完跑 `refresh`。全部处理完 `hudiff review md` 可以导出 markdown 贴到 PR。

## 用户从页面「发给 agent」时会发生什么

- 有 agent 在这个会话上 `wait` → 直接叫醒它, 评论从 `wait` 的输出里来。
- 没有 → 页面让用户从本机的 Claude Code 窗口列表里选一个(优先列同仓库的), 之后这个会话就
  绑定它; 「发给 agent」会往那个窗口里**打一句提示词**:
  `hudiff 会话 <id> 有 N 条新评论, 请处理: hudiff -s <id> comments --new`。
  所以你可能在对话里收到这样一句话 —— 照做就行(流程 B), 它不是用户手打的。

## 别踩

- **`open` 是「新建会话」, 不是「打开会话」。** 同一个目标调两次就有两个会话, 首页上并排
  出现两条一模一样的 PR, 用户得自己认哪个是活的。**一个 diff 只 `open` 一次**, 之后所有
  操作都用 `--session <id>`。想再看一眼文件列表 / diff, 用 `show` / `ctx --session <id>`;
  想拿地址用 `hudiff url -s <id>`; 想看开过哪些用 `hudiff ls`。真建重了,
  `hudiff rm <没写说明的那个 id>` 删掉。
- **每条命令都带 `--session <id>`**(`open --json` 输出里的 `session`)。不带时取的是
  「当前仓库最近一次 open 的会话」, 同一仓库开着别的 Claude Code 实例时会串。
  `--session` 放子命令前后都行。
- 「发给 agent」发的是**会话**, 不是某个进程: 谁在这个会话上 `wait` 谁被叫醒。多实例时
  各自 open 各自的会话就不会互相收到。
- 会话在 `~/.local/state/hudiff/sessions/<id>/`, 不在 /tmp, 不自动清; `hudiff ls` 看,
  `hudiff url -s <id>` 打印地址(重放), `hudiff url --home` 是首页, `hudiff rm <id>` 删。
  server 重启不影响: 端口固定, 页面自动重连, `wait` 会等它回来。
- **`notes` 是整份覆盖, 不是增量合并。** 第二次提交会把第一次的说明全部冲掉 —— 哪怕这次
  的 JSON 里根本没提到那些文件。它输出的「还没覆盖的 hunk」是在说**这一份**缺什么, 不是
  在邀请你分批补。大 diff 分批**读**没问题, 但 JSON 要在本地攒成一份完整的再 `notes` 一次
  (`title` / `summary` / `diagrams` 同理, 漏写就等于删掉)。
- `notes` 校验失败时不写入, 改好再跑; 不要 `--force`。
- 不要自己动 `comments.json`, server 是唯一写者; `reply` / `resolve` 会走 HTTP。
- `refresh` 只对工作区 / commit / 区间 / PR 会话有效, `--patch` 会话没有来源可重取。
- `hudiff stop` 关 server(数据不删), 下次任何命令再拉起。
- `hudiff diagram` 列出所有图; `hudiff diagram auto-map --src` 看自动图的 d2 源码(想在它基础上
  改一张更好的图时可以拿来当底稿); `hudiff diagram <id> -o x.svg` 导出。
