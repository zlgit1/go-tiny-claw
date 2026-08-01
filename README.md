<div align="center">

# 🐾 go-tiny-claw - Week 05

**可观测性与评测：链路追踪、Token 成本核算、评测跑分**

本分支是系列的收官--给 Agent 装上「仪表盘」。每一次 LLM 调用、每一个工具执行的耗时与调用链都可回放追踪，每个会话的 Token 消耗与费用都有账本，还能用沙箱化的测试集自动度量 Agent 的能力。

![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)
![Status](https://img.shields.io/badge/status-learning%20project-orange)
![Phase](https://img.shields.io/badge/phase-observability-success)

</div>

---

## 📖 本分支简介

前四周把 Agent 从骨架养到了「能干活、有记忆、有防线」，但还缺一环：**可度量**。出问题时无法回放哪一步慢了、哪次调用爆了 Token；也无法系统性地验证 Agent 到底能不能干好某类任务。Week 05 用三件大事补上：

1. **链路追踪（Tracing）** -- 自研轻量 Span 树，在 `loop` 与 `registry` 的 5 个关键点埋点，任务结束时导出 JSON 到 `.claw/traces/`，可回放整个调用链与耗时。
2. **Token/成本核算（CostTracker）** -- 用装饰器模式包裹 Provider，透明地记录每次调用的输入/输出 Token 与费用，累加到 Session 账本。
3. **评测跑分（Eval Harness）** -- `BenchmarkRunner` 为每个用例建物理隔离沙箱，跑 Setup -> Agent 执行 -> Validate 脚本断言，产出成功率与成本报表。

> ⚠️ 本分支的 `cmd/claw/main.go` 是一个 **Tracing 链路追踪测试脚本**（硬编码 prompt 触发一轮并发调用 `bash` + `write_file`，用于观察 trace 树中两个工具 Span 平行挂在 Turn 节点下）。`CostTracker` 的用法在文件顶部的注释代码里演示，实际生效的入口未启用它（但 `tracker.go` 已完整实现，`cmd/bench` 与 `master` 分支均有使用）。

---

## 🆕 相对 Week 04 的增量

| 维度 | Week 04 | Week 05 |
|------|---------|---------|
| 调用链可观测 | 只有 log | Span 树埋点 + JSON 导出回放 |
| 成本核算 | 无 | `CostTracker` 装饰器 + Session 账本 |
| 能力验证 | 手动跑 prompt 看 | `BenchmarkRunner` 沙箱 + 脚本断言 |
| 入口 | `cmd/claw` | + `cmd/bench` 评测入口 |
| schema | 无 Usage | `Message.Usage`（Prompt/Completion Tokens） |
| Session | 无计费字段 | `TotalPromptTokens` / `TotalCompletionTokens` / `TotalCostCNY` |

---

## ✨ 核心特性

| 能力 | 说明 |
|------|------|
| 📊 **链路追踪** | `StartSpan` 通过 context 级联父子关系，自动构建树；并发工具的 Span 平行挂在同一 Turn 下；`ExportTraceToFile` 导出美化 JSON |
| 📍 **5 处埋点** | Root(Agent.Run) -> Turn-N -> LM.Thinking / LLM.Action -> Tool.Execute；defer 保证异常退出也结束 Span |
| 💰 **成本核算** | `CostTracker` 实现 `LLMProvider` 接口，包裹真实 Provider 透明注入；按 `PricingModel` 计价表算费，累加到 `Session.RecordUsage` |
| 🧪 **沙箱评测** | 每个用例独立目录隔离；`SetupScript` 准备靶机 -> Agent 执行 -> `ValidateScript`（`exit 0` 即通过）；输出成功率、耗时、成本报表 |
| 🔁 **死循环探测 / 审批 / 子智能体** | 沿用 Week 04 全部安全防线 |
| 🧠 **Session / Compactor / Plan / Recovery** | 沿用 Week 03 全部上下文管理 |

---

## 🏗️ 架构概览（Tracing 埋点）

```
Agent.Run                          [Span: Agent.Run]  ← Root，defer 导出 JSON
  │
  ├─ Turn-1                        [Span: Turn-1]
  │    ├─ LLM.Thinking             [Span: LLM.Thinking]
  │    ├─ LLM.Action               [Span: LLM.Action]
  │    ├─ Tool.Execute (bash)      [Span: Tool.Execute] ┐ 并发，平行挂在 Turn 下
  │    └─ Tool.Execute (write_file)[Span: Tool.Execute] ┘
  │
  └─ Turn-2 ...
```

`CostTracker` 的透明包裹：

```
Engine ──► CostTracker.Generate ──► DeepSeek.Generate
                │  记录 Token/费用
                └──► Session.RecordUsage (累加账本)
```

---

## 🚀 快速开始

### 环境要求

- **Go 1.26+**
- **`DEEPSEEK_API_KEY`**（必填）

### 入口一：Tracing 测试（cmd/claw）

```bash
git clone https://github.com/zlgit1/go-tiny-claw.git
cd go-tiny-claw
git checkout week05

export DEEPSEEK_API_KEY="sk-xxxxxxxx"

# 构建入口用 ./cmd/claw（根目录 server.go/helloworld.go 遗留冲突，见「已知问题」）
go run ./cmd/claw
```

`main.go` 硬编码 prompt 让 Agent 在一轮内并行执行 `bash 'sleep 2 && echo ...'` 与 `write_file trace_test.md`，运行后可在 `workspace/.claw/traces/trace_test_trace_001_*.json` 看到完整的 Span 树回放。

### 入口二：评测跑分（cmd/bench）

```bash
export DEEPSEEK_API_KEY="sk-xxxxxxxx"
go run ./cmd/bench
```

`cmd/bench` 内置两个示例用例：

| 用例 | 靶机 | 断言 |
|------|------|------|
| `test_001_edit` | 生成含错误版本号的 `config.json` | 要求 Agent 用 `edit_file` 改版本号，`grep` 验证 |
| `test_002_code_gen` | 生成 `math.go`（Multiply 函数） | 要求 Agent 写出 `math_test.go`，`go test` 验证通过 |

每个用例在独立沙箱目录执行，最后输出成功率与总成本报表。

---

## 📂 项目结构

```
go-tiny-claw/                     (week05 分支)
├── cmd/
│   ├── claw/main.go              # Tracing 测试（触发并发工具，看 trace 树）
│   └── bench/main.go             # ★ 新增：评测跑分入口
├── internal/
│   ├── schema/message.go         # +Usage 结构（Prompt/Completion Tokens）
│   ├── provider/
│   │   ├── deepseek.go           # 提取 Usage 回填到 Message
│   │   ├── openai.go / claude.go / interface.go
│   ├── tools/
│   │   ├── registry.go           # Execute 内加 Span 埋点
│   │   ├── subagent.go / read_file / write_file / bash / edit_file.go
│   ├── engine/
│   │   ├── loop.go               # 5 处 Tracing 埋点 + RunSub
│   │   ├── reminder.go / reporter.go / terminal_reporter.go
│   ├── context/
│   │   ├── session.go            # +Token/Cost 字段 + RecordUsage
│   │   ├── compactor / composer / recovery / skill.go
│   ├── observability/            # ★ 本周新增
│   │   ├── trace.go              # Span 树 + StartSpan/EndSpan + JSON 导出
│   │   └── tracker.go            # CostTracker 装饰器 + PricingModel 计价
│   ├── eval/                     # ★ 本周新增
│   │   └── benchmark.go          # TestCase + BenchmarkRunner 沙箱评测
│   └── feishu/
│       ├── bot.go / approval.go
├── workspace/                    # 示例靶机
└── go.mod / go.sum / .gitignore
```

---

## 📦 新增模块说明

### `internal/observability/trace.go` - 链路追踪
`Span` 结构含 `Name` / `StartTime` / `EndTime` / `DurationMs` / `Attributes` / `Children`（`sync.Mutex` 保护并发子 Span 写入）。`StartSpan(ctx, name)` 从 context 取父 Span 并挂载自己，返回衍生 context；`ExportTraceToFile` 把根 Span 序列化为美化 JSON 落盘到 `<workDir>/.claw/traces/`。

### `internal/observability/tracker.go` - 成本核算
`CostTracker` 持有 `nextProvider` + `modelName` + `session`，实现 `LLMProvider.Generate`：调用底层 Provider 后，从响应的 `Usage` 按计价表算出费用，`session.RecordUsage` 累加，并打印仪表盘日志。**装饰器模式让 Engine 毫无感知**。

### `internal/eval/benchmark.go` - 评测跑分
`TestCase` 含 `SetupScript` / `TaskPrompt` / `ValidateScript` / `MaxTurns`。`BenchmarkRunner.runSingleTest` 为每个用例创建隔离沙箱目录，跑 Setup -> 组装带 `CostTracker` 的 Engine -> Agent 执行 -> 跑 Validate 脚本（`exit 0` 即通过），汇总成功率与成本。

### loop.go 的 5 处埋点
1. `Agent.Run` Root Span（defer 导出 trace）
2. `Turn-N` 每轮循环
3. `LLM.Thinking` 慢思考调用
4. `LLM.Action` 行动调用
5. `Tool.Execute`（在 registry 内，并发工具 Span 平行挂在 Turn 下）

---

## ⚠️ 已知问题

1. **根 package 编译冲突**：根目录 `server.go` / `helloworld.go` 都声明 `func main()`，导致 `go build ./...` 失败。**用 `go build ./cmd/claw ./cmd/bench` 构建**。`master` 分支已清理。

2. **`cmd/claw/main.go` 顶部注释代码**：约 40 行被注释的 CostTracker/财务报表演示代码保留作参考；实际生效的是底部的 Tracing 测试（未启用 CostTracker，但 `tracker.go` 已完整实现）。

---

## 🗺️ 系列收官

Week 05 是本系列的最后一个分支。至此 Agent 具备了完整能力栈：

| 周次 | 主题 |
|------|------|
| Week 01 | 骨架与契约（mock 跑通 ReAct 循环） |
| Week 02 | 真实工具系统 + 并行执行 + 双展现层 |
| Week 03 | 上下文管理（Session/Compactor/Recovery/Plan） |
| Week 04 | 安全防线（死循环探测/人工审批/子智能体） |
| **Week 05** | **可观测性（Tracing/CostTracker/Eval）** |

主线（含全部能力、已清理根目录遗留脚手架、`cmd/claw` 重构为 flag CLI 并启用 CostTracker、含完整 README）位于 `master` 分支。

---

## 📝 License

本项目为个人学习实践项目，目前**尚未声明开源许可证**。

<div align="center">

<sub> Week 05 - Agent 戴上仪表盘，所行皆可度量。系列收官。 </sub>

</div>
