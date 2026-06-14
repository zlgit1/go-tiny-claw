package engine

import (
	"context"
	"fmt"
	"log"
	"sync"

	ctxpkg "github.com/zlgit1/go-tiny-claw/internal/context" // 引入我们新建的 context 包
	"github.com/zlgit1/go-tiny-claw/internal/provider"
	"github.com/zlgit1/go-tiny-claw/internal/schema"
	"github.com/zlgit1/go-tiny-claw/internal/tools"
)

// AgentEngine 是微型 OS 的核心驱动
type AgentEngine struct {
	provider provider.LLMProvider
	registry tools.Registry
	// WorkDir (工作区): 借鉴 OpenClaw 的理念，Agent 必须有一个明确的物理边界
	// WorkDir        string 【注意】：我们移除了 Engine 层级的 WorkDir，因为 WorkDir 现在应该跟随 Session 走！
	EnableThinking bool // 【新增】慢思考模式开关
	// composer       *ctxpkg.PromptComposer
	PlanMode  bool                    // 【新增】暴露给外部的计划模式开关
	compactor *ctxpkg.Compactor       // 【新增】压缩器实例
	recovery  *ctxpkg.RecoveryManager // 【新增】自愈管理器
}

func NewAgentEngine(p provider.LLMProvider, r tools.Registry, enableThinking bool, planMode bool) *AgentEngine {
	return &AgentEngine{
		provider: p,
		registry: r,
		// WorkDir:        workDir, 【注意】：我们移除了 Engine 层级的 WorkDir，因为 WorkDir 现在应该跟随 Session 走！
		EnableThinking: enableThinking,
		// (假装这里能获取到 WorkDir 初始化 Composer，生产环境中应在 Run 中动态构造)
		// composer: ctxpkg.NewPromptComposer("."),
		// 【初始化压缩器】：为了便于今天的极端测试，我们将水位线阈值设积极（例如 3000 字符），

		PlanMode: planMode,
		// 并保护最近的 6 条消息（大约两轮 Turn 的交互）
		compactor: ctxpkg.NewCompactor(3000, 6),
		recovery:  ctxpkg.NewRecoveryManager(), // 初始化 Recovery
	}
}

// Run 启动 Agent 的生命周期
// 【核心改造】: 移除 userPrompt 参数，改为接收一个具体的 Session 实例
func (e *AgentEngine) Run(ctx context.Context, session *ctxpkg.Session, reporter Reporter) error {

	log.Printf("[Engine] 慢思考模式 (Thinking Phase): %v\n", e.EnableThinking)
	log.Printf("[Engine] 唤醒会话 [%s]，锁定工作区: %s (PlanMode: %v)\n", session.ID, session.WorkDir, e.PlanMode)

	// 根据当前 Session 的工作区，动态组装最新的 System Prompt并传入当前的 PlanMode 状态
	composer := ctxpkg.NewPromptComposer(session.WorkDir, e.PlanMode) // 【新增】引擎持有 Composer 实例

	// 【核心修改】动态组装 System Prompt，彻底替换掉以前硬编码的面条提示词！
	systemMsg := composer.Build()

	// 1. 初始化会话的 Context (上下文内存)
	// 在真实的场景中，这里会由动态 Prompt 组装器加载 AGENTS.md。目前我们先硬编码。
	// contextHistory := []schema.Message{
	// 	systemMsg, // 注入动态组装的内核、AGENTS.md 与 Skills
	// 	{
	// 		Role:    schema.RoleUser,
	// 		Content: userPrompt,
	// 	},
	// }

	// 2. The Main Loop: 心跳开始 (标准的 ReAct 循环)
	for {
		// 获取当前挂载的所有工具定义
		availableTools := e.registry.GetAvailableTools()

		// 1. 【上下文组装】: System Prompt + 截取最近的 6 条消息作为 Working Memory
		// 在实际业务中，由于工具返回结果可能很长，短期工作记忆往往设为 6-10 条足以维系连贯对话
		// workingMemory := session.GetWorkingMemory(6)

		// 1. 从 Session 提取出近期的 Working Memory (例如最近 20 条，给压缩器留下充足的判断空间)
		workingMemory := session.GetWorkingMemory(20)

		var contextHistory []schema.Message
		contextHistory = append(contextHistory, systemMsg)
		contextHistory = append(contextHistory, workingMemory...)
		// 2. 【核心注入点】: 在向 Provider 发起推理前，过一遍内存压缩器！
		// 无论你带出了多少上下文，如果字符总数超标，早期日志将被掩码化，超大日志将被掐头去尾
		compactedContext := e.compactor.Compact(contextHistory)

		// ====================================================================
		//  Phase 1: 慢思考阶段 (Thinking) - 剥夺工具，强制规划
		// ====================================================================

		if e.EnableThinking {
			log.Println("[Engine][Phase 1] 剥夺工具访问权，强制进入慢思考与规划阶段...")
			if reporter != nil {
				// 【触发 Reporter】: 开始慢思考
				reporter.OnThinking(ctx)
			}
			// 核心机制：传入的 availableTools 为 nil！
			// 大模型看不到任何 JSON Schema，被迫只能输出纯文本的思考过程。
			thinkResp, err := e.provider.Generate(ctx, compactedContext, nil)

			if err != nil {
				return fmt.Errorf("Thinking 阶段生成失败: %w", err)
			}
			// 如果模型输出了思考过程，我们将其作为 Assistant 消息追加到上下文中
			if thinkResp.Content != "" {
				// 将思考过程持久化到 Session 中！
				session.Append(*thinkResp)

				compactedContext = append(compactedContext, *thinkResp)
			}
		}

		// ====================================================================
		// Phase 2: 行动阶段 (Action) - 恢复工具，顺着规划执行
		// ====================================================================

		// 此时的 contextHistory 中已经包含了上一阶段模型自己的 Thinking Trace。
		// 模型会顺着自己的逻辑，结合恢复的 availableTools 发起精准的工具调用。
		// 向大模型发起推理请求 (包含 Reasoning)
		actionResp, err := e.provider.Generate(ctx, compactedContext, availableTools)
		if err != nil {
			return fmt.Errorf("Action 阶段生成失败: %w", err)
		}
		// 【驾驭精髓】：注意，写入 Session（硬盘/全量内存）的永远是全量的真实响应，不受 Compact 影响！
		// Compact 只作用于本轮发给大模型的那个临时 Context。
		// 将大模型的行动响应持久化到 Session 中
		session.Append(*actionResp)
		compactedContext = append(compactedContext, *actionResp)

		// 如果模型回复了纯文本，打印出来 (这通常是它的思考过程，或是最终结果)
		if actionResp.Content != "" && reporter != nil {
			// 【触发 Reporter】: 输出阶段性总结或最终回复
			reporter.OnMessage(ctx, actionResp.Content)
		}

		// 3. 退出条件判断
		// 如果模型没有请求任何工具调用，说明它认为任务已经完成，跳出循环。
		if len(actionResp.ToolCalls) == 0 {
			log.Println("[Engine] 任务完成，退出循环。")
			break
		}

		// 【核心改造开始】: 从串行 (Sequential) 演进为并行 (Parallel)
		// 1. 预分配一个固定长度的切片，用于安全地存放各个并发工具的执行结果（Observation）
		// 长度与 ToolCalls 的数量完全一致
		observationMsgs := make([]schema.Message, len(actionResp.ToolCalls))

		// 2. 声明 WaitGroup 用于阻塞等待所有协程完成
		var wg sync.WaitGroup

		// 3. 遍历模型请求的所有工具，为每一个工具单独 Fork 出一个 Goroutine
		for i, toolCall := range actionResp.ToolCalls {
			wg.Add(1) // 增加计数器

			// 开启协程。注意：一定要将索引 i 和 toolCall 作为参数传入匿名函数，防止闭包变量捕获陷阱！
			go func(idx int, call schema.ToolCall) {
				defer wg.Done() // 协程结束时计数器减一
				if reporter != nil {
					// 【触发 Reporter】: 报告即将在底层执行的工具
					reporter.OnToolCall(ctx, call.Name, string(call.Arguments))
				}

				// 调用底层 Registry 执行工具（物理操作）
				result := e.registry.Execute(ctx, call)

				// 【核心拦截与注入】
				finalOutput := result.Output
				if result.IsError {
					// 发生错误，交由 RecoveryManager 诊断并注入“锦囊妙计”
					finalOutput = e.recovery.AnalyzeAndInject(call.Name, result.Output)
					log.Printf(" -> [Go-%d] ❌ 注入救援指南: %s\n", idx, finalOutput)
				} else {
					log.Printf(" -> [Go-%d] ✅ 工具执行成功 (返回 %d 字节)\n", idx, len(result.Output))
				}
				if reporter != nil {
					// 为了防止大文件读取导致飞书消息过长被截断，我们仅汇报工具执行状态
					// 注意：传递给大模型的 observationMsgs 依然是完整数据，只是人类看到的 Reporter 是缩略版
					displayOutput := result.Output
					if len(displayOutput) > 200 {
						displayOutput = displayOutput[:200] + "... (已截断)"
					}
					// 【触发 Reporter】: 汇报工具物理执行的结果
					reporter.OnToolResult(ctx, call.Name, displayOutput, result.IsError)
				}
				// 将执行结果封装为一条用户消息 (RoleUser)
				obsMsg := schema.Message{
					Role:       schema.RoleUser,
					Content:    result.Output,
					ToolCallID: call.ID,
				}

				// 【线程安全】: 由于每个 Goroutine 操作的是预分配切片的不同索引，
				// 这里不需要加锁 (Mutex)，性能极高！
				observationMsgs[idx] = obsMsg

			}(i, toolCall) // 闭包传参
		}

		// 4. Join 阻塞等待：主循环挂起，直到所有的并发协程全部执行完毕
		wg.Wait()
		log.Println("[Engine] 所有并发工具执行完毕，开始聚合观察结果 (Observation)...")

		// 5. 聚合装填：将并行的结果，按照原本的顺序，一次性追加到上下文时间线中
		// 这等价于 contextHistory = append(contextHistory, observationMsgs...)
		// for _, obs := range observationMsgs {
		// 	contextHistory = append(contextHistory, obs)
		// }

		// 将所有的工具执行结果（Observation）持久化到 Session 中，开启下一轮的复盘与推理
		session.Append(observationMsgs...)

		// 循环回到开头，模型将带着这一批新的 Observation 继续它的下一轮思考...
	}
	return nil
}
