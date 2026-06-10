// Package toolcall 实现 tool_call_normalizer 改写器。
//
// 作用：在 Anthropic 工具调用格式与上游期望格式之间做通用规整。
// 注意：通道私有的工具协议差异（如 Kiro）由对应 Adapter 处理，
// 这里只做通道无关的轻量修补，例如补齐缺失的 input_schema。
package toolcall

import (
	"context"
	"encoding/json"

	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/transformer"
)

// 空对象 schema，作为缺失 input_schema 时的安全默认值。
var emptyObjectSchema = json.RawMessage(`{"type":"object","properties":{}}`)

// Normalizer 工具调用规整改写器。
type Normalizer struct {
	transformer.Base
}

// New 构造 Normalizer。
func New() *Normalizer { return &Normalizer{} }

// Name 返回改写器名。
func (n *Normalizer) Name() string { return "tool_call_normalizer" }

// TransformRequest 为缺失 input_schema 的工具补齐空对象 schema，
// 避免部分上游因 schema 缺失而拒绝请求。
func (n *Normalizer) TransformRequest(_ context.Context, req *domain.MessagesRequest) (*domain.MessagesRequest, error) {
	if req == nil || len(req.Tools) == 0 {
		return req, nil
	}
	clone := *req
	clone.Tools = make([]domain.Tool, len(req.Tools))
	copy(clone.Tools, req.Tools)
	for i := range clone.Tools {
		if len(clone.Tools[i].InputSchema) == 0 {
			clone.Tools[i].InputSchema = emptyObjectSchema
		}
	}
	return &clone, nil
}
