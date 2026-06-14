// cmd/claw/main.go
// package main

// import (
// 	"bufio"
// 	"context"
// 	"fmt"
// 	"log"
// 	"os"
// 	"os/signal"
// 	"strings"
// 	"syscall"

// 	ctxpkg "github.com/zlgit1/go-tiny-claw/internal/context"
// 	"github.com/zlgit1/go-tiny-claw/internal/engine"
// 	"github.com/zlgit1/go-tiny-claw/internal/feishu"
// 	"github.com/zlgit1/go-tiny-claw/internal/provider"
// 	"github.com/zlgit1/go-tiny-claw/internal/schema"
// 	"github.com/zlgit1/go-tiny-claw/internal/tools"
// )

// func main() {
// 	if os.Getenv("DEEPSEEK_API_KEY") == "" {
// 		log.Fatal("请先导出 DEEPSEEK_API_KEY 环境变量")
// 	}
// 	workDir, _ := os.Getwd()
// 	workDir += "/workspace"

// 	llmProvider := provider.NewDeepSeekProvider("deepseek-v4-flash")

// 	registry := tools.NewRegistry()
// 	registry.Register(tools.NewReadFileTool(workDir))
// 	registry.Register(tools.NewWriteFileTool(workDir))
// 	registry.Register(tools.NewBashTool(workDir))
// 	registry.Register(tools.NewEditFileTool(workDir))

// 	eng := engine.NewAgentEngine(llmProvider, registry, false, false)

// 	// 假设一个bot绑定一个session
// 	sessionID := "test_command_intercept_001"
// 	sess := ctxpkg.GlobalSessionMgr.GetOrCreate(sessionID, workDir)

// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()

// 	// 飞书模式：有环境变量时创建 bot（必须在注册中间件之前，因为中间件需要 bot.Reporter()）
// 	var bot *feishu.FeishuBot
// 	if os.Getenv("FEISHU_APP_ID") != "" && os.Getenv("FEISHU_APP_SECRET") != "" {
// 		bot = feishu.NewFeishuBot(eng, sess)
// 	}

// 	// 【核心注入】注册安全拦截 Middleware
// 	registry.Use(func(ctx context.Context, call schema.ToolCall) (bool, string) {
// 		argsStr := string(call.Arguments)

// 		// 检查是否命中高危特征库
// 		if feishu.IsDangerousCommand(call.Name, argsStr) {
// 			taskID := call.ID // 使用大模型生成的唯一 ToolCallID 作为 TaskID

// 			// 挂起当前协程，发送消息给飞书，死死等待人类的审批！
// 			var reporter *feishu.FeishuReporter
// 			if bot != nil {
// 				reporter = bot.Reporter()
// 			}
// 			allowed, reason := feishu.GlobalApprovalMgr.WaitForApproval(taskID, call.Name, argsStr, reporter)

// 			if !allowed {
// 				return false, reason // 拒绝，将理由传回给大模型
// 			}
// 			return true, "" // 同意，放行底层工具
// 		}

// 		// 没命中黑名单，直接 YOLO 放行
// 		return true, ""
// 	})

// 	sigChan := make(chan os.Signal, 1)
// 	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

// 	// 飞书模式：后台启动 WebSocket
// 	if bot != nil {
// 		go func() {
// 			log.Println("🚀 飞书 WebSocket 长连接模式启动...")
// 			if err := bot.StartWebSocket(ctx); err != nil {
// 				log.Printf("❌ WebSocket 连接失败: %v\n", err)
// 			}
// 		}()
// 	}
// 	// 终端交互模式：始终启动
// 	fmt.Println("🖥️  Go Tiny Claw 终端模式 (输入 exit 或 quit 退出)")
// 	fmt.Println("─────────────────────────────────────────────────")

// 	reporter := engine.NewTerminalReporter()
// 	scanner := bufio.NewScanner(os.Stdin)

// 	for {
// 		fmt.Print("\n> ")

// 		inputCh := make(chan string, 1)
// 		go func() {
// 			if scanner.Scan() {
// 				inputCh <- scanner.Text()
// 			} else {
// 				inputCh <- ""
// 			}
// 		}()

// 		select {
// 		case <-sigChan:
// 			fmt.Println("\n📴 再见！")
// 			cancel()
// 			return
// 		case input := <-inputCh:
// 			input = strings.TrimSpace(input)
// 			if input == "" {
// 				continue
// 			}
// 			if input == "exit" || input == "quit" {
// 				fmt.Println("📴 再见！")
// 				cancel()
// 				return
// 			}

// 			runCtx, runCancel := context.WithCancel(ctx)
// 			done := make(chan struct{})

// 			go func() {
// 				defer close(done)
// 				if err := eng.Run(runCtx, sess, reporter); err != nil && runCtx.Err() == nil {
// 					log.Printf("❌ Agent 运行失败: %v\n", err)
// 				}
// 			}()

// 			select {
// 			case <-done:
// 				runCancel()
// 			case <-sigChan:
// 				runCancel()
// 				<-done
// 				fmt.Println("\n📴 再见！")
// 				cancel()
// 				return
// 			}
// 		}
// 	}
// }

// cmd/claw/main.go
package main

import (
	"context"
	"log"
	"os"

	ctxpkg "github.com/zlgit1/go-tiny-claw/internal/context"
	"github.com/zlgit1/go-tiny-claw/internal/engine"
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

	llmProvider := provider.NewDeepSeekProvider("deepseek-v4-flash")

	// 	registry := tools.NewRegistry()
	// 	registry.Register(tools.NewReadFileTool(workDir))
	// 	registry.Register(tools.NewWriteFileTool(workDir))
	// 	registry.Register(tools.NewBashTool(workDir))
	// 	registry.Register(tools.NewEditFileTool(workDir))

	// 	// 关闭 Plan 模式，让它在死胡同里专注地展示挣扎过程
	// 	eng := engine.NewAgentEngine(llmProvider, registry, false, false)
	// 	reporter := engine.NewTerminalReporter()

	// 	sessionID := "test_doom_loop_001"
	// 	sess := ctxpkg.GlobalSessionMgr.GetOrCreate(sessionID, workDir)

	// 	prompt := `
	//     帮我读取当前目录下的 secret_key.txt。
	//     注意：我们的文件系统现在非常不稳定，经常报 File Not Found。
	//     如果报错了，请你【千万不要改变参数】，直接原样再次调用 read_file 尝试，直到成功或连续重试 5 次为止。
	//     `

	// 	log.Println("\n>>> 🚀 启动死循环干预测试...")
	// 	sess.Append(schema.Message{Role: schema.RoleUser, Content: prompt})

	// 	err := eng.Run(context.Background(), sess, reporter)
	// 	if err != nil {
	// 		log.Fatalf("引擎运行崩溃: %v", err)
	// 	}
	// }

	reporter := engine.NewTerminalReporter()

	// 【防御沙箱】为子智能体准备受限的只读注册表
	readOnlyRegistry := tools.NewRegistry()
	readOnlyRegistry.Register(tools.NewReadFileTool(workDir))
	readOnlyRegistry.Register(tools.NewBashTool(workDir)) // 允许简单的 grep 等搜索操作

	// 为主智能体准备全功能注册表
	mainRegistry := tools.NewRegistry()
	mainRegistry.Register(tools.NewReadFileTool(workDir))
	mainRegistry.Register(tools.NewWriteFileTool(workDir))
	mainRegistry.Register(tools.NewBashTool(workDir))
	mainRegistry.Register(tools.NewEditFileTool(workDir))

	// 初始化主引擎
	eng := engine.NewAgentEngine(llmProvider, mainRegistry, false, false)

	// 【核心装配】：将带有 Engine 引用和只读 Registry 的 Subagent 工具注册进主线
	mainRegistry.Register(tools.NewSubagentTool(eng, readOnlyRegistry, reporter))

	sessionID := "test_subagent_001"
	sess := ctxpkg.GlobalSessionMgr.GetOrCreate(sessionID, workDir)

	prompt := `
    我需要你在这个遗留项目里，找到那个“核心密码”。
    为了防止污染主上下文，请你务必派出子智能体（spawn_subagent）去执行探索任务。
    你可以让子智能体使用 bash 去查找当前目录（及其所有子目录）下名为 config.txt 的文件。
    子智能体拿到密码向你汇报后，请你亲自使用 write_file 工具，将密码写在根目录的 answer.txt 里。
    `

	log.Println("\n>>> 🚀 启动多智能体协同测试...")
	sess.Append(schema.Message{Role: schema.RoleUser, Content: prompt})

	err := eng.Run(context.Background(), sess, reporter)
	if err != nil {
		log.Fatalf("引擎运行崩溃: %v", err)
	}
}
