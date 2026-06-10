package keypool

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/upstream"
)

func mkKeys() []*domain.UpstreamKey {
	return []*domain.UpstreamKey{
		{ID: 1, ChannelID: 7, Status: domain.KeyActive},
		{ID: 2, ChannelID: 7, Status: domain.KeyActive},
		{ID: 3, ChannelID: 7, Status: domain.KeyActive},
	}
}

func TestRoundRobin(t *testing.T) {
	p := New(time.Minute)
	p.Load(7, mkKeys())
	var ids []int64
	for i := 0; i < 4; i++ {
		k, err := p.Acquire(context.Background(), 7)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, k.ID)
	}
	// 轮询应循环：1,2,3,1
	want := []int64{1, 2, 3, 1}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("轮询顺序 = %v, 期望 %v", ids, want)
		}
	}
}

func TestNoKeys(t *testing.T) {
	p := New(time.Minute)
	_, err := p.Acquire(context.Background(), 99)
	if !errors.Is(err, domain.ErrNoUpstreamKey) {
		t.Fatalf("无 Key 应返回 ErrNoUpstreamKey, 得到 %v", err)
	}
}

func TestCooldownOn5xx(t *testing.T) {
	p := New(5 * time.Minute)
	keys := mkKeys()
	p.Load(7, keys)
	k, _ := p.Acquire(context.Background(), 7)
	p.Release(k, upstream.Result{Success: false, StatusCode: 503, Err: errors.New("boom")})
	if k.Status != domain.KeyCooldown {
		t.Fatalf("5xx 后应进入 cooldown, 状态 = %s", k.Status)
	}
	if k.LastError == "" {
		t.Fatal("应记录 last_error")
	}
}

func TestCooldownRecovery(t *testing.T) {
	p := New(time.Minute)
	keys := []*domain.UpstreamKey{{ID: 1, ChannelID: 7, Status: domain.KeyActive}}
	p.Load(7, keys)

	// 用可控时钟把所有 Key 打入 cooldown
	base := time.Now()
	p.now = func() time.Time { return base }
	k, _ := p.Acquire(context.Background(), 7)
	p.Release(k, upstream.Result{StatusCode: 429})
	if _, err := p.Acquire(context.Background(), 7); !errors.Is(err, domain.ErrNoUpstreamKey) {
		t.Fatal("cooldown 期间应无可用 Key")
	}

	// 时间前进超过 cooldown，应自动恢复
	p.now = func() time.Time { return base.Add(2 * time.Minute) }
	if _, err := p.Acquire(context.Background(), 7); err != nil {
		t.Fatalf("cooldown 到期后应恢复可用: %v", err)
	}
}

func TestReleaseSuccessNoCooldown(t *testing.T) {
	p := New(time.Minute)
	keys := mkKeys()
	p.Load(7, keys)
	k, _ := p.Acquire(context.Background(), 7)
	p.Release(k, upstream.Result{Success: true, StatusCode: 200})
	if k.Status != domain.KeyActive {
		t.Fatal("成功调用不应改变 Key 状态")
	}
	p.Release(nil, upstream.Result{}) // nil 安全
}
