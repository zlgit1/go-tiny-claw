package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/go-deepseek/deepseek/response"
	"github.com/zlgit1/go-tiny-claw/internal/schema"
)

const deepseekBaseURL = "https://api.deepseek.com"

// DeepSeekProvider 通过 go-deepseek/deepseek 库接入 DeepSeek 大模型
type DeepSeekProvider struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

// NewDeepSeekProvider 创建 DeepSeek provider，须设置环境变量 DEEPSEEK_API_KEY
func NewDeepSeekProvider(model string) *DeepSeekProvider {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		panic("请设置 DEEPSEEK_API_KEY 环境变量")
	}
	return &DeepSeekProvider{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// --- 请求体类型（本地定义，以弥补 go-deepseek/deepseek 库 request.Message 不支持 tool_calls 的局限）---

type dsChatRequest struct {
	Messages  []dsMessage `json:"messages"`
	Model     string      `json:"model"`
	MaxTokens int         `json:"max_tokens,omitempty"`
	Tools     []dsTool    `json:"tools,omitempty"`
}

type dsMessage struct {
	Role       string      `json:"role"`
	Content    string      `json:"content,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	ToolCalls  []dsToolCall `json:"tool_calls,omitempty"`
}

type dsTool struct {
	Type     string         `json:"type"`
	Function dsToolFunction `json:"function"`
}

type dsToolFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters"`
}

type dsToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function dsToolCallFunc   `json:"function"`
}

type dsToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Generate 实现 LLMProvider 接口
func (p *DeepSeekProvider) Generate(ctx context.Context, msgs []schema.Message, availableTools []schema.ToolDefinition) (*schema.Message, error) {
	// 1. 翻译上下文消息
	var dsMsgs []dsMessage
	for _, msg := range msgs {
		dsMsgs = append(dsMsgs, p.translateMessage(msg))
	}

	// 2. 翻译工具定义
	var dsTools []dsTool
	for _, td := range availableTools {
		dsTools = append(dsTools, dsTool{
			Type: "function",
			Function: dsToolFunction{
				Name:        td.Name,
				Description: td.Description,
				Parameters:  td.InputSchema,
			},
		})
	}

	// 3. 构建请求
	reqBody := dsChatRequest{
		Model:     p.model,
		Messages:  dsMsgs,
		MaxTokens: 4096,
	}
	if len(dsTools) > 0 {
		reqBody.Tools = dsTools
	}

	// 4. 序列化并发送 HTTP 请求
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("DeepSeek 请求序列化失败: %w", err)
	}

	url := deepseekBaseURL + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("DeepSeek 创建 HTTP 请求失败: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("DeepSeek API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("DeepSeek 读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DeepSeek API 返回异常状态码 %d: %s", resp.StatusCode, string(respBytes))
	}

	// 5. 使用库的 response 类型解析响应
	var chatResp response.ChatCompletionsResponse
	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		return nil, fmt.Errorf("DeepSeek 响应解析失败: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("DeepSeek API 返回了空的 Choices")
	}

	// 6. 反向翻译为内部 schema.Message
	choice := chatResp.Choices[0]
	if choice.Message == nil {
		return nil, fmt.Errorf("DeepSeek API 返回了空的 Message")
	}

	resultMsg := &schema.Message{
		Role:    schema.RoleAssistant,
		Content: choice.Message.Content,
	}

	for _, tc := range choice.Message.ToolCalls {
		resultMsg.ToolCalls = append(resultMsg.ToolCalls, schema.ToolCall{
			ID:        tc.Id,
			Name:      tc.Function.Name,
			Arguments: json.RawMessage(tc.Function.Arguments),
		})
	}

	return resultMsg, nil
}

// translateMessage 将内部 schema.Message 转为 DeepSeek 格式
// 规则：
//   - RoleSystem  → role="system"
//   - RoleUser + ToolCallID 为空   → role="user"
//   - RoleUser + ToolCallID 非空   → role="tool" (工具执行结果)
//   - RoleAssistant                → role="assistant" (含 tool_calls 回填)
func (p *DeepSeekProvider) translateMessage(msg schema.Message) dsMessage {
	switch msg.Role {
	case schema.RoleSystem:
		return dsMessage{Role: "system", Content: msg.Content}

	case schema.RoleUser:
		if msg.ToolCallID != "" {
			// 工具执行结果
			return dsMessage{Role: "tool", Content: msg.Content, ToolCallID: msg.ToolCallID}
		}
		return dsMessage{Role: "user", Content: msg.Content}

	case schema.RoleAssistant:
		dsm := dsMessage{Role: "assistant", Content: msg.Content}
		for _, tc := range msg.ToolCalls {
			dsm.ToolCalls = append(dsm.ToolCalls, dsToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: dsToolCallFunc{
					Name:      tc.Name,
					Arguments: string(tc.Arguments),
				},
			})
		}
		return dsm

	default:
		return dsMessage{Role: "user", Content: msg.Content}
	}
}
