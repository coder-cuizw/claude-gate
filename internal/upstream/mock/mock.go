// Package mock 提供一个本地合成的 Adapter，用于离线自测与演示。
//
// 它不访问网络，直接根据请求合成一段标准 Anthropic 响应（含 usage）与对应的
// SSE 事件序列，使网关可在没有真实上游的情况下端到端跑通（注册为 custom 通道）。
package mock

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/claude-gate/claude-gate/internal/domain"
)

// Adapter 本地合成适配器。
type Adapter struct{ channel *domain.UpstreamChannel }

// New 构造 mock 适配器。
func New(ch *domain.UpstreamChannel) *Adapter { return &Adapter{channel: ch} }

// Name 返回通道类型名。
func (a *Adapter) Name() string { return string(domain.ChannelCustom) }

// 合成回复文本与 usage（usage 故意给出可观测的数值，便于验证缓存计费改写）。
const replyText = "这是 claude-gate 本地 mock 通道合成的回复。"

func mockUsage(req *domain.MessagesRequest) domain.RawUsage {
	// 简单按消息数估算输入，固定输出，便于自测断言
	in := 100 + len(req.Messages)*50
	return domain.RawUsage{
		InputTokens:              in,
		OutputTokens:             42,
		CacheCreationInputTokens: 10,
		CacheReadInputTokens:     200,
	}
}

// Send 合成非流式响应。
func (a *Adapter) Send(_ context.Context, req *domain.MessagesRequest, _ *domain.UpstreamKey) (*domain.MessagesResponse, error) {
	usage := mockUsage(req)
	content, _ := json.Marshal([]map[string]any{{"type": "text", "text": replyText}})
	return &domain.MessagesResponse{
		ID:         "msg_mock_" + fmt.Sprint(time.Now().UnixNano()),
		Type:       "message",
		Role:       "assistant",
		Model:      req.Model,
		Content:    content,
		StopReason: "end_turn",
		Usage:      &usage,
	}, nil
}

// SendStream 合成标准 Anthropic SSE 事件序列。
func (a *Adapter) SendStream(ctx context.Context, req *domain.MessagesRequest, _ *domain.UpstreamKey) (<-chan domain.StreamEvent, error) {
	usage := mockUsage(req)
	out := make(chan domain.StreamEvent, 8)
	go func() {
		defer close(out)
		emit := func(event string, data any) bool {
			b, _ := json.Marshal(data)
			select {
			case out <- domain.StreamEvent{Event: event, Data: b}:
				return true
			case <-ctx.Done():
				return false
			}
		}
		if !emit("message_start", map[string]any{"type": "message_start", "message": map[string]any{"id": "msg_mock", "role": "assistant", "model": req.Model, "usage": map[string]int{"input_tokens": usage.InputTokens, "output_tokens": 0}}}) {
			return
		}
		emit("content_block_start", map[string]any{"type": "content_block_start", "index": 0, "content_block": map[string]any{"type": "text", "text": ""}})
		emit("content_block_delta", map[string]any{"type": "content_block_delta", "index": 0, "delta": map[string]any{"type": "text_delta", "text": replyText}})
		emit("content_block_stop", map[string]any{"type": "content_block_stop", "index": 0})
		// message_delta 携带最终 usage（缓存计费策略在此改写）
		emit("message_delta", map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": "end_turn"}, "usage": map[string]int{
			"output_tokens": usage.OutputTokens, "input_tokens": usage.InputTokens,
			"cache_creation_input_tokens": usage.CacheCreationInputTokens, "cache_read_input_tokens": usage.CacheReadInputTokens,
		}})
		emit("message_stop", map[string]any{"type": "message_stop"})
	}()
	return out, nil
}
