<div align="center">

# 🐾 go-tiny-claw — Week 01

**Agent 驾驭框架的第一块基石：用 mock 跑通 ReAct 双阶段循环**

本分支是项目的初始骨架（`Initial commit: go-tiny-claw project setup`）。它不接真实大模型，而是用一组 mock 把「Thinking → Action → Observation」的引擎心跳跑通，并定义好后续所有工程要遵守的三大接口契约。

![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)
![Status](https://img.shields.io/badge/status-learning%20project-orange)
![Phase](https://img.shields.io/badge/phase-skeleton-blueviolet)

</div>

---

## 📖 本分支简介

Week 01 的目标是**搭骨架、定契约、跑通循环**，而不是做出一个能干活的 Agent。

具体来说，它做了三件事：

1. **定义数据模型**（`internal/schema`）——`Message` / `Role` / `ToolCall` / `ToolResult` / `ToolDefinition`，这是与大模型沟通的通用语言。
2. **定义三大接口契约**（`provider` / `tools` / `engine`）——LLM 怎么调、工具怎么路由、循环怎么转，后续 Week 02–05 的所有工程都填进这套契约里。
3. **用 mock 验证心跳**（`cmd/claw`）——`mockProvider` 模拟「先思考、再调 bash、最后总结退出」的完整两轮循环，**零配置即可运行**，证明引擎逻辑正确。

> 真实的 LLM Provider（DeepSeek / 智谱 GLM 的 OpenAI 与 Claude 兼容端点）实现已就位于 `internal/provider/`，但本分支的 `main.go` 尚未接入，留待后续分支挂载真实大脑。

---

## ✨ 核心特性（Week 01 已具备）

| 能力 | 说明 |
|------|------|
| 🧠 **Thinking-Action 双阶段循环** | 每轮先剥夺工具强制慢思考（规划），再恢复工具执行行动，标准 ReAct 范式 |
| 🔁 **完整 Turn 生命周期** | Thinking → Action → Observation → 下一轮，无 ToolCall 即任务完成退出 |
| 🧩 **三大接口契约** | `LLMProvider` / `Registry` / `AgentEngine` 接口先行，实现可插拔 |
| 🛠️ **工具调用闭环** | 模型发起 `ToolCall` → Registry 路由执行 → 结果带 `ToolCallID` 回灌上下文，维系推理链 |
| 📁 **工作区边界** | `AgentEngine.WorkDir` 确立 Agent 的物理活动边界（借鉴 OpenClaw 理念） |
| ✅ **零配置可跑** | mock 驱动，无需任何 API Key，`go run` 即可看到完整循环日志 |
| 🔌 **多 LLM 实现已就位** | DeepSeek / OpenAI(智谱) / Claude(智谱) 三套 Provider 代码已存在，待接入 |

---

## 🏗️ 架构概览

```
                ┌─────────────────────────────┐
  userPrompt ─► │        AgentEngine          │
                │   (WorkDir + EnableThinking) │
                └──────────────┬──────────────┘
                               │
              ┌────────────────┴───────────────┐
              ▼                                 ▼
      ┌──────────────┐                  ┌──────────────┐
      │  Phase 1     │                  │  Phase 2     │
      │  Thinking    │ ── 追加思考 ──►  │  Action      │
      │  (tools=nil) │                  │  (tools 恢复) │
      └──────────────┘                  └──────┬───────┘
                                               │
                                    ┌──────────┴──────────┐
                                    ▼                     ▼
                               无 ToolCall             有 ToolCall
                               → 任务结束           Registry.Execute
                                                   → Observation 回灌
                                                   → 回到循环顶部
```

**单轮（Turn）数据流：**

```
[System Prompt + userPrompt]
        │
        ▼
 Phase 1: Generate(ctx, history, nil)      ← 剥夺工具，强制纯文本规划
        │  (思考 Trace 追加为 Assistant 消息)
        ▼
 Phase 2: Generate(ctx, history, tools)    ← 恢复工具，发起 ToolCall
        │
        ├── 无 ToolCall  → break，任务完成
        │
        └── 有 ToolCall  → registry.Execute(call)
                         → 结果封装为 User 消息(带 ToolCallID)
                         → append 到 history
                         → 进入下一轮
```

---

## 🚀 快速开始

### 环境要求

- **Go 1.26+**
- **无需任何 API Key**（本分支用 mock 驱动）

### 运行

```bash
git clone https://github.com/zlgit1/go-tiny-claw.git
cd go-tiny-claw
git checkout week01

go run ./cmd/claw
```

### 预期输出

```
[Engine] 引擎启动，锁定工作区: /path/to/go-tiny-claw
[Engine] 慢思考模式 (Thinking Phase): true
========== [Turn 1] 开始 ==========
[Engine][Phase 1] 剥夺工具访问权，强制进入慢思考与规划阶段...
🧠 [内部思考 Trace]: 【推理中】目标是检查文件。我需要先调用 bash 工具执行 ls ...
[Engine][Phase 2] 恢复工具挂载，等待模型采取行动...
🤖 [对外回复]: 我要执行我刚才计划的步骤了。
[Engine] 模型请求调用 1 个工具...
  -> 🛠️ 执行工具: bash, 参数: {"command": "ls -la"}
  -> ✅ 工具执行成功 (返回 51 字节)
========== [Turn 2] 开始 ==========
...
[Engine] 任务完成，退出循环。
```

可以看到引擎在 Turn 1 先思考、再调 `bash ls -la`，拿到 mock 返回的文件列表后，于 Turn 2 总结「我看到了 main.go，任务圆满完成」并退出——完整的 ReAct 闭环。

---

## 📂 项目结构

```
go-tiny-claw/                     (week01 分支)
├── cmd/
│   └── claw/
│       └── main.go               # mock 驱动的入口：mockProvider + mockRegistry
├── internal/
│   ├── schema/
│   │   └── message.go            # 数据模型：Message / ToolCall / ToolResult ...
│   ├── provider/
│   │   ├── interface.go          # LLMProvider 接口契约
│   │   ├── deepseek.go           # DeepSeek 实现（已就位，未接入 main）
│   │   ├── openai.go             # 智谱 GLM 的 OpenAI 兼容实现
│   │   └── claude.go             # 智谱 GLM 的 Claude 兼容实现
│   ├── tools/
│   │   └── registry.go           # Registry 接口契约（仅接口，无实现）
│   └── engine/
│       └── loop.go               # AgentEngine 主循环（Thinking/Action 双阶段）
├── go.mod / go.sum
├── a.txt / b.txt / c.txt         # 早期手动测试素材（模拟前端报错/后端响应等）
├── mock_log.txt                  # 模拟 OOM 的冗长日志素材
└── workspace/
    ├── answer.txt                # 测试素材：藏匿的「密码」
    └── legacy/v1/auth/config.txt # 测试素材：同上密码的另一种位置
```

> 仓库根的 `a.txt` / `b.txt` / `c.txt` / `mock_log.txt` / `workspace/legacy/...` 是早期用来手动喂给 Agent 做寻宝/排障练习的**测试素材**，不是工程代码，可在后续分支清理。

---

## 📦 模块说明

### `internal/schema`
全局数据模型，定义大模型与引擎之间传递消息的通用结构。`Role` 区分 `system` / `user` / `assistant`；`ToolCall.Arguments` 用 `json.RawMessage` 延迟解析，把参数反序列化的责任下放给具体工具。

### `internal/provider`
- `LLMProvider` 接口：只有一个 `Generate(ctx, messages, availableTools)` 方法。
- 三套实现已就位（DeepSeek 直连 HTTP、智谱 GLM 经 OpenAI/Claude 兼容 SDK），本分支 `main.go` 暂未调用。

### `internal/tools`
- `Registry` 接口：`GetAvailableTools()` 返回工具 Schema、`Execute(call)` 路由执行。
- **本分支仅定义接口**，无具体工具实现，`main.go` 用 `mockRegistry` 返回一个伪造的 `bash` 工具。真实的 `read_file` / `write_file` / `bash` 等工具在后续分支实现。

### `internal/engine`
`AgentEngine.Run(ctx, userPrompt)` 是核心心跳：
1. 硬编码 System Prompt + 用户输入初始化上下文；
2. **Phase 1 Thinking**：`Generate(ctx, history, nil)` 传 `nil` 工具，模型被迫只输出纯文本规划；
3. **Phase 2 Action**：`Generate(ctx, history, tools)` 恢复工具，模型发起 `ToolCall`；
4. 串行执行工具，结果带 `ToolCallID` 回灌为 `user` 消息；
5. 无 `ToolCall` 即任务完成，退出循环。

### `cmd/claw`
入口。`mockProvider` 内置两轮脚本化响应（第一轮调 `bash ls -la`，第二轮总结退出），`mockRegistry` 返回假的文件列表，用来在无 LLM 的情况下验证引擎循环。

---

## 🗺️ 能力边界与后续演进

Week 01 是**最小可运行骨架**，刻意保持极简。以下能力本分支**没有**，在后续分支逐步补全：

| 缺失能力 | 后续分支 |
|----------|----------|
| 真实 LLM 接入、真实工具实现（`read_file` 等） | Week 02 |
| 会话管理、上下文压缩、错误自愈、Plan 模式 | Week 03 |
| 死循环探测、飞书审批、子智能体 | Week 04 |
| 链路追踪、Token/成本核算、评测跑分 | Week 05 |

主线（含全部能力与完整 README）位于 `master` 分支。

---

## 📝 License

本项目为个人学习实践项目，目前**尚未声明开源许可证**。

<div align="center">

<sub> Week 01 — 骨架先立，契约先行。 </sub>

</div>
