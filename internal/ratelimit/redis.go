package ratelimit

import (
	"context"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// RedisLimiter 是基于 Redis 的固定窗口实现（INCRBY + EXPIRE，跨实例一致）。
type RedisLimiter struct {
	rdb *goredis.Client
}

var _ Limiter = (*RedisLimiter)(nil)

// NewRedis 构造 Redis 限流器，复用现有连接池。
func NewRedis(rdb *goredis.Client) *RedisLimiter { return &RedisLimiter{rdb: rdb} }

// Incr 累加并返回窗口累计值；delta=0 时只读当前值。
func (l *RedisLimiter) Incr(ctx context.Context, key string, delta int, window time.Duration) (int, error) {
	if delta == 0 {
		v, err := l.rdb.Get(ctx, key).Int()
		if errors.Is(err, goredis.Nil) {
			return 0, nil
		}
		return v, err
	}
	pipe := l.rdb.Pipeline()
	n := pipe.IncrBy(ctx, key, int64(delta))
	pipe.Expire(ctx, key, window)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return int(n.Val()), nil
}
