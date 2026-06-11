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

// 中间层定位：disabled 的 Key 不被选中。
func TestSkipDisabled(t *testing.T) {
	p := New(0)
	p.Load(7, []*domain.UpstreamKey{
		{ID: 1, ChannelID: 7, Status: domain.KeyDisabled},
		{ID: 2, ChannelID: 7, Status: domain.KeyActive},
	})
	k, err := p.Acquire(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if k.ID != 2 {
		t.Fatalf("应跳过 disabled，选中 active(ID=2)，得到 %d", k.ID)
	}
}

// 全部 disabled 时无可用 Key。
func TestAllDisabled(t *testing.T) {
	p := New(0)
	p.Load(7, []*domain.UpstreamKey{{ID: 1, ChannelID: 7, Status: domain.KeyDisabled}})
	if _, err := p.Acquire(context.Background(), 7); !errors.Is(err, domain.ErrNoUpstreamKey) {
		t.Fatalf("全部 disabled 应返回 ErrNoUpstreamKey，得到 %v", err)
	}
}

// 失败仅被动记录 last_error，不改变状态（不做号池管理/冷却）。
func TestReleaseRecordsErrorOnly(t *testing.T) {
	p := New(0)
	p.Load(7, mkKeys())
	k, _ := p.Acquire(context.Background(), 7)
	p.Release(k, upstream.Result{Success: false, StatusCode: 503, Err: errors.New("boom")})
	if k.Status != domain.KeyActive {
		t.Fatalf("失败不应改变 Key 状态（中间层不做冷却），状态 = %s", k.Status)
	}
	if k.LastError == "" {
		t.Fatal("应被动记录 last_error")
	}
	p.Release(nil, upstream.Result{})                      // nil 安全
	p.Release(k, upstream.Result{Success: true})           // 成功不记录
}
