// Package streaming 实现 streaming_event_fixer 改写器。
//
// 作用：修复部分上游流式响应中的通用格式问题，例如：
//   - data 行缺少 event 类型时，从 JSON 的 type 字段补出 SSE event 名；
//   - 丢弃完全空的事件。
//
// 通道私有的分帧问题（如 Kiro 私有事件帧）由对应 Adapter 重封装，不在此处。
package streaming

import (
	"context"
	"encoding/json"

	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/transformer"
)

// Fixer 流式事件修复改写器。
type Fixer struct {
	transformer.Base
}

// New 构造 Fixer。
func New() *Fixer { return &Fixer{} }

// Name 返回改写器名。
func (f *Fixer) Name() string { return "streaming_event_fixer" }

// TransformStreamEvent 修复流式事件：补全 event 名，丢弃空事件。
func (f *Fixer) TransformStreamEvent(_ context.Context, ev *domain.StreamEvent) (*domain.StreamEvent, error) {
	if ev == nil {
		return nil, nil
	}
	// 完全空的事件直接丢弃
	if ev.Event == "" && len(ev.Data) == 0 {
		return nil, nil
	}
	// event 缺失但 data 中带 type 字段时，用 type 补出 event 名
	if ev.Event == "" && len(ev.Data) > 0 {
		var probe struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(ev.Data, &probe); err == nil && probe.Type != "" {
			clone := *ev
			clone.Event = probe.Type
			return &clone, nil
		}
	}
	return ev, nil
}
