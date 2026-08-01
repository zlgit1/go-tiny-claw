# 极简 Go Web Server 项目搭建计划

## 目标
将现有的单文件 `ping.go` 扩展为标准 Go 项目结构，提供极简、高性能的 HTTP JSON API 服务。

## 架构设计

### 技术选型
- **语言**: Go (原生 net/http)
- **零第三方依赖**: 保持极简，不引入 gin/echo 等框架
- **响应格式**: 统一 JSON，含 `code` + `message` 字段（遵循 AGENTS.md）

### 项目结构
```
.
├── go.mod              # Go Module 定义
├── main.go             # 程序入口，注册路由、启动服务
├── ping.go             # 保留不动（已有文件，不可删除）
├── response/
│   └── response.go     # 统一 JSON 响应封装
└── handler/
    └── handler.go      # 路由处理器
```

### 设计原则
1. `ping.go` **保留不删**（禁忌事项），将其改造为 handler 包的一部分
2. 所有 API 响应都走 `response` 包的 `JSON()` 函数，确保格式统一
3. 错误信息全部使用中文（AGENTS.md 强制）
4. 新增路由: `/ping`(已有), `/health`, `/api/info`

## 🔒 并发安全分析 (2025-06-15 检查)

### 发现的问题

| 文件 | 风险等级 | 问题描述 |
|------|---------|----------|
| `main.go` | 🔴 **高** | 1000 个 goroutine 并发 `count++` 无锁保护，存在 Data Race |
| `response/response.go` | 🟢 低 | 纯函数，无共享状态，安全 |
| `auth.go` | 🟢 低 | 纯函数，无共享状态，安全 |
| `handler/ping.go` | 🟢 低 | 无共享状态，安全 |

### 修复方案
- `main.go`: 引入 `sync.Mutex`，对 `count++` 加锁
- 也可使用 `sync/atomic`，但 `Mutex` 更通用易懂，此处选用
