// cmd/claw/main.go
package main

import (
	"context"
	"log"
	"os"

	ctxpkg "github.com/zlgit1/go-tiny-claw/internal/context"
	"github.com/zlgit1/go-tiny-claw/internal/engine" // 导入监控包
	"github.com/zlgit1/go-tiny-claw/internal/provider"
	"github.com/zlgit1/go-tiny-claw/internal/schema"
	"github.com/zlgit1/go-tiny-claw/internal/tools"
)

func main() {
	if os.Getenv("DEEPSEEK_API_KEY") == "" {
		log.Fatal("请先导出 DEEPSEEK_API_KEY 环境变量")
	}

	workDir, _ := os.Getwd()
	workDir += "/workspace"
	modelName := "deepseek-v4-flash"

	// 1. 初始化真实的底层大脑
	llmProvider := provider.NewDeepSeekProvider(modelName)

	// 	sessionID := "test_observability_001"
	// 	sess := ctxpkg.GlobalSessionMgr.GetOrCreate(sessionID, workDir)

	// 	// 2. 核心拼装：用 Tracker 将真实的大脑包裹起来
	// 	trackedProvider := observability.NewCostTracker(realProvider, modelName, sess)

	// 	registry := tools.NewRegistry()
	// 	registry.Register(tools.NewBashTool(workDir))

	// 	// 3. 将被包裹的 Provider 注入给 Engine (Engine 毫不知情)
	// 	eng := engine.NewAgentEngine(trackedProvider, registry, false, false)
	// 	reporter := engine.NewTerminalReporter()

	// 	prompt := `请用 bash 帮我用 date 命令查一下现在的时间。`

	// 	log.Println("\n>>> 🚀 启动带仪表盘的可观测性测试...")
	// 	sess.Append(schema.Message{Role: schema.RoleUser, Content: prompt})

	// 	err := eng.Run(context.Background(), sess, reporter)
	// 	if err != nil {
	// 		log.Fatalf("引擎运行崩溃: %v", err)
	// 	}

	// 	log.Printf("\n================ 财务报表 ================\n")
	// 	log.Printf("会话 ID: %s\n", sess.ID)
	// 	log.Printf("总消耗 Input Tokens: %d\n", sess.TotalPromptTokens)
	// 	log.Printf("总消耗 Output Tokens: %d\n", sess.TotalCompletionTokens)
	// 	log.Printf("总计费用 (CNY): ¥%.6f\n", sess.TotalCostCNY)
	// 	log.Printf("==========================================\n")
	// }

	registry := tools.NewRegistry()
	registry.Register(tools.NewBashTool(workDir))
	registry.Register(tools.NewWriteFileTool(workDir))

	eng := engine.NewAgentEngine(llmProvider, registry, false, false)
	reporter := engine.NewTerminalReporter()
	sess := ctxpkg.GlobalSessionMgr.GetOrCreate("test_trace_001", workDir)

	// 触发一个跨工具类型的并发任务
	prompt := `
    为了加快执行速度，请你在一轮回复中，【同时并行】完成以下两件事：
    1. 使用 bash 工具执行 'sleep 2 && echo "系统环境检查完毕"'
    2. 使用 write_file 工具，在当前目录下创建一个 'trace_test.md'，内容写上 "测试并发的写入"。
    请确保你是分别调用两个不同的工具，不要试图把它们合并成一个命令！
    `
	sess.Append(schema.Message{Role: schema.RoleUser, Content: prompt})

	log.Println("\n>>> 🚀 启动带 Tracing 链路追踪的测试...")
	err := eng.Run(context.Background(), sess, reporter)
	if err != nil {
		log.Fatalf("引擎崩溃: %v", err)
	}
}
