// Package kiro 实现 KiroAdapter。
//
// 当前阶段：**先做透传**。Kiro 通道暂按 Anthropic 兼容方式直接转发到其 base_url，
// 不开发私有协议适配。待后续根据真实报错，再在本包内逐步覆盖私有认证 / 协议 /
// 流式分帧 / usage 提取（任务书 §5.5 ⭐；§10 要求不臆测 wire format）。
//
// 设计预留：透传由通用 httpproxy 适配器承担；将来需要私有协议时，可在此包内
// 实现完整的 upstream.Adapter（请求转换 / 响应重封装 / 令牌刷新），替换透传。
package kiro

import (
	"strings"

	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/upstream/httpproxy"
)

// New 构造 Kiro 适配器（当前为透传实现）。
func New(ch *domain.UpstreamChannel) *httpproxy.Adapter {
	base := ""
	authHeader := "Authorization"
	authScheme := "Bearer"
	if ch != nil {
		base = strings.TrimRight(ch.BaseURL, "/")
		// 允许通过 config 指定透传时的认证头，便于按实际上游调整
		if h, ok := ch.Config["auth_header"].(string); ok && h != "" {
			authHeader = h
		}
		if s, ok := ch.Config["auth_scheme"].(string); ok {
			authScheme = s
		}
	}
	return httpproxy.New(httpproxy.Options{
		ChannelType: domain.ChannelKiro,
		BaseURL:     base,
		AuthHeader:  authHeader,
		AuthScheme:  authScheme,
	})
}
