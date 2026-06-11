package memory

import (
	"context"
	"sync"
	"time"

	"github.com/claude-gate/claude-gate/internal/store"
)

// Cache 是内存版热路径缓存，实现 store.Cache（带 TTL）。
type Cache struct {
	mu    sync.RWMutex
	items map[string]cacheItem
}

type cacheItem struct {
	val    []byte
	expire time.Time // 零值表示永不过期
}

var _ store.Cache = (*Cache)(nil)

// NewCache 构造内存缓存。
func NewCache() *Cache { return &Cache{items: map[string]cacheItem{}} }

// Get 读取键值，过期或不存在返回 found=false。
func (c *Cache) Get(_ context.Context, key string) ([]byte, bool, error) {
	c.mu.RLock()
	it, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false, nil
	}
	if !it.expire.IsZero() && time.Now().After(it.expire) {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		return nil, false, nil
	}
	return it.val, true, nil
}

// Set 写入键值，ttlSeconds<=0 表示永不过期。
func (c *Cache) Set(_ context.Context, key string, val []byte, ttlSeconds int) error {
	var exp time.Time
	if ttlSeconds > 0 {
		exp = time.Now().Add(time.Duration(ttlSeconds) * time.Second)
	}
	c.mu.Lock()
	c.items[key] = cacheItem{val: val, expire: exp}
	c.mu.Unlock()
	return nil
}

// Del 删除键。
func (c *Cache) Del(_ context.Context, keys ...string) error {
	c.mu.Lock()
	for _, k := range keys {
		delete(c.items, k)
	}
	c.mu.Unlock()
	return nil
}
