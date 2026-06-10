// Package transformer 实现请求改写流水线（任务书 §5.4）。
//
// Pipeline 负责"通道无关的通用改写"：在请求送往上游前、响应返回客户前，
// 按分组配置链式改写。通道私有协议的转换属于 Adapter 职责，不在此处。
//
// 设计原则：
//   - 每个 Transformer 一个子包，便于增删；
//   - Pipeline 顺序由分组配置决定，前序输出是后序输入；
//   - 单个 Transformer 失败可配置 fail-fast 或 skip。
package transformer

import (
	"context"
	"fmt"

	"github.com/claude-gate/claude-gate/internal/domain"
)

// Transformer 是请求/响应改写器的统一接口。
type Transformer interface {
	// Name 返回改写器名，与分组配置中的 name 一致。
	Name() string
	// TransformRequest 在请求送往上游前改写。
	TransformRequest(ctx context.Context, req *domain.MessagesRequest) (*domain.MessagesRequest, error)
	// TransformResponse 在非流式响应返回客户前改写。
	TransformResponse(ctx context.Context, resp *domain.MessagesResponse) (*domain.MessagesResponse, error)
	// TransformStreamEvent 在每个流式事件返回客户前改写。返回 nil 表示丢弃该事件。
	TransformStreamEvent(ctx context.Context, ev *domain.StreamEvent) (*domain.StreamEvent, error)
}

// FailMode 定义单个 Transformer 失败时的处理策略。
type FailMode string

const (
	// FailFast：任一 Transformer 出错则整条流水线中断并返回错误。
	FailFast FailMode = "fail-fast"
	// FailSkip：出错的 Transformer 被跳过，保留上一步结果继续。
	FailSkip FailMode = "skip"
)

// stage 是流水线中的一个环节，绑定改写器与其失败策略。
type stage struct {
	t        Transformer
	failMode FailMode
}

// Pipeline 是一组按序执行的 Transformer。
type Pipeline struct {
	stages []stage
}

// NewPipeline 构造空流水线。
func NewPipeline() *Pipeline { return &Pipeline{} }

// Use 追加一个改写器及其失败策略。
func (p *Pipeline) Use(t Transformer, mode FailMode) *Pipeline {
	if mode == "" {
		mode = FailFast
	}
	p.stages = append(p.stages, stage{t: t, failMode: mode})
	return p
}

// Len 返回流水线中改写器数量。
func (p *Pipeline) Len() int { return len(p.stages) }

// ApplyRequest 顺序执行所有改写器的 TransformRequest。
func (p *Pipeline) ApplyRequest(ctx context.Context, req *domain.MessagesRequest) (*domain.MessagesRequest, error) {
	cur := req
	for _, s := range p.stages {
		next, err := s.t.TransformRequest(ctx, cur)
		if err != nil {
			if s.failMode == FailSkip {
				continue
			}
			return nil, fmt.Errorf("transformer %s 改写请求失败: %w", s.t.Name(), err)
		}
		if next != nil {
			cur = next
		}
	}
	return cur, nil
}

// ApplyResponse 顺序执行所有改写器的 TransformResponse。
func (p *Pipeline) ApplyResponse(ctx context.Context, resp *domain.MessagesResponse) (*domain.MessagesResponse, error) {
	cur := resp
	for _, s := range p.stages {
		next, err := s.t.TransformResponse(ctx, cur)
		if err != nil {
			if s.failMode == FailSkip {
				continue
			}
			return nil, fmt.Errorf("transformer %s 改写响应失败: %w", s.t.Name(), err)
		}
		if next != nil {
			cur = next
		}
	}
	return cur, nil
}

// ApplyStreamEvent 顺序执行所有改写器的 TransformStreamEvent。
// 任一改写器返回 nil 事件即视为丢弃，提前结束。
func (p *Pipeline) ApplyStreamEvent(ctx context.Context, ev *domain.StreamEvent) (*domain.StreamEvent, error) {
	cur := ev
	for _, s := range p.stages {
		next, err := s.t.TransformStreamEvent(ctx, cur)
		if err != nil {
			if s.failMode == FailSkip {
				continue
			}
			return nil, fmt.Errorf("transformer %s 改写流事件失败: %w", s.t.Name(), err)
		}
		if next == nil {
			return nil, nil // 事件被丢弃
		}
		cur = next
	}
	return cur, nil
}

// Base 提供 Transformer 的默认空实现，便于改写器只覆盖关心的方法。
type Base struct{}

func (Base) TransformRequest(_ context.Context, req *domain.MessagesRequest) (*domain.MessagesRequest, error) {
	return req, nil
}
func (Base) TransformResponse(_ context.Context, resp *domain.MessagesResponse) (*domain.MessagesResponse, error) {
	return resp, nil
}
func (Base) TransformStreamEvent(_ context.Context, ev *domain.StreamEvent) (*domain.StreamEvent, error) {
	return ev, nil
}
