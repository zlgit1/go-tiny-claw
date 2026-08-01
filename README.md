<div align="center">

# 🐾 go-tiny-claw - Week 02

**从 mock 骨架到真实可用 Agent：工具系统 + 并行执行 + 双展现层**

本分支把 Week 01 那个用 mock 跑通心跳的骨架，升级成一个能接真实大模型、挂真实工具、并行执行、在终端或飞书里对话的可用 Agent。

![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)
![Status](https://img.shields.io/badge/status-learning%20project-orange)
![Phase](https://img.shields.io/badge/phase-real%20agent-brightgreen)

</div>

---

## 📖 本分支简介

Week 01 只验证了「循环能转」，Week 02 让循环「真的能干活」。三件大事：

1. **工具系统落地** -- `Registry` 从纯接口变成有 `BaseTool` 抽象 + `registryImpl` 实现的完整路由层，并内置 4 个真实工具：`read_file` / `write_file` / `bash` / `edit_file`。
2. **循环接入现实** -- `main.go` 抛弃 mock，接入真实 DeepSeek，并在单轮内把多个 `ToolCall` 从**串行**升级为**并行**（goroutine + WaitGroup）。
3. **双展现层** -- 抽象出 `Reporter` 接口，引擎对外的状态流可同时驱动**终端彩色输出**与**飞书消息卡片**，同一份逻辑无缝切换 UI。

> 本分支的 Agent 已可在终端 REPL 里连续对话，或挂在飞书群里接收 @消息。但还没有会话管理、上下文压缩、审批拦截等驾驭工程--那是 Week 03–05 的事。

---

## 🆕 相对 Week 01 的增量

| 维度 | Week 01 | Week 02 |
|------|---------|---------|
| 大脑 | mockProvider 脚本化 | 真实 DeepSeek API |
| 工具 | `Registry` 仅接口，mock 假 bash | `BaseTool` + 实现 + 4 个真实工具 |
| 执行 | 串行逐个执行 | goroutine 并行 + WaitGroup 聚合 |
| 输出 | `log.Printf` 硬编码 | `Reporter` 接口（终端 / 飞书） |
| 交互 | 单次 prompt 即退出 | 终端 REPL + 飞书 WebSocket 双模式 |
| 依赖 | 仅 LLM SDK | + 飞书 `larksuite/oapi-sdk-go`、`gorilla/websocket` |

---

## ✨ 核心特性

| 能力 | 说明 |
|------|------|
| 🛠️ **真实工具系统** | `BaseTool` 接口（Name/Definition/Execute）+ `Registry` O(1) map 路由，幻觉工具名直接报错回灌模型 |
| ⚡ **并行工具执行** | 单轮内多个 `ToolCall` 各开 goroutine，预分配切片按索引写结果（无锁），最后按序聚合 |
| 📡 **Reporter 抽象** | 4 个回调：`OnThinking` / `OnToolCall` / `OnToolResult` / `OnMessage`，终端与飞书各一实现 |
| 💬 **终端 REPL** | `bufio.Scanner` 循环读取，支持 `exit`/`quit`，`SIGINT`/`SIGTERM` 优雅退出 |
| 🤖 **飞书 Bot** | WebSocket 长连接（无需公网 IP、自动重连），每条消息独立 goroutine 处理 |
| 🧠 **Thinking-Action 双阶段** | 沿用 Week 01 的 ReAct 心跳，Phase 1 剥夺工具强制规划，Phase 2 恢复工具执行 |
| 🛡️ **工具驾驭底线** | bash 工具内置 30s 超时、工作区绑定、错误原样回传自愈、输出截断防 OOM |

---

## 🏗️ 架构概览

```
                ┌─────────────────────────────┐
  用户输入 ───► │        AgentEngine          │
  (终端/飞书)   │  Run(ctx, prompt, reporter) │
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
                               -> 任务结束           ┌──┴──┐
                                                    ▼     ▼  ... (并行)
                                                 goroutine per ToolCall
                                                    │
                              Reporter ◄─────────────┤ OnToolCall / OnToolResult
                                                    │
                                                    ▼
                                              Registry.Execute
                                              -> Observation 回灌
                                              -> 下一轮
```

**并行执行的关键设计**：预分配 `observationMsgs := make([]schema.Message, len(toolCalls))`，每个 goroutine 只写自己的索引位，无需加锁；`wg.Wait()` 后按序聚合到上下文。

---

## 🚀 快速开始

### 环境要求

- **Go 1.26+**
- **`DEEPSEEK_API_KEY`**（必填，真实 LLM 调用）

### 构建与运行

```bash
git clone https://github.com/zlgit1/go-tiny-claw.git
cd go-tiny-claw
git checkout week02

export DEEPSEEK_API_KEY="sk-xxxxxxxx"

# 注意：构建入口用 ./cmd/claw，不要用 ./...
# （根目录 server.go/helloworld.go 是遗留脚手架，二者都声明了 main() 会冲突；
#  这两个文件在 master 分支已被清理）
go run ./cmd/claw
```

启动后进入交互式 REPL：

```
🖥️  Go Tiny Claw 终端模式 (输入 exit 或 quit 退出)
─────────────────────────────────────────────────

> 帮我读一下 workspace 下的文件，并总结它们的内容
```

### 飞书模式（可选）

设置飞书环境变量后，启动时会**同时**后台运行飞书 WebSocket 长连接：

```bash
export FEISHU_APP_ID="cli_xxxxxxxx"
export FEISHU_APP_SECRET="xxxxxxxx"
go run ./cmd/claw
```

| 环境变量 | 必填 | 用途 |
|----------|------|------|
| `DEEPSEEK_API_KEY` | ✅ | DeepSeek 大模型调用 |
| `FEISHU_APP_ID` | 飞书模式 | 飞书应用 ID |
| `FEISHU_APP_SECRET` | 飞书模式 | 飞书应用密钥 |

> ⚠️ `workDir` 当前硬编码为 `./workspace`，Agent 的工具操作都被限制在该目录下。

---

## 📂 项目结构

```
go-tiny-claw/                     (week02 分支)
├── cmd/
│   └── claw/
│       └── main.go               # 真实 DeepSeek + 4 工具 + REPL + 飞书双模式
├── internal/
│   ├── schema/
│   │   └── message.go            # 数据模型（沿用 Week 01）
│   ├── provider/
│   │   ├── interface.go          # LLMProvider 接口
│   │   ├── deepseek.go           # DeepSeek 实现（main 实际接入）
│   │   ├── openai.go             # 智谱 GLM 的 OpenAI 兼容实现
│   │   └── claude.go             # 智谱 GLM 的 Claude 兼容实现
│   ├── tools/
│   │   ├── registry.go           # BaseTool 接口 + Registry 实现
│   │   ├── read_file.go          # 读文件工具
│   │   ├── write_file.go         # 写文件工具
│   │   ├── edit_file.go          # 模糊替换工具
│   │   └── bash.go               # bash 命令工具
│   ├── engine/
│   │   ├── loop.go               # 主循环（并行工具执行 + Reporter 回调）
│   │   ├── reporter.go           # Reporter 接口
│   │   └── terminal_reporter.go  # 终端彩色输出实现
│   └── feishu/
│       └── bot.go                # 飞书 Bot + FeishuReporter
├── go.mod / go.sum
├── .gitignore
├── server.go / helloworld.go     # ⚠️ 遗留脚手架（与 main 冲突，master 已清理）
└── workspace/                    # Agent 工作区（工具操作的物理边界）
```

---

## 📦 模块说明

### `internal/tools` - 工具系统
- **`BaseTool` 接口**：`Name()` / `Definition()` / `Execute(ctx, args)`，所有工具的统一契约。
- **`Registry` 实现**：`map[string]BaseTool` O(1) 路由；工具未找到时返回 `IsError=true` 的结果回灌模型，利用模型自纠错。
- **4 个内置工具**（均绑定 `workDir` 物理边界）：

| 工具 | 能力 |
|------|------|
| `read_file` | 读取工作区内文件 |
| `write_file` | 创建/覆盖文件（path + content） |
| `edit_file` | 模糊查找并替换代码片段 |
| `bash` | 执行任意 bash 命令，支持管道与 `&&` |

### `internal/engine` - 引擎
- **`loop.go`**：`Run(ctx, userPrompt, reporter)`。沿用 Thinking/Action 双阶段；**并行执行**多个 ToolCall（goroutine + WaitGroup + 预分配切片无锁写入）；在 4 个时机触发 Reporter 回调。
- **`reporter.go`**：`Reporter` 接口，4 个回调方法，让引擎与展现层解耦。
- **`terminal_reporter.go`**：终端彩色输出，参数截断 150 字符保持清爽。

### `internal/feishu` - 飞书集成
- **`FeishuBot`**：WebSocket 长连接（推荐，自动重连）+ HTTP Webhook 两种接入方式；每条消息独立 goroutine，不阻塞回调。
- **`FeishuReporter`**：实现 `Reporter` 接口，把引擎状态格式化为飞书文本消息；编译时 `var _ engine.Reporter = (*FeishuReporter)(nil)` 做类型保证。

### bash 工具的 4 道驾驭底线
1. **时间预算**：`context.WithTimeout(30s)`，防止 `top` / 常驻服务卡死进程
2. **工作区绑定**：`cmd.Dir = workDir`，命令在指定目录执行
3. **错误原样回传**：不返回 Go `error` 阻断循环，而是把 stderr 拼成字符串返回，让模型自己分析报错并重试
4. **长度截断**：输出超 8000 字节截断，防 OOM

---

## 🗺️ 能力边界与后续演进

Week 02 让 Agent「能干活」了，但驾驭工程还很薄。以下能力本分支**没有**，在后续分支补全：

| 缺失能力 | 后续分支 |
|----------|----------|
| 会话管理、上下文压缩、错误自愈注入、Plan 模式 | Week 03 |
| 死循环探测、飞书人工审批、子智能体 | Week 04 |
| 链路追踪、Token/成本核算、评测跑分 | Week 05 |

主线（含全部能力、已清理遗留脚手架、含完整 README）位于 `master` 分支。

---

## 📝 License

本项目为个人学习实践项目，目前**尚未声明开源许可证**。

<div align="center">

<sub> Week 02 - 骨架长出血肉，工具开始发力。 </sub>

</div>
