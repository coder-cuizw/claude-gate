// Package ratelimit 提供按分组的固定窗口限流（rpm / tpm，任务书 §5.2 / §2.1）。
//
// 两种实现：MemoryLimiter（单机自测）与 RedisLimiter（分布式，见 redis.go）。
// 都满足 Limiter 接口，由 app 按存储模式注入。
package ratelimit

import (
	"context"
	"sync"
	"time"
)

// Limiter 是固定窗口计数器。
type Limiter interface {
	// Incr 在 window 窗口内对 key 累加 delta，返回累加后的窗口累计值。
	// delta=0 用于只读当前累计（如 tpm 事前检查）。
	Incr(ctx context.Context, key string, delta int, window time.Duration) (int, error)
}

// MemoryLimiter 是内存固定窗口实现，带后台过期清理。
type MemoryLimiter struct {
	mu   sync.Mutex
	m    map[string]*bucket
	stop chan struct{}
}

type bucket struct {
	val int
	exp time.Time
}

// NewMemory 构造内存限流器并启动清理协程。
func NewMemory() *MemoryLimiter {
	l := &MemoryLimiter{m: make(map[string]*bucket), stop: make(chan struct{})}
	go l.sweep()
	return l
}

// Incr 累加并返回窗口累计值。
func (l *MemoryLimiter) Incr(_ context.Context, key string, delta int, window time.Duration) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	b := l.m[key]
	if b == nil || now.After(b.exp) {
		b = &bucket{exp: now.Add(window)}
		l.m[key] = b
	}
	b.val += delta
	return b.val, nil
}

func (l *MemoryLimiter) sweep() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			now := time.Now()
			l.mu.Lock()
			for k, b := range l.m {
				if now.After(b.exp) {
					delete(l.m, k)
				}
			}
			l.mu.Unlock()
		case <-l.stop:
			return
		}
	}
}

// Close 停止清理协程。
func (l *MemoryLimiter) Close() { close(l.stop) }
