<div align="center">

# 🐾 go-tiny-claw - Week 03

**上下文管理与驾驭工程：Session、压缩、自愈、Plan 模式**

本分支给 Week 02 那个「能干活但健忘」的 Agent 装上了记忆系统与会话生命周期--让它能持续长程工作、上下文不爆、报错能自救、复杂任务能拆解持久化。

![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)
![Status](https://img.shields.io/badge/status-learning%20project-orange)
![Phase](https://img.shields.io/badge/phase-context%20mgmt-blueviolet)

</div>

---

## 📖 本分支简介

Week 02 的 Agent 每轮把全部历史塞给大模型，跑几轮就会 OOM，报错了也只能傻重试，长任务没记忆。Week 03 引入整个 `internal/context/` 包来根治这些问题：

1. **Session 会话管理** -- 持久化完整历史，但每轮只取「最近 N 条」作为工作记忆，并自动剔除孤儿工具响应避免 API 400。
2. **Compactor 上下文压缩** -- 超过水位线后，远期历史全量掩码、近期记忆掐头去尾，防 OOM。
3. **Composer 动态系统提示** -- 用组装器替代硬编码 prompt，按工作区加载 `AGENTS.md` 与 Skills，并支持 Plan 模式。
4. **Recovery 错误自愈** -- 匹配报错特征（路径错、命令找不到、模糊替换失败…）注入救援指南，引导模型自纠错。
5. **Plan 模式** -- 强制把架构思路与进度持久化到 `PLAN.md` / `TODO.md`，支持断点续传。

> ⚠️ 本分支的 `cmd/claw/main.go` 是一个**演示自愈的单次测试脚本**（硬编码一个故意诱发 `edit_file` 失败的 prompt），不再是 Week 02 的交互式 REPL。飞书集成因 `loop.Run` 签名重构未同步，本分支暂时编译失败（见下文「已知问题」）。

---

## 🆕 相对 Week 02 的增量

| 维度 | Week 02 | Week 03 |
|------|---------|---------|
| 上下文 | 每轮全量历史，硬编码 prompt | `Session` 工作记忆 + `Compactor` 压缩 |
| 系统提示 | 硬编码字符串 | `Composer` 动态组装（AGENTS.md + Skills + Plan） |
| 报错处理 | 原样回传，靠模型自己悟 | `Recovery` 匹配特征注入救援指南 |
| 长任务 | 无状态 | Plan 模式：PLAN.md/TODO.md 状态外部化 + 断点续传 |
| 引擎签名 | `Run(ctx, prompt, reporter)` | `Run(ctx, *Session, reporter)` |
| WorkDir | Engine 持有 | 移到 Session（会话绑定工作区） |

---

## ✨ 核心特性

| 能力 | 说明 |
|------|------|
| 🧠 **Session 工作记忆** | `GetWorkingMemory(N)` 截取最近 N 条；首条若是孤儿工具响应（有 ToolCallID 但对应 ToolCall 被截断）则丢弃，避免大模型 API 400 |
| 🔒 **并发安全会话** | `sync.RWMutex` 保护历史读写；`GlobalSessionMgr` 单例支持多会话隔离 |
| 🗜️ **双重降级压缩** | 远期工具输出 >200 字符全量掩码；近期保护区单条 >1000 字符掐头去尾（保留前 500 + 后 500）；System Prompt 与 ToolCalls 永不动 |
| 📝 **动态系统提示** | 极简内核（身份 + 6 条纪律）+ `AGENTS.md` 项目规范 + `.claw/skills/*/SKILL.md` 技能外挂 |
| 📋 **Plan 模式** | 强制环境嗅探（PLAN.md/TODO.md 是否存在）-> 全新任务则创建、断点续传则续跑 -> 单步完成实时打勾 -> 迷失时重读 TODO 自救 |
| 🩺 **错误自愈注入** | `RecoveryManager` 按 `edit_file`/`read_file`/`write_file`/`bash` 分类匹配报错关键字，拼接 `[系统救援指南]` 回灌模型 |
| ⚡ **并行工具执行** | 沿用 Week 02 的 goroutine + WaitGroup + 预分配切片无锁写入 |
| 💾 **全量持久化** | 写入 Session 的是全量真实响应，Compact 只作用于本轮发给大模型的临时上下文 |

---

## 🏗️ 架构概览

```
                ┌─────────────────────────────────────┐
  Session ───►  │          AgentEngine.Run()           │
  (history)     │   (compactor + recovery + planMode)  │
                └──────────────────┬──────────────────┘
                                   │
        ┌──────────────────────────┴──────────────────────────┐
        ▼                                                     │
  ┌──────────────┐                                            │
  │  Composer    │ ── 组装 System Prompt ──┐                  │
  │ (AGENTS+Skill│                         ▼                  │
  │  +PlanMode)  │              ┌──────────────────┐          │
  └──────────────┘              │ contextHistory   │          │
                                │ = systemMsg      │          │
  Session.GetWorkingMemory(20) ─┤ + workingMemory  │          │
                                └────────┬─────────┘          │
                                         ▼                    │
                                ┌──────────────────┐          │
                                │   Compactor      │ ── 超标则压缩
                                │   .Compact()     │          │
                                └────────┬─────────┘          │
                                         ▼                    │
                              Phase 1 Thinking ──► Phase 2 Action
                                         │            │       │
                                         │     无 ToolCall → 结束
                                         │            │       │
                                         │     有 ToolCall → 并行执行
                                         │            │       │
                                         │            ▼       │
                                         │     ┌──────────────┐
                                         │     │  Recovery    │ ── 失败则注入救援指南
                                         │     │  (if error)  │       │
                                         │     └──────┬───────┘       │
                                         │            ▼               │
                                         └─── 全量 Observation 持久化到 Session
                                                              下一轮 ──┘
```

---

## 🚀 快速开始

### 环境要求

- **Go 1.26+**
- **`DEEPSEEK_API_KEY`**（必填）

### 构建与运行

```bash
git clone https://github.com/zlgit1/go-tiny-claw.git
cd go-tiny-claw
git checkout week03

export DEEPSEEK_API_KEY="sk-xxxxxxxx"

# 构建入口用 ./cmd/claw（不要用 ./...，见「已知问题」）
go run ./cmd/claw
```

### 预期行为

`main.go` 硬编码了一个**自愈测试场景**：命令 Agent 直接用 `edit_file` 修改 `workspace/auth.go` 的 `login` 函数，但故意不给它先读文件的机会，诱发 `old_text` 不匹配错误。运行后可以观察到：

1. Agent 第一次 `edit_file` 失败（old_text 与文件实际内容不一致）
2. `RecoveryManager` 匹配到「未找到 old_text」特征，注入救援指南：*「请先使用 read_file 重新读取该文件，获取最新内容后再编辑」*
3. Agent 读取文件后，带着正确内容重新编辑成功

这正是「错误自愈注入」的演示闭环。

---

## 📂 项目结构

```
go-tiny-claw/                     (week03 分支)
├── cmd/
│   └── claw/
│       └── main.go               # 自愈测试脚本（硬编码 prompt 演示 Recovery）
├── internal/
│   ├── schema/message.go
│   ├── provider/                 # DeepSeek/OpenAI/Claude 三实现
│   ├── tools/                    # BaseTool + Registry + 4 工具（沿用 Week 02）
│   ├── engine/
│   │   ├── loop.go               # 接入 Session/Compactor/Recovery/Composer
│   │   ├── reporter.go
│   │   └── terminal_reporter.go
│   ├── context/                  # ★ 本周新增
│   │   ├── session.go            # 会话 + 工作记忆 + 全局 SessionManager
│   │   ├── compactor.go          # 上下文压缩（双重降级）
│   │   ├── composer.go           # 动态系统提示 + Plan 模式
│   │   ├── recovery.go           # 错误特征匹配 + 救援指南注入
│   │   └── skill.go              # SKILL.md 加载与 YAML frontmatter 解析
│   └── feishu/bot.go             # ⚠️ 因 loop 签名变更未同步，本分支编译失败
├── workspace/                    # 示例靶机（Agent 操作的对象）
│   ├── AGENTS.md                 # 项目专属规范
│   ├── PLAN.md / TODO.md         # Plan 模式的状态外部化产物
│   ├── main.go / auth.go / ...   # 一个极简 Go Web Server demo
│   ├── .claw/skills/git-workflow/SKILL.md
│   └── tiny-claw                 # ⚠️ 7.8MB 构建产物（被误跟踪，见「已知问题」）
├── go.mod / go.sum / .gitignore
└── server.go / helloworld.go     # ⚠️ 遗留脚手架（与 main 冲突，master 已清理）
```

---

## 📦 模块说明（`internal/context/`）

### `session.go` - 会话管理
`Session` 持有完整 `history`（`sync.RWMutex` 保护）。`GetWorkingMemory(limit)` 从尾部截取最近 N 条，并丢弃首条「孤儿工具响应」（有 `ToolCallID` 但对应 `ToolCall` 已被截断的消息），避免大模型 API 报 400。`GlobalSessionMgr` 提供按 ID 获取/创建会话的单例，支撑多用户隔离。

### `compactor.go` - 上下文压缩
`Compact()` 在总字符数超过 `MaxChars` 水位线时触发：
- **远期历史**（非工作记忆区）：工具输出 >200 字符全量掩码替换、模型推理 >200 字符折叠
- **近期保护区**：单条工具输出 >1000 字符掐头去尾（前 500 + 后 500）
- System Prompt 与 `ToolCalls` 永不修改（维系逻辑链）

### `composer.go` - 动态系统提示
`Build()` 按顺序组装：极简内核（身份 + 6 条核心纪律）→ Plan 模式指令（若开启）→ `AGENTS.md`（若存在）→ Skills（若存在）。

### `recovery.go` - 错误自愈
`AnalyzeAndInject(toolName, rawError)` 按工具分类匹配报错关键字（如 `no such file`、`command not found`、`未找到 old_text`），命中则拼接 `[系统救援指南]` 回灌，未命中则原样返回。

### `skill.go` - 技能加载
扫描 `<workDir>/.claw/skills/*/SKILL.md`，解析 YAML frontmatter（`name` / `description`），把技能正文格式化后注入系统提示。

---

## 📋 Plan 模式

开启 `PlanMode` 后，系统提示会注入一套**绝对顺序**指令：

1. **环境嗅探**：用 `bash ls` 检查 `PLAN.md` / `TODO.md` 是否存在
2. **分支 A（全新任务）**：不存在则创建 PLAN.md（架构设计）+ TODO.md（checkbox 步骤）
3. **分支 B（断点续传）**：已存在则**绝不覆盖**，读 TODO.md 找第一个 `- [ ]` 续跑
4. **单步打勾**：每完成一个子任务立即用 `edit_file` 改为 `- [x]`，禁止「一口气写完最后打勾」
5. **迷失自救**：报错或卡住时重读 TODO.md 确认位置

这让 Agent 在系统重启或人类接管后能从断点继续，而不依赖短期记忆。

---

## ⚠️ 已知问题

1. **根 package 编译冲突**：根目录 `server.go` / `helloworld.go` 都声明了 `func main()` 且 `server.go` 引用未定义的 `user`，导致 `go build ./...` 失败。**用 `go build ./cmd/claw` 构建**。这两个遗留脚手架在 `master` 分支已清理。

2. **飞书集成未同步**：本分支把 `engine.Run` 签名从 `(ctx, prompt string, reporter)` 改为 `(ctx, *Session, reporter)`，但 `internal/feishu/bot.go` 的 `handleAgentRun` 仍传 string，导致 feishu 包编译失败（`cannot use prompt as *Session`）。`cmd/claw` 不依赖 feishu，不受影响。该问题在后续分支修复。

3. **`workspace/tiny-claw`**：一个 7.8MB 的编译产物被误提交到仓库（应被 `.gitignore` 忽略）。历史遗留，未在本分支清理。

---

## 🗺️ 能力边界与后续演进

| 缺失能力 | 后续分支 |
|----------|----------|
| 死循环探测、飞书人工审批、子智能体 | Week 04 |
| 链路追踪、Token/成本核算、评测跑分 | Week 05 |

> 本分支的 `Session` 尚无 Token/Cost 统计字段（Week 05 加入），`loop` 也还没有链路追踪埋点。

主线（含全部能力、已清理遗留、含完整 README）位于 `master` 分支。

---

## 📝 License

本项目为个人学习实践项目，目前**尚未声明开源许可证**。

<div align="center">

<sub> Week 03 - Agent 学会了记忆，不再健忘。 </sub>

</div>
