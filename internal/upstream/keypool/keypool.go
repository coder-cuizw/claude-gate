// Package keypool 实现上游 Key 池的调度（任务书 §5.5）。
//
// MVP 策略：
//   - 轮询选取 status=active 的 Key；
//   - 收到 429/5xx → status=cooldown，cooldown_until = now + cooldownDur；
//   - 后台 goroutine 周期检查 cooldown 到期的 Key 自动恢复。
package keypool

import (
	"context"
	"sync"
	"time"

	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/upstream"
)

// MemoryPool 是基于内存的 Key 池实现，便于单测与单机部署。
//
// 生产中可用 PG/Redis 持久化状态后实现同一 upstream.KeyPool 接口。
type MemoryPool struct {
	mu         sync.Mutex
	keys       map[int64][]*domain.UpstreamKey // channelID -> keys
	rr         map[int64]int                   // channelID -> 轮询游标
	cooldown   time.Duration
	now        func() time.Time // 便于测试注入时间
}

// 确保实现接口。
var _ upstream.KeyPool = (*MemoryPool)(nil)

// New 构造内存 Key 池。cooldown<=0 时使用默认 5 分钟（任务书 §5.5）。
func New(cooldown time.Duration) *MemoryPool {
	if cooldown <= 0 {
		cooldown = 5 * time.Minute
	}
	return &MemoryPool{
		keys:     make(map[int64][]*domain.UpstreamKey),
		rr:       make(map[int64]int),
		cooldown: cooldown,
		now:      time.Now,
	}
}

// Load 加载某通道的 Key 列表。
func (p *MemoryPool) Load(channelID int64, keys []*domain.UpstreamKey) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.keys[channelID] = keys
	p.rr[channelID] = 0
}

// Acquire 轮询取一把可用 Key。无可用时返回 domain.ErrNoUpstreamKey。
func (p *MemoryPool) Acquire(_ context.Context, channelID int64) (*domain.UpstreamKey, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.recoverExpiredLocked(channelID)

	list := p.keys[channelID]
	n := len(list)
	if n == 0 {
		return nil, domain.ErrNoUpstreamKey
	}
	// 从游标开始轮询一圈，找第一把 active。
	start := p.rr[channelID]
	for i := 0; i < n; i++ {
		idx := (start + i) % n
		k := list[idx]
		if k.Status == domain.KeyActive {
			p.rr[channelID] = (idx + 1) % n
			t := p.now()
			k.LastUsedAt = &t
			return k, nil
		}
	}
	return nil, domain.ErrNoUpstreamKey
}

// Release 上报调用结果；429/5xx 触发该 Key 进入 cooldown。
func (p *MemoryPool) Release(key *domain.UpstreamKey, result upstream.Result) {
	if key == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if result.Success {
		return
	}
	if result.StatusCode == 429 || result.StatusCode >= 500 {
		until := p.now().Add(p.cooldown)
		key.Status = domain.KeyCooldown
		key.CooldownUntil = &until
		if result.Err != nil {
			key.LastError = result.Err.Error()
		}
	}
}

// recoverExpiredLocked 把 cooldown 到期的 Key 恢复为 active（调用方须持锁）。
func (p *MemoryPool) recoverExpiredLocked(channelID int64) {
	now := p.now()
	for _, k := range p.keys[channelID] {
		if k.Status == domain.KeyCooldown && k.CooldownUntil != nil && now.After(*k.CooldownUntil) {
			k.Status = domain.KeyActive
			k.CooldownUntil = nil
		}
	}
}
