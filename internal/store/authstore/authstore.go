// Package authstore 用 store.ConfigStore + store.Cache 实现 auth.Store。
//
// 热路径（API Key → Group 解析）优先命中 Redis，未命中再查配置存储并回填，
// 满足任务书 §5.2「热路径无 PG 查询（Redis 命中时）」与「Key 禁用后 60s 内生效」。
package authstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/claude-gate/claude-gate/internal/auth"
	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/store"
)

// CacheKeyPrefix 是 API Key 记录的 Redis 键前缀（任务书 §10：统一 cg:）。
const CacheKeyPrefix = "cg:apikey:"

// Adapter 实现 auth.Store。
type Adapter struct {
	cfg    store.ConfigStore
	cache  store.Cache
	ttlSec int
}

var _ auth.Store = (*Adapter)(nil)

// New 构造适配器。ttlSec<=0 时取默认 60s。
func New(cfg store.ConfigStore, cache store.Cache, ttlSec int) *Adapter {
	if ttlSec <= 0 {
		ttlSec = 60
	}
	return &Adapter{cfg: cfg, cache: cache, ttlSec: ttlSec}
}

// CacheKey 返回某前缀对应的缓存键。
func CacheKey(prefix string) string { return CacheKeyPrefix + prefix }

// LookupAPIKeyByPrefix 先查缓存，未命中查配置存储并回填。
func (a *Adapter) LookupAPIKeyByPrefix(ctx context.Context, prefix string) (*auth.APIKeyRecord, error) {
	key := CacheKey(prefix)
	if a.cache != nil {
		if raw, ok, _ := a.cache.Get(ctx, key); ok {
			var rec auth.APIKeyRecord
			if err := json.Unmarshal(raw, &rec); err == nil {
				if rec.ID == 0 { // 负缓存：前缀不存在
					return nil, nil
				}
				return &rec, nil
			}
		}
	}

	ak, err := a.cfg.GetAPIKeyByPrefix(ctx, prefix)
	if errors.Is(err, store.ErrNotFound) {
		a.setCache(ctx, key, &auth.APIKeyRecord{}) // 负缓存，挡住对不存在前缀的反复穿透
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查 API Key 失败: %w", err)
	}
	rec := &auth.APIKeyRecord{ID: ak.ID, GroupID: ak.GroupID, KeyHash: ak.KeyHash, Enabled: ak.Enabled, ExpiresAt: ak.ExpiresAt}
	a.setCache(ctx, key, rec)
	return rec, nil
}

func (a *Adapter) setCache(ctx context.Context, key string, rec *auth.APIKeyRecord) {
	if a.cache == nil {
		return
	}
	if b, err := json.Marshal(rec); err == nil {
		_ = a.cache.Set(ctx, key, b, a.ttlSec)
	}
}

// LoadGroup 加载分组及其通道。
func (a *Adapter) LoadGroup(ctx context.Context, groupID int64) (*domain.Group, *domain.UpstreamChannel, error) {
	g, err := a.cfg.GetGroup(ctx, groupID)
	if err != nil {
		return nil, nil, err
	}
	ch, err := a.cfg.GetChannel(ctx, g.ChannelID)
	if err != nil {
		return g, nil, err
	}
	return g, ch, nil
}

// Invalidate 在配置变更时主动失效某前缀缓存（任务书 §5.2）。
func (a *Adapter) Invalidate(ctx context.Context, prefix string) {
	if a.cache != nil {
		_ = a.cache.Del(ctx, CacheKey(prefix))
	}
}
