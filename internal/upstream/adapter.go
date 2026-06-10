// Package upstream 是上游适配层（任务书 §5.5 ⭐ 多通道核心）。
//
// 职责：屏蔽不同上游通道（Kiro / Official / Bedrock / Vertex / Relay）的差异，
// 对主链路提供统一的 Adapter 调用接口；并管理上游 Key 池。
//
// 通道隔离原则（任务书 §10）：通道差异只允许出现在 internal/upstream/{type}/ 内，
// 禁止泄漏到 gateway / transformer / cache。
package upstream

import (
	"context"

	"github.com/claude-gate/claude-gate/internal/domain"
)

// Adapter 是上游通道的统一适配接口。
type Adapter interface {
	// Name 返回通道类型名（与 channel.type 一致）。
	Name() string
	// Send 发送非流式请求，返回标准 Anthropic 响应。
	Send(ctx context.Context, req *domain.MessagesRequest, key *domain.UpstreamKey) (*domain.MessagesResponse, error)
	// SendStream 发送流式请求，返回标准 Anthropic SSE 事件流。
	// 返回的 channel 在请求结束或 ctx 取消时必须关闭，避免 goroutine 泄漏。
	SendStream(ctx context.Context, req *domain.MessagesRequest, key *domain.UpstreamKey) (<-chan domain.StreamEvent, error)
}

// Result 是一次上游调用的结果反馈，用于 Key 池健康调度。
type Result struct {
	Success    bool
	StatusCode int
	Err        error
}

// KeyPool 管理某通道下的上游 Key 调度（任务书 §5.5）。
type KeyPool interface {
	// Acquire 取一把可用 Key（status=active）。无可用 Key 时返回 domain.ErrNoUpstreamKey。
	Acquire(ctx context.Context, channelID int64) (*domain.UpstreamKey, error)
	// Release 上报调用结果；429/5xx 会触发该 Key 进入 cooldown。
	Release(key *domain.UpstreamKey, result Result)
}
