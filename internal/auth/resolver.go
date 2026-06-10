package auth

import (
	"context"
	"time"

	"github.com/claude-gate/claude-gate/internal/cache"
	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/transformer"
)

// ResolvedGroup 是一把 API Key 解析后的完整运行上下文。
type ResolvedGroup struct {
	APIKeyID      int64
	Group         *domain.Group
	Channel       *domain.UpstreamChannel
	CacheStrategy cache.Strategy
	Transformers  []transformer.Transformer
}

// GroupResolver 把 API Key 解析为可执行的分组上下文（任务书 §5.2）。
type GroupResolver interface {
	Resolve(ctx context.Context, apiKey string) (*ResolvedGroup, error)
}

// APIKeyRecord 是存储层返回的 API Key 记录（已去除明文）。
type APIKeyRecord struct {
	ID        int64
	GroupID   int64
	KeyHash   string
	Enabled   bool
	ExpiresAt *time.Time
}

// Store 是解析器依赖的最小存储接口，便于用 mock 或真实 PG 实现注入。
type Store interface {
	// LookupAPIKeyByPrefix 按前缀查出候选 Key 记录（前缀唯一）。
	LookupAPIKeyByPrefix(ctx context.Context, prefix string) (*APIKeyRecord, error)
	// LoadGroup 加载分组及其通道。
	LoadGroup(ctx context.Context, groupID int64) (*domain.Group, *domain.UpstreamChannel, error)
}

// Resolver 是 GroupResolver 的默认实现。
//
// 它不直接持有 Redis；缓存命中逻辑由调用方包装（见 internal/store/redis）。
// 这里专注"解析正确性"：前缀查询 → hash 比对 → 各种禁用/过期判定 → 构建策略与改写器。
type Resolver struct {
	store Store
}

// NewResolver 构造解析器。
func NewResolver(store Store) *Resolver { return &Resolver{store: store} }

// Resolve 实现 GroupResolver。
//
// 错误信息明确区分：key 不存在 / key 过期 / key 禁用 / group 禁用（任务书 §5.2 验收）。
func (r *Resolver) Resolve(ctx context.Context, apiKey string) (*ResolvedGroup, error) {
	prefix, secret, err := ParseAPIKey(apiKey)
	if err != nil {
		return nil, err
	}
	rec, err := r.store.LookupAPIKeyByPrefix(ctx, prefix)
	if err != nil {
		return nil, domain.ErrInvalidAPIKey.Wrap(err)
	}
	if rec == nil {
		return nil, domain.ErrInvalidAPIKey
	}
	if !VerifySecret(secret, rec.KeyHash) {
		return nil, domain.ErrInvalidAPIKey
	}
	if !rec.Enabled {
		return nil, domain.ErrAPIKeyDisabled
	}
	if rec.ExpiresAt != nil && rec.ExpiresAt.Before(time.Now()) {
		return nil, domain.ErrAPIKeyExpired
	}

	group, channel, err := r.store.LoadGroup(ctx, rec.GroupID)
	if err != nil {
		return nil, domain.ErrInternal.Wrap(err)
	}
	if group == nil || !group.Enabled {
		return nil, domain.ErrGroupDisabled
	}

	strategy, err := cache.New(group.CacheStrategy)
	if err != nil {
		return nil, domain.ErrInternal.Wrap(err)
	}

	return &ResolvedGroup{
		APIKeyID:      rec.ID,
		Group:         group,
		Channel:       channel,
		CacheStrategy: strategy,
		Transformers:  nil, // 由 transformer.Registry 按 group.TransformerConfig 构建
	}, nil
}
