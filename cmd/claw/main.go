// // cmd/claw/main.go
// package main

// import (
// 	"context"
// 	"log"
// 	"os"

// 	"github.com/zlgit1/go-tiny-claw/internal/engine"
// 	"github.com/zlgit1/go-tiny-claw/internal/provider"
// 	"github.com/zlgit1/go-tiny-claw/internal/tools"
// )

// func main() {
// 	if os.Getenv("DEEPSEEK_API_KEY") == "" {
// 		log.Fatal("请先导出 DEEPSEEK_API_KEY 环境变量")
// 	}

// 	workDir, _ := os.Getwd()

// 	llmProvider := provider.NewDeepSeekProvider("deepseek-v4-flash")
// 	registry := tools.NewRegistry()

// 	// 挂载极简工具集
// 	registry.Register(tools.NewReadFileTool(workDir))
// 	registry.Register(tools.NewWriteFileTool(workDir))
// 	registry.Register(tools.NewBashTool(workDir))

// 	// 【新增挂载】
// 	registry.Register(tools.NewEditFileTool(workDir))

// 	// 实例化核心引擎，关闭慢思考阶段，享受 YOLO 急速模式
// 	eng := engine.NewAgentEngine(llmProvider, registry, workDir, true)

// 	// 发起一个需要连贯物理动作的任务
// 	// prompt := `
// 	// 请帮我执行以下操作：
// 	// 1. 用 bash 查看一下我当前电脑的 Go 版本。
// 	// 2. 帮我写一个简单的 helloworld.go 文件，输出 "Hello, go-tiny-claw!"。
// 	// 3. 用 bash 编译并运行这个 go 文件，确认它能正常工作。
// 	// `

// 	// 发起一个需要局部修改的指令
// 	// prompt := ` 我当前目录下有一个 server.go 文件。
// 	// 请帮我把里面 "TODO: 增加鉴权逻辑" 下面的那个 if 语句，整个替换为：
// 	// if user == nil {
// 	//    fmt.Println("Forbidden!")
// 	//    return
// 	// }
// 	// `

// 	// 下发一个需要收集多源信息的任务
// 	prompt := `
//     我当前目录下有 a.txt, b.txt, c.txt 三个文件。
//     为了节省时间，请你同时一次性读取这三个文件，并将它们的内容综合起来，告诉我它们分别记录了什么领域的信息。
//     `

//		err := eng.Run(context.Background(), prompt)
//		if err != nil {
//			log.Fatalf("引擎运行崩溃: %v", err)
//		}
//	}
//
// cmd/claw/main.go
package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/zlgit1/go-tiny-claw/internal/engine"
	"github.com/zlgit1/go-tiny-claw/internal/feishu"
	"github.com/zlgit1/go-tiny-claw/internal/provider"
	"github.com/zlgit1/go-tiny-claw/internal/tools"
)

func main() {
	workDir, _ := os.Getwd()
	workDir += "/workspace"

	llmProvider := provider.NewDeepSeekProvider("deepseek-v4-flash")

	registry := tools.NewRegistry()
	registry.Register(tools.NewReadFileTool(workDir))
	registry.Register(tools.NewWriteFileTool(workDir))
	registry.Register(tools.NewBashTool(workDir))
	registry.Register(tools.NewEditFileTool(workDir))

	eng := engine.NewAgentEngine(llmProvider, registry, workDir, true)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 飞书模式：有环境变量时后台启动
	if os.Getenv("FEISHU_APP_ID") != "" && os.Getenv("FEISHU_APP_SECRET") != "" {
		bot := feishu.NewFeishuBot(eng)
		go func() {
			log.Println("🚀 飞书 WebSocket 长连接模式启动...")
			if err := bot.StartWebSocket(ctx); err != nil {
				log.Printf("❌ WebSocket 连接失败: %v\n", err)
			}
		}()
	}
	// 终端交互模式：始终启动
	fmt.Println("🖥️  Go Tiny Claw 终端模式 (输入 exit 或 quit 退出)")
	fmt.Println("─────────────────────────────────────────────────")

	reporter := engine.NewTerminalReporter()
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("\n> ")

		inputCh := make(chan string, 1)
		go func() {
			if scanner.Scan() {
				inputCh <- scanner.Text()
			} else {
				inputCh <- ""
			}
		}()

		select {
		case <-sigChan:
			fmt.Println("\n📴 再见！")
			cancel()
			return
		case input := <-inputCh:
			input = strings.TrimSpace(input)
			if input == "" {
				continue
			}
			if input == "exit" || input == "quit" {
				fmt.Println("📴 再见！")
				cancel()
				return
			}

			runCtx, runCancel := context.WithCancel(ctx)
			done := make(chan struct{})

			go func() {
				defer close(done)
				if err := eng.Run(runCtx, input, reporter); err != nil && runCtx.Err() == nil {
					log.Printf("❌ Agent 运行失败: %v\n", err)
				}
			}()

			select {
			case <-done:
				runCancel()
			case <-sigChan:
				runCancel()
				<-done
				fmt.Println("\n📴 再见！")
				cancel()
				return
			}
		}
	}

}
