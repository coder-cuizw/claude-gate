package upstream

import (
	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/upstream/kiro"
	"github.com/claude-gate/claude-gate/internal/upstream/mock"
	"github.com/claude-gate/claude-gate/internal/upstream/official"
	"github.com/claude-gate/claude-gate/internal/upstream/relay"
)

// DefaultRegistry 返回注册了生产通道的注册表：
//   - official：直连官方（标准）
//   - kiro：先做透传（私有协议适配后续按真实报错再补，见 internal/upstream/kiro）
//   - relay：Anthropic 兼容第三方中转（透传 / 自定义 Bearer）
//
// custom (mock) 不在此注册——它是离线测试用，由 RegisterMock 显式挂入，避免生产链路误用。
// Bedrock / Vertex 已按需移除；后续要接入只需实现 Adapter 并在此注册，主链路不变。
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(domain.ChannelOfficial, func(ch *domain.UpstreamChannel) (Adapter, error) {
		return official.New(ch), nil
	})
	r.Register(domain.ChannelKiro, func(ch *domain.UpstreamChannel) (Adapter, error) {
		return kiro.New(ch), nil
	})
	r.Register(domain.ChannelRelay, func(ch *domain.UpstreamChannel) (Adapter, error) {
		return relay.New(ch), nil
	})
	return r
}

// RegisterMock 把本地 mock 适配器（custom 类型）挂入注册表。仅离线测试 / BuildMemory 使用。
func RegisterMock(r *Registry) {
	r.Register(domain.ChannelCustom, func(ch *domain.UpstreamChannel) (Adapter, error) {
		return mock.New(ch), nil
	})
}
