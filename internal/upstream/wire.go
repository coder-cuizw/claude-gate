package upstream

import (
	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/upstream/kiro"
	"github.com/claude-gate/claude-gate/internal/upstream/mock"
	"github.com/claude-gate/claude-gate/internal/upstream/official"
	"github.com/claude-gate/claude-gate/internal/upstream/relay"
)

// DefaultRegistry 返回注册了当前支持通道的注册表：
//   - official：直连官方（标准）
//   - kiro：先做透传（私有协议适配后续按真实报错再补，见 internal/upstream/kiro）
//   - relay：Anthropic 兼容第三方中转（透传 / 自定义 Bearer）
//   - custom：本地 mock 合成通道，用于离线自测与演示
//
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
	r.Register(domain.ChannelCustom, func(ch *domain.UpstreamChannel) (Adapter, error) {
		return mock.New(ch), nil
	})
	return r
}
