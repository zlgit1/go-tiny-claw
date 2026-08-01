<div align="center">

# 🐾 go-tiny-claw

**从零手搓的极简 AI Coding Agent 驾驭框架（Harness）**

用 Go 从第一性原理实现一个类 Claude Code 的自主编程智能体内核 —— 工具调用、上下文压缩、错误自愈、死循环干预、链路追踪、成本核算、人工审批，一个都不缺。

![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)
![Status](https://img.shields.io/badge/status-learning%20project-orange)
![Paradigm](https://img.shields.io/badge/paradigm-ReAct%20Agent-blueviolet)

</div>

---

## 📖 项目简介

`go-tiny-claw` 是一个**教学性质的 Agent 驾驭框架（Agent Harness）**实现。它不依赖任何 Agent 框架（LangChain / AutoGen 等），而是用一个最小化的 Go 代码库，把一个能自主读代码、写代码、跑命令、修 Bug 的智能体从零搭起来。

核心理念是「**驾驭工程（Steering Engineering）**」：大模型只是大脑，真正决定 Agent 上限的，是包裹在它外面的那一层工程控制——上下文怎么管、工具怎么路由、报错怎么救、死循环怎么打断、花钱怎么算账、高危操作怎么拦。

> 仓库顶层目录的 `helloworld.go` / `server.go` / `*.txt` 等是早期练习遗留的脚手架，真正的工程代码位于 `cmd/` 与 `internal/`。

---

## ✨ 核心特性

| 能力 | 说明 |
|------|------|
| 🧠 **Thinking-Action 双阶段循环** | 每轮先慢思考（Reasoning），再行动（Action），观察（Observation）后进入下一轮，标准 ReAct 范式 |
| 🛠️ **可插拔工具系统** | `BaseTool` 接口 + `Registry` 路由，内置 `read_file` / `write_file` / `edit_file` / `bash` / `spawn_subagent` |
| ⚡ **并行工具执行** | 单轮内多个 `tool_call` 通过 goroutine 并发执行，结果按序回收 |
| 🗜️ **上下文压缩（Compactor）** | 超过水位线后，远期历史全量掩码、近期记忆掐头去尾截断，防 OOM |
| 🧩 **工作记忆（Working Memory）** | 仅取最近 N 条消息，并自动剔除「孤儿工具响应」避免 API 400 |
| 🩺 **错误自愈注入（Recovery）** | 匹配报错特征（`no such file` / `command not found` / 模糊替换失败…）注入救援指南 |
| 🔁 **死循环探测（Reminder）** | MD5 指纹识别连续 3 次相同失败，强制注入打断指令 |
| 📋 **Plan 模式** | 强制把架构思路持久化到 `PLAN.md` / `TODO.md`，支持断点续传 |
| 🎓 **技能外挂（Skills）** | 从 `.claw/skills/*/SKILL.md` 动态加载 SOP 注入系统提示 |
| 💰 **成本核算（CostTracker）** | 装饰器包裹 Provider，按模型计价累加 Token 与费用到 Session |
| 📊 **全息链路追踪（Tracing）** | 树形 Span 嵌套，导出 JSON 到 `.claw/traces/` 供回放 |
| 🤖 **子智能体（Subagent）** | `spawn_subagent` 派出只读探路者，万行代码化作一段摘要回传主干 |
| 🛡️ **人工审批（Human-in-the-Loop）** | 危险命令命中黑名单 → 挂起协程 → 飞书 `approve` / `reject` 口令放行 |
| 📡 **多端 Reporter** | 终端彩色输出 / 飞书消息卡片，同一引擎无缝切换展现层 |
| 🔌 **多 LLM Provider** | DeepSeek / 智谱 GLM（OpenAI 兼容 & Claude 兼容端点）统一抽象 |
| 🧪 **评测跑分（Eval Harness）** | 沙箱化测试用例：Setup → Agent 执行 → Validate 脚本断言，产出成功率与成本报表 |

---

## 🏗️ 架构概览

```
                    ┌─────────────────────────────────────────┐
   用户 Prompt ───► │            AgentEngine.Run()             │
                    │                                         │
                    │  ┌──────────┐   ┌─────────────────────┐ │
                    │  │ Composer │──►│   System Prompt     │ │
                    │  │ (AGENTS  │   │  (身份+规范+技能)    │ │
                    │  │  +Skill) │   └─────────────────────┘ │
                    │  └──────────┘            │              │
                    │                          ▼              │
                    │  ┌──────────────────────────────────┐   │
                    │  │            Main Loop             │   │
                    │  │  Thinking ──► Action ──► Observe │   │
                    │  └──────────────────────────────────┘   │
                    │       │            │           │        │
                    │       ▼            ▼           ▼        │
                    │  CostTracker   Registry    Compactor    │
                    │  (Token/费用)  (工具+MW)   (上下文压缩)  │
                    │       │            │                      │
                    │       ▼            ▼                      │
                    │   LLM Provider  Tool 执行                 │
                    │  (DeepSeek/GLM) (read/write/bash...)     │
                    └────────────────────┬────────────────────┘
                                         │
                          ┌──────────────┴──────────────┐
                          ▼                             ▼
                   TerminalReporter              FeishuReporter
                   (CLI 彩色输出)               (飞书卡片 + 审批)
```

**单轮循环（Turn）的数据流：**

```
System Prompt + Working Memory(最近N条)
        │
        ▼
   Compactor 压缩 ──► [可选] Thinking 调用 ──► Action 调用(带 Tools)
                                              │
                                   ┌──────────┴──────────┐
                                   ▼                     ▼
                              无 ToolCall             有 ToolCall
                              → 任务结束           并发执行 → Recovery 注入
                                                   → Reminder 死循环探测
                                                   → 回到循环顶部
```

---

## 📦 模块说明

| 包 | 职责 |
|----|------|
| `internal/schema` | 全局数据模型：`Message` / `ToolCall` / `ToolResult` / `Usage` |
| `internal/provider` | `LLMProvider` 统一契约 + DeepSeek / OpenAI(智谱) / Claude(智谱) 三实现 |
| `internal/tools` | `BaseTool` 接口、`Registry` 路由、中间件链、5 个内置工具 |
| `internal/engine` | `AgentEngine` 主循环、`ReminderInjector` 死循环探测、`Reporter` 接口及终端实现 |
| `internal/context` | `Session` 会话、`Compactor` 压缩、`PromptComposer` 系统提示组装、`RecoveryManager` 错误自愈、`SkillLoader` 技能加载 |
| `internal/observability` | `Span` 树形链路追踪、`CostTracker` 成本装饰器 |
| `internal/eval` | `BenchmarkRunner` 评测跑分器（沙箱隔离 + 脚本断言） |
| `internal/feishu` | 飞书 Bot（WebSocket/HTTP）、`FeishuReporter`、`ApprovalManager` 人工审批 |
| `cmd/claw` | CLI 入口（YOLO 模式，终端输出） |
| `cmd/bench` | 评测跑分入口 |
| `cmd/agentops` | 飞书 ChatOps 服务端（Webhook + 审批中间件） |

---

## 🚀 快速开始

### 环境要求

- **Go 1.26+**
- 任一 LLM API Key（推荐 DeepSeek，便宜）

### 1. 克隆并构建

```bash
git clone https://github.com/zlgit1/go-tiny-claw.git
cd go-tiny-claw
go build -o bin/claw ./cmd/claw
```

### 2. 配置环境变量

```bash
# 必填：LLM 大脑
export DEEPSEEK_API_KEY="sk-xxxxxxxx"

# 飞书 ChatOps 模式才需要（cmd/agentops）
export FEISHU_APP_ID="cli_xxxxxxxx"
export FEISHU_APP_SECRET="xxxxxxxx"
# 智谱 GLM（OpenAI/Claude 兼容端点）需要
export ZHIPU_API_KEY="xxxxxxxx"
```

### 3. 跑起来

```bash
# 让 Agent 在 workspace 目录下干活（终端彩色输出）
./bin/claw -prompt "阅读 workspace/main.go，帮我修复其中的并发数据竞态" -dir ./workspace

# 指定会话 ID，支持断点续传
./bin/claw -prompt "继续刚才的任务" -dir ./workspace -session my_feature_branch
```

CLI 参数：

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-prompt` | （必填） | 交给 Agent 执行的任务描述 |
| `-dir` | `.` | Agent 运行的工作区目录 |
| `-session` | `cli_default_session` | 会话 ID，用于断点续传与账单隔离 |

任务结束后，本次执行的链路追踪会落盘到 `<workdir>/.claw/traces/`，并在终端打印累计 Token 与费用。

---

## 📖 三个入口

### `cmd/claw` — CLI 模式

本地终端使用，默认 **YOLO 模式**（全权信任，不挂审批中间件），适合开发自测。

### `cmd/bench` — 评测跑分

```bash
export DEEPSEEK_API_KEY="sk-xxxx"
go run ./cmd/bench
```

为每个用例创建物理隔离的沙箱目录，执行 `SetupScript` 准备靶机 → 让 Agent 干活 → 跑 `ValidateScript` 断言（`exit 0` 即通过），最后输出成功率与总成本报表。内置两个示例用例：模糊编辑准确性、代码阅读 + 单测生成。

### `cmd/agentops` — 飞书 ChatOps 服务端

```bash
export DEEPSEEK_API_KEY="sk-xxxx"
export FEISHU_APP_ID="cli_xxxx"
export FEISHU_APP_SECRET="xxxx"
go run ./cmd/agentops
# 监听 :48080/webhook/event，需配合 ngrok 暴露公网
```

在飞书群里 @机器人 即可下发任务；Agent 遇到危险命令（`rm -r` / `sudo` / 覆盖 `.go` 源码…）会**挂起协程**并发卡片请求审批，人类回复 `approve <taskID>` 或 `reject <taskID>` 决定是否放行。

---

## ⚙️ 配置与扩展

### AGENTS.md — 项目专属规范

在工作区根目录放一个 `AGENTS.md`，`PromptComposer` 会自动读取并注入系统提示，约束 Agent 在该项目中的行为红线。示例见 `workspace/AGENTS.md`（运维基线守则）。

### Skills — 标准作业外挂

在 `<workdir>/.claw/skills/<name>/SKILL.md` 放置带 YAML Frontmatter 的技能文件，`SkillLoader` 会扫描并按触发条件注入：

```markdown
---
name: git-workflow
description: 当人类用户要求你"提交代码"或执行 Git 操作时，必须使用此技能。
---

# 提交流程 SOP
1. 先用 bash 调用 git status 确认改动
2. commit message 必须用 Emoji 开头，例如 🚀 feat: ...
3. 严禁 git commit -am "update" 这种敷衍提交
```

### 中间件 — 自定义安全策略

`Registry.Use(mw)` 挂载全局中间件，在工具执行前拦截：

```go
registry.Use(func(ctx context.Context, call schema.ToolCall) (bool, string) {
    if feishu.IsDangerousCommand(call.Name, string(call.Arguments)) {
        // 挂起 → 飞书审批 → 放行或拒绝
        allowed, reason := feishu.GlobalApprovalMgr.WaitForApproval(...)
        return allowed, reason
    }
    return true, ""
})
```

### 接入新的 LLM

实现 `provider.LLMProvider` 接口（只有一个 `Generate` 方法）即可：

```go
type LLMProvider interface {
    Generate(ctx context.Context, messages []schema.Message,
        availableTools []schema.ToolDefinition) (*schema.Message, error)
}
```

用 `observability.NewCostTracker` 包一层即可获得 Token 与成本追踪，无需改动主循环。

---

## 📂 项目结构

```
go-tiny-claw/
├── cmd/
│   ├── claw/              # CLI 入口
│   ├── bench/             # 评测跑分入口
│   └── agentops/          # 飞书 ChatOps 服务端入口
├── internal/
│   ├── schema/            # 数据模型
│   ├── provider/          # LLM 抽象与实现 (deepseek/openai/claude)
│   ├── tools/             # 工具系统 + 中间件
│   ├── engine/            # Agent 主循环 / 提醒 / Reporter
│   ├── context/           # 会话 / 压缩 / 组装 / 自愈 / 技能
│   ├── observability/     # 链路追踪 / 成本核算
│   ├── eval/              # 评测跑分器
│   └── feishu/            # 飞书 Bot / 审批
├── workspace/             # 示例工作区（Agent 实际操作的对象）
│   ├── AGENTS.md          # 项目专属规范
│   ├── .claw/
│   │   ├── skills/        # 技能外挂 (git-workflow / ops_troubleshoot)
│   │   └── traces/        # 链路追踪落盘
│   └── ...                # 一个极简 Go Web Server 靶机项目
├── go.mod
└── README.md
```

---

## 🗺️ 开发历程

本项目按周迭代，每个 commit 是一个完整里程碑：

| 阶段 | 主题 |
|------|------|
| **Week 01** | 项目脚手架、Go Module、DeepSeek Provider、`ReadFile` 工具 |
| **Week 02** | 工具系统（Registry + 中间件）、Reporter、飞书 Bot、Agent 主循环 |
| **Week 03** | 上下文管理（Session / Compactor / Recovery）、Plan 模式、断点续传 |
| **Week 04** | 提醒引擎（死循环探测）、飞书审批、子智能体工具、循环增强 |
| **Week 05** | Benchmark 评测、链路追踪（Tracing）、Token 与成本核算 |

---

## 📝 License

本项目基于 [MIT License](./LICENSE) 开源，Copyright (c) 2026 zlgit1。

你可以自由使用、复制、修改、合并、发布、分发甚至商用，只需在所有副本中保留版权声明与许可声明即可。本项目按"原样"提供，不附带任何担保。

<div align="center">

<sub> Built with ☕ and a lot of 驾驭工程. </sub>

</div>
