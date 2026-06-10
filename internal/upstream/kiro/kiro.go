// Package kiro 实现 KiroAdapter：唯一需要重度特殊处理的私有逆向通道（任务书 §5.5 ⭐）。
//
// ⚠️ 重要：Kiro 的具体协议细节（认证流程、端点、私有 schema、流式分帧格式）
// 由项目方单独提供。任务书 §10 明确要求："遇到 Kiro 相关格式问题先问，不要臆测
// wire format"。因此本文件只给出 Adapter 的结构骨架与各处理环节的清晰占位，
// 待项目方提供真实 wire format 后再补全实现，绝不臆测。
//
// KiroAdapter 需要在内部完成（不外泄到主链路）：
//  1. 认证与令牌刷新：维护 Kiro 私有凭证，按需刷新；过期/失效转 Key 池 cooldown
//  2. 请求转换：Anthropic Messages → Kiro 私有请求格式
//  3. 响应/流式转换：Kiro 私有响应帧 → 标准 Anthropic SSE 事件序列
//  4. usage 提取：从 Kiro 响应提取/估算 token 数，回填统一 Usage 供策略引擎使用
package kiro

import (
	"context"
	"sync"
	"time"

	"github.com/claude-gate/claude-gate/internal/domain"
)

// Adapter Kiro 私有通道适配器（骨架）。
type Adapter struct {
	channel *domain.UpstreamChannel

	mu          sync.RWMutex
	tokenCache  map[int64]tokenState // upstreamKeyID -> 令牌状态
	refreshFunc RefreshFunc          // 令牌刷新实现，待项目方协议补全
}

// tokenState 保存某 Key 的访问令牌与过期时间。
type tokenState struct {
	accessToken string
	expiresAt   time.Time
}

// RefreshFunc 是 Kiro 令牌刷新的可注入实现。
// 真实实现需调用 Kiro 私有刷新端点；签名保持稳定，便于后续替换。
type RefreshFunc func(ctx context.Context, key *domain.UpstreamKey) (accessToken string, expiresAt time.Time, err error)

// New 构造 Kiro 适配器骨架。
func New(ch *domain.UpstreamChannel) *Adapter {
	return &Adapter{
		channel:    ch,
		tokenCache: make(map[int64]tokenState),
	}
}

// WithRefreshFunc 注入令牌刷新实现。
func (a *Adapter) WithRefreshFunc(f RefreshFunc) *Adapter {
	a.refreshFunc = f
	return a
}

// Name 返回通道类型名。
func (a *Adapter) Name() string { return string(domain.ChannelKiro) }

// errAwaitingProtocol 在缺少项目方私有协议时统一返回，避免臆测 wire format。
var errAwaitingProtocol = domain.ErrUpstreamFailure.WithMessage(
	"Kiro 私有协议细节待项目方提供，KiroAdapter 暂未启用（详见 docs/channels.md）")

// Send 非流式请求（待协议补全）。
//
// 真实实现流程：ensureToken → 转换请求为 Kiro 私有格式 → 调用私有端点 →
// 将私有响应转换为 Anthropic 响应 → 提取 usage。
func (a *Adapter) Send(_ context.Context, _ *domain.MessagesRequest, _ *domain.UpstreamKey) (*domain.MessagesResponse, error) {
	return nil, errAwaitingProtocol
}

// SendStream 流式请求（待协议补全）。
//
// 真实实现流程：ensureToken → 转换请求 → 读取 Kiro 私有事件帧 →
// 重封装为标准 Anthropic SSE（message_start / content_block_delta /
// message_delta / message_stop …）→ 在 message_delta 回填 usage。
func (a *Adapter) SendStream(_ context.Context, _ *domain.MessagesRequest, _ *domain.UpstreamKey) (<-chan domain.StreamEvent, error) {
	return nil, errAwaitingProtocol
}

// ensureToken 返回某 Key 的有效访问令牌，过期则刷新（骨架）。
// 令牌刷新失败应由调用方转为 Key 池 cooldown。
func (a *Adapter) ensureToken(ctx context.Context, key *domain.UpstreamKey) (string, error) {
	a.mu.RLock()
	st, ok := a.tokenCache[key.ID]
	a.mu.RUnlock()
	if ok && time.Now().Before(st.expiresAt) {
		return st.accessToken, nil
	}
	if a.refreshFunc == nil {
		return "", errAwaitingProtocol
	}
	token, exp, err := a.refreshFunc(ctx, key)
	if err != nil {
		return "", err
	}
	a.mu.Lock()
	a.tokenCache[key.ID] = tokenState{accessToken: token, expiresAt: exp}
	a.mu.Unlock()
	return token, nil
}
