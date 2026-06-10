package upstream

import (
	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/upstream/kiro"
	"github.com/claude-gate/claude-gate/internal/upstream/official"
)

// DefaultRegistry 返回注册了 MVP（M1）通道的注册表：official（标准）+ kiro（特殊）。
//
// Bedrock / Vertex / Relay 属于 M5，待实现后在此追加注册（任务书 §5.5 路线图）。
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(domain.ChannelOfficial, func(ch *domain.UpstreamChannel) (Adapter, error) {
		return official.New(ch), nil
	})
	r.Register(domain.ChannelKiro, func(ch *domain.UpstreamChannel) (Adapter, error) {
		return kiro.New(ch), nil
	})
	return r
}
