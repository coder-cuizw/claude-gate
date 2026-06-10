package domain

import "encoding/json"

// 本文件定义对外恒定的 Anthropic Messages 协议结构。
// 无论下游走哪种上游通道，claude-gate 对客户暴露的都是这套协议；
// 通道私有协议的差异收敛在各 Adapter 内部（见 internal/upstream）。

// MessagesRequest 对应 POST /v1/messages 的请求体（简化但够用的子集）。
type MessagesRequest struct {
	Model       string          `json:"model"`
	Messages    []Message       `json:"messages"`
	System      json.RawMessage `json:"system,omitempty"` // 可能是 string 或 block 数组
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
	Tools       []Tool          `json:"tools,omitempty"`
	ToolChoice  json.RawMessage `json:"tool_choice,omitempty"`
	Metadata    map[string]any  `json:"metadata,omitempty"`
	// Extra 保留未显式建模的字段，确保转发时不丢信息。
	Extra map[string]json.RawMessage `json:"-"`
}

// Message 单条对话消息。
type Message struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"` // string 或 content block 数组
}

// Tool 工具定义。
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

// MessagesResponse 非流式响应体。
type MessagesResponse struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Role       string          `json:"role"`
	Model      string          `json:"model"`
	Content    json.RawMessage `json:"content"`
	StopReason string          `json:"stop_reason,omitempty"`
	Usage      *RawUsage       `json:"usage,omitempty"`
}

// RawUsage 对应 Anthropic 响应里的 usage 字段。
type RawUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// ToUsage 把 Anthropic 原生 usage 转为内部统一 Usage。
func (r *RawUsage) ToUsage() Usage {
	if r == nil {
		return Usage{}
	}
	return Usage{
		InputTokens:         r.InputTokens,
		OutputTokens:        r.OutputTokens,
		CacheCreationTokens: r.CacheCreationInputTokens,
		CacheReadTokens:     r.CacheReadInputTokens,
	}
}

// StreamEvent 表示一个 SSE 事件（Anthropic 流式协议）。
//
// 典型事件类型：message_start / content_block_start / content_block_delta /
// content_block_stop / message_delta / message_stop / ping。
type StreamEvent struct {
	Event string          `json:"-"`    // SSE event: 行
	Data  json.RawMessage `json:"-"`    // SSE data: 行（JSON）
}
