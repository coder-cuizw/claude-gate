// Package relay 实现 RelayAdapter：Anthropic 兼容的第三方中转。
//
// 复用通用透传适配器，支持透传客户凭证或自定义 Bearer（按 config.auth_mode）。
package relay

import (
	"strings"

	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/upstream/httpproxy"
)

// New 按通道配置构造第三方中转适配器。
func New(ch *domain.UpstreamChannel) *httpproxy.Adapter {
	base := ""
	if ch != nil {
		base = strings.TrimRight(ch.BaseURL, "/")
	}
	return httpproxy.New(httpproxy.Options{
		ChannelType: domain.ChannelRelay,
		BaseURL:     base,
		AuthHeader:  "Authorization",
		AuthScheme:  "Bearer",
	})
}
