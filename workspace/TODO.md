# 任务列表

- [x] 步骤1: 初始化 Go Module (`go.mod`)
- [x] 步骤2: 抽取 `response` 包 — 统一 JSON 响应封装
- [x] 步骤3: 改造 `ping.go` — 从 main 包改为 handler 包，移除 main 函数
- [x] 步骤4: 创建 `main.go` — 注册路由、启动服务
- [x] 步骤5: 编译运行验证 — `go build` 并通过 curl 测试

---

## 🔒 并发安全专项修复

- [x] **分析**: 发现 `main.go` 中 1000 个 goroutine 并发 `count++` 存在数据竞态（Data Race）
- [x] **修复**: 使用 `sync.Mutex` 保护 count 累加操作
- [x] **验证**: 运行 `go run -race main.go` 确认无竞态，结果稳定为 1000
