package upstream

import (
	"fmt"
	"sync"

	"github.com/claude-gate/claude-gate/internal/domain"
)

// Factory 按通道配置构造 Adapter。
type Factory func(ch *domain.UpstreamChannel) (Adapter, error)

// Registry 按 channel.type 选择并缓存 Adapter（任务书 §8 registry.go）。
//
// 新增通道只需实现 Adapter 接口并在此注册，主链路无需改动（任务书 §10）。
type Registry struct {
	mu        sync.RWMutex
	factories map[domain.ChannelType]Factory
}

// NewRegistry 构造空注册表。
func NewRegistry() *Registry {
	return &Registry{factories: make(map[domain.ChannelType]Factory)}
}

// Register 注册某通道类型的 Adapter 工厂。
func (r *Registry) Register(t domain.ChannelType, f Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[t] = f
}

// Build 按通道配置构造对应 Adapter。
func (r *Registry) Build(ch *domain.UpstreamChannel) (Adapter, error) {
	if ch == nil {
		return nil, domain.ErrAdapterNotFound
	}
	r.mu.RLock()
	f, ok := r.factories[ch.Type]
	r.mu.RUnlock()
	if !ok {
		return nil, domain.ErrAdapterNotFound.WithMessage(fmt.Sprintf("未注册的通道类型: %s", ch.Type))
	}
	return f(ch)
}

// Supports 返回某通道类型是否已注册。
func (r *Registry) Supports(t domain.ChannelType) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.factories[t]
	return ok
}
