// Package modelmap 实现 model_mapper 改写器：把客户请求的模型别名映射为上游模型名。
package modelmap

import (
	"context"

	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/transformer"
)

// Mapper 模型别名映射改写器。映射表来源于 model_mappings 表（按通道）。
type Mapper struct {
	transformer.Base
	// mapping 客户模型名 → 上游模型名。
	mapping map[string]string
}

// New 用给定映射表构造 Mapper。
func New(mapping map[string]string) *Mapper {
	if mapping == nil {
		mapping = map[string]string{}
	}
	return &Mapper{mapping: mapping}
}

// Name 返回改写器名。
func (m *Mapper) Name() string { return "model_mapper" }

// TransformRequest 若命中映射表，则把请求中的 model 替换为上游模型名。
func (m *Mapper) TransformRequest(_ context.Context, req *domain.MessagesRequest) (*domain.MessagesRequest, error) {
	if req == nil {
		return req, nil
	}
	if upstream, ok := m.mapping[req.Model]; ok && upstream != "" {
		clone := *req
		clone.Model = upstream
		return &clone, nil
	}
	return req, nil
}
