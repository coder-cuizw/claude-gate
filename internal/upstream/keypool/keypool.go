// Package keypool 在同一通道的多把上游 Key 间做简单轮询选择。
//
// 设计定位（任务书裁剪）：claude-gate 只做中间层，**不做号池管理**。
// 这里不维护冷却状态机、不刷新令牌、不做账号健康调度——Key 由使用方在外部
// 开好后直接配置，网关只负责在 status=active 的 Key 间轮询转发。
package keypool

import (
	"context"
	"sync"
	"time"

	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/upstream"
)

// MemoryPool 是内存版 Key 轮询选择器。
type MemoryPool struct {
	mu   sync.Mutex
	keys map[int64][]*domain.UpstreamKey // channelID -> keys
	rr   map[int64]int                   // channelID -> 轮询游标
}

var _ upstream.KeyPool = (*MemoryPool)(nil)

// New 构造内存 Key 选择器。保留 cooldown 入参仅为兼容旧调用，当前不使用。
func New(_ time.Duration) *MemoryPool {
	return &MemoryPool{
		keys: make(map[int64][]*domain.UpstreamKey),
		rr:   make(map[int64]int),
	}
}

// Load 加载某通道的 Key 列表。
func (p *MemoryPool) Load(channelID int64, keys []*domain.UpstreamKey) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.keys[channelID] = keys
	p.rr[channelID] = 0
}

// Acquire 轮询取一把 active 的 Key。无可用时返回 domain.ErrNoUpstreamKey。
func (p *MemoryPool) Acquire(_ context.Context, channelID int64) (*domain.UpstreamKey, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	list := p.keys[channelID]
	n := len(list)
	if n == 0 {
		return nil, domain.ErrNoUpstreamKey
	}
	start := p.rr[channelID]
	for i := 0; i < n; i++ {
		idx := (start + i) % n
		k := list[idx]
		if k.Status == domain.KeyActive {
			p.rr[channelID] = (idx + 1) % n
			t := time.Now()
			k.LastUsedAt = &t
			return k, nil
		}
	}
	return nil, domain.ErrNoUpstreamKey
}

// Release 上报调用结果。中间层定位下仅被动记录最近错误，不做冷却/降级。
func (p *MemoryPool) Release(key *domain.UpstreamKey, result upstream.Result) {
	if key == nil || result.Success || result.Err == nil {
		return
	}
	p.mu.Lock()
	key.LastError = result.Err.Error()
	p.mu.Unlock()
}
