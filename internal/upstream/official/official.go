// Package official 实现 OfficialAdapter：直连 Anthropic 官方 API。
//
// 标准通道参照实现：原生 Messages 协议、标准 SSE，复用通用透传适配器，
// 仅在认证头（x-api-key）与默认 base_url 上做配置。
package official

import (
	"strings"

	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/upstream/httpproxy"
)

const defaultBaseURL = "https://api.anthropic.com"

// New 按通道配置构造官方适配器。
func New(ch *domain.UpstreamChannel) *httpproxy.Adapter {
	base := defaultBaseURL
	version := "2023-06-01"
	if ch != nil {
		if ch.BaseURL != "" {
			base = strings.TrimRight(ch.BaseURL, "/")
		}
		if v, ok := ch.Config["anthropic_version"].(string); ok && v != "" {
			version = v
		}
	}
	return httpproxy.New(httpproxy.Options{
		ChannelType:      domain.ChannelOfficial,
		BaseURL:          base,
		AuthHeader:       "x-api-key",
		AnthropicVersion: version,
	})
}
