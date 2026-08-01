<div align="center">

# 🐾 go-tiny-claw - Week 04

**安全防线与多智能体：死循环探测、人工审批、子智能体**

本分支给 Agent 装上三道驾驭防线--防止它在死胡同里空转、给高危操作加上人类把关、派出子智能体去探索而不污染主上下文--并引入可插拔的中间件链作为统一的安全策略层。

![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)
![Status](https://img.shields.io/badge/status-learning%20project-orange)
![Phase](https://img.shields.io/badge/phase-safety%20%26%20multi--agent-red)

</div>

---

## 📖 本分支简介

Week 03 的 Agent 会记忆了，但仍有两个致命问题：一是陷入死循环时会无脑重试空转烧钱，二是高危操作（`rm -rf`、覆盖源码）没有任何拦截。Week 04 用三件大事补上安全防线：

1. **Reminder 死循环探测** -- 用 MD5 指纹识别「连续 3 次以相同参数失败」，触发后注入严厉的 `[SYSTEM REMINDER]` 打断模型执念。
2. **飞书人工审批** -- `ApprovalManager` 让高危工具调用挂起协程、发飞书卡片，等人类回复 `approve` / `reject` 后才放行或拒绝。
3. **子智能体（Subagent）** -- `spawn_subagent` 工具派出一个只读探路者，跑完返回一段精炼摘要，万行代码探索化作一次轻量回传，不污染主上下文。

同时引入 **Registry 中间件链**（`Use(mw)`），让审批等安全策略可插拔地挂在工具执行前。

> ⚠️ 本分支的 `cmd/claw/main.go` 是一个**多智能体协同测试脚本**（硬编码 prompt 让主 Agent 派子智能体找密码）。文件顶部保留了约 140 行注释掉的旧实验代码（REPL + 审批中间件 + 死循环测试），供参考。

---

## 🆕 相对 Week 03 的增量

| 维度 | Week 03 | Week 04 |
|------|---------|---------|
| 死循环 | 无，模型可无脑重试 | `ReminderInjector` MD5 指纹探测，连续 3 次失败强制打断 |
| 高危操作 | 无拦截，YOLO 执行 | `ApprovalManager` + 中间件，挂起等飞书审批 |
| 探索任务 | 全部塞进主上下文 | `SubagentTool` 派只读探路者，返回摘要 |
| 工具执行 | 直接路由 | `Registry.Use(mw)` 中间件链，执行前先过策略层 |
| 飞书 | 编译失败（签名未同步） | 修复：`NewFeishuBot(eng, sess)` + approve/reject 口令 |
| Compactor 水位线 | 3000（测试值） | 20000（正常值） |
| 8MB 二进制 | `workspace/tiny-claw` 误提交 | ✅ 已删除 |

---

## ✨ 核心特性

| 能力 | 说明 |
|------|------|
| 🔁 **死循环探测** | `ReminderInjector` 对 `工具名+参数` 取 MD5 指纹，连续失败计数；达 3 次注入 `[SYSTEM REMINDER]` 作为 User 消息（最高近因权重），强制模型跳出局部执念；成功则清零计数 |
| 🛡️ **人工审批** | `IsDangerousCommand` 正则黑名单（`rm -r`/`sudo`/`drop`/覆盖 `.go`）；命中后 `WaitForApproval` 挂起协程发飞书卡片，`ResolveApproval` 由 webhook 唤醒；拒绝理由回灌模型 |
| 🔌 **中间件链** | `Registry.Use(MiddlewareFunc)` 挂载全局中间件，`Execute` 前依次运行，任一拒绝即返回 `IsError` 结果；安全策略与工具实现解耦 |
| 🤖 **子智能体** | `SubagentTool` 通过 `AgentRunner` 接口拉起 `RunSub`--一次性、无外部 Session、最多 10 轮、仅持只读 Registry、强制关闭慢思考；不调工具即汇报退出 |
| 🧠 **双阶段 + 并行** | 沿用 Thinking/Action 双阶段与 goroutine 并行工具执行 |
| 🩺 **错误自愈** | 沿用 Week 03 的 `RecoveryManager` |
| ✅ **content 非空保证** | 组装 assistant 消息时确保 content 非空（DeepSeek API 要求），空则填「调用工具中...」 |

---

## 🏗️ 架构概览

```
                ┌─────────────────────────────────────────┐
  Session ───►  │            AgentEngine                  │
                │  (compactor + recovery + reminder)      │
                └──────────────────┬──────────────────────┘
                                   │
                    Phase 1 Thinking ──► Phase 2 Action
                                   │            │
                                   │     无 ToolCall -> 结束
                                   │            │
                                   │     有 ToolCall -> 并行执行
                                   │            │
                                   │            ▼
                                   │     ┌──────────────┐
                                   │     │  Registry    │
                                   │     │  .Execute()  │
                                   │     └──────┬───────┘
                                   │            │
                                   │     ┌──────▼───────┐
                                   │     │ Middleware链 │ ── 命中黑名单?
                                   │     │ (approval)   │    是 -> 挂起飞书审批
                                   │     └──────┬───────┘    否 -> 放行
                                   │            │
                                   │     ┌──────▼───────┐
                                   │     │  Recovery    │ ── 失败则注入救援指南
                                   │     └──────┬───────┘
                                   │            │
                                   │            ▼
                                   │     Observation 持久化到 Session
                                   │            │
                                   │     ┌──────▼───────┐
                                   └─────│  Reminder    │ ── 连续3次相同失败?
                                         │  CheckAndInject │ 是 -> 注入打断指令
                                         └──────────────┘    下一轮 ──┘
```

**子智能体（RunSub）的隔离设计**：独立的 `contextHistory`（不写入主 Session）、专属只读 Registry、10 轮硬上限、退出即返回纯文本摘要给主 Agent。

---

## 🚀 快速开始

### 环境要求

- **Go 1.26+**
- **`DEEPSEEK_API_KEY`**（必填）

### 构建与运行

```bash
git clone https://github.com/zlgit1/go-tiny-claw.git
cd go-tiny-claw
git checkout week04

export DEEPSEEK_API_KEY="sk-xxxxxxxx"

# 构建入口用 ./cmd/claw（根目录 server.go/helloworld.go 遗留冲突，见「已知问题」）
go run ./cmd/claw
```

### 预期行为

`main.go` 硬编码了一个**多智能体协同测试场景**：命令主 Agent 派子智能体去工作区子目录里找名为 `config.txt` 的文件、取出「核心密码」，子智能体汇报后，主 Agent 亲自用 `write_file` 把密码写入 `answer.txt`。运行后可观察到：

1. 主 Agent 调用 `spawn_subagent` 工具
2. 子智能体（带 `[Subagent]` 标记）用 `bash find` 搜索 config.txt 并读取
3. 子智能体返回密码摘要给主 Agent
4. 主 Agent 用 `write_file` 写入 answer.txt，任务完成

> `main.go` 顶部注释掉的代码还包含两个可参考的实验：飞书审批中间件挂载示例、死循环干预测试（命令 Agent 原样重试 read_file 5 次，观察 Reminder 打断）。

---

## 📂 项目结构

```
go-tiny-claw/                     (week04 分支)
├── cmd/
│   └── claw/
│       └── main.go               # 多智能体协同测试（顶部含注释的审批/死循环实验）
├── internal/
│   ├── schema/message.go
│   ├── provider/                 # DeepSeek/OpenAI/Claude
│   ├── tools/
│   │   ├── registry.go           # ★ 新增 Middleware 链（Use/中间件拦截）
│   │   ├── subagent.go           # ★ 新增子智能体工具 + AgentRunner 接口
│   │   ├── read_file / write_file / bash / edit_file.go
│   ├── engine/
│   │   ├── loop.go               # 接入 Reminder + RunSub 子循环
│   │   ├── reminder.go           # ★ 新增死循环探测
│   │   ├── reporter.go / terminal_reporter.go
│   ├── context/                  # Session/Compactor/Composer/Recovery/Skill（沿用 Week 03）
│   └── feishu/
│       ├── bot.go                # 修复签名 + 接入 approve/reject 口令
│       └── approval.go           # ★ 新增人工审批管理器
├── workspace/                    # 示例靶机
└── go.mod / go.sum / .gitignore
```

---

## 📦 新增模块说明

### `internal/engine/reminder.go` - 死循环探测
`ReminderInjector` 维护 `consecutiveFailures map[指纹]int`。`CheckAndInject(lastToolCall, lastResult)`：
- 工具成功 -> 清空所有计数器
- 工具失败 -> 该指纹计数 +1
- 计数 ≥ 3 -> 返回一条严厉的 `[SYSTEM REMINDER]` User 消息，要求模型停止猜测、改变策略或向人类求助

### `internal/feishu/approval.go` - 人工审批
- `IsDangerousCommand(toolName, args)`：正则黑名单（`rm -r`/`sudo`/`drop`/`>.*\.go`），仅对 `bash`/`write_file`/`edit_file` 生效
- `ApprovalManager.WaitForApproval(taskID, ...)`：创建 channel 挂起当前协程，发飞书卡片，阻塞等待结果
- `ResolveApproval(taskID, allowed, reason)`：飞书 webhook 回调唤醒挂起的协程
- `GlobalApprovalMgr` 全局单例，在中间件与 webhook 间共享 pending 任务

### `internal/tools/subagent.go` - 子智能体
`SubagentTool` 持有 `AgentRunner` 接口（打破 tools->engine 循环依赖）和只读 Registry。`Execute` 调用 `RunSub`，返回 `【子智能体探索报告】` 摘要。

### `internal/tools/registry.go` - 中间件链
新增 `MiddlewareFunc` 类型与 `Use(mw)` 方法。`Execute` 在路由查找后、工具执行前，依次运行所有中间件，任一返回 `allowed=false` 即拦截并返回带拒绝理由的 `IsError` 结果。

### `internal/engine/loop.go` - RunSub 子循环
`RunSub` 是专为子智能体的受限循环：独立 contextHistory、只读 Registry、最多 10 轮、强制关闭慢思考；不调工具即返回纯文本汇报。

---

## ⚠️ 已知问题

1. **根 package 编译冲突**：根目录 `server.go` / `helloworld.go` 都声明 `func main()`，导致 `go build ./...` 失败。**用 `go build ./cmd/claw` 构建**。`master` 分支已清理。

2. **`main.go` 顶部大量注释代码**：约 140 行被注释的旧实验代码（REPL + 审批中间件 + 死循环测试）保留作参考，实际生效的是底部的多智能体协同测试。

---

## 🗺️ 能力边界与后续演进

| 缺失能力 | 后续分支 |
|----------|----------|
| 链路追踪（Span 树）、Token/成本核算、评测跑分 | Week 05 |

> 本分支的 `Session` 尚无 Token/Cost 统计字段，`loop` 也还没有链路追踪埋点--这些可观测性能力在 Week 05 引入。

主线（含全部能力、已清理遗留、含完整 README）位于 `master` 分支。

---

## 📝 License

本项目为个人学习实践项目，目前**尚未声明开源许可证**。

<div align="center">

<sub> Week 04 - 三道防线立起，Agent 不再蛮干。 </sub>

</div>
