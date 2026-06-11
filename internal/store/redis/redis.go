// Package redis 用 go-redis 实现 store.Cache（热路径缓存 / API Key→Group 解析）。
package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/claude-gate/claude-gate/internal/store"
)

// Cache 是基于 Redis 的缓存实现。
type Cache struct {
	rdb *goredis.Client
}

var _ store.Cache = (*Cache)(nil)

// New 连接 Redis 并探活。
func New(ctx context.Context, addr, password string, db int) (*Cache, error) {
	rdb := goredis.NewClient(&goredis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		PoolSize:     50,
		MinIdleConns: 5,
	})
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("Redis 连接探活失败: %w", err)
	}
	return &Cache{rdb: rdb}, nil
}

// Get 读取键值；不存在返回 found=false。
func (c *Cache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	b, err := c.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

// Set 写入键值；ttlSeconds<=0 表示不过期。
func (c *Cache) Set(ctx context.Context, key string, val []byte, ttlSeconds int) error {
	var ttl time.Duration
	if ttlSeconds > 0 {
		ttl = time.Duration(ttlSeconds) * time.Second
	}
	return c.rdb.Set(ctx, key, val, ttl).Err()
}

// Del 删除键。
func (c *Cache) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return c.rdb.Del(ctx, keys...).Err()
}

// Client 暴露底层客户端，供限流器等复用同一连接池。
func (c *Cache) Client() *goredis.Client { return c.rdb }

// Ping 供 readyz 探活。
func (c *Cache) Ping(ctx context.Context) error { return c.rdb.Ping(ctx).Err() }

// Close 关闭连接。
func (c *Cache) Close() error { return c.rdb.Close() }
