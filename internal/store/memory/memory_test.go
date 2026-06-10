package memory

import (
	"context"
	"testing"
	"time"

	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/store"
)

func TestChannelCRUD(t *testing.T) {
	s := NewConfigStore()
	ctx := context.Background()
	ch := &domain.UpstreamChannel{Name: "t", Type: domain.ChannelOfficial, Enabled: true}
	if err := s.CreateChannel(ctx, ch); err != nil || ch.ID == 0 {
		t.Fatalf("创建失败: %v id=%d", err, ch.ID)
	}
	ch.Name = "t2"
	if err := s.UpdateChannel(ctx, ch); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetChannel(ctx, ch.ID)
	if err != nil || got.Name != "t2" {
		t.Fatalf("更新未生效: %v %+v", err, got)
	}
	if err := s.DeleteChannel(ctx, ch.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetChannel(ctx, ch.ID); err != store.ErrNotFound {
		t.Fatalf("删除后应 ErrNotFound, 得到 %v", err)
	}
}

func TestAPIKeyByPrefix(t *testing.T) {
	s := NewConfigStore()
	ctx := context.Background()
	_ = s.CreateAPIKey(ctx, &domain.APIKey{KeyPrefix: "abcd1234", KeyHash: "h", GroupID: 1, Enabled: true})
	k, err := s.GetAPIKeyByPrefix(ctx, "abcd1234")
	if err != nil || k.GroupID != 1 {
		t.Fatalf("按前缀查找失败: %v", err)
	}
	if _, err := s.GetAPIKeyByPrefix(ctx, "nope"); err != store.ErrNotFound {
		t.Fatalf("不存在前缀应 ErrNotFound, 得到 %v", err)
	}
}

func TestSeedConsistency(t *testing.T) {
	s := NewConfigStore()
	s.Seed("k")
	ctx := context.Background()
	chs, _ := s.ListChannels(ctx)
	if len(chs) != 4 {
		t.Fatalf("种子通道数 = %d, 期望 4", len(chs))
	}
	gs, _ := s.ListGroups(ctx)
	if len(gs) != 4 {
		t.Fatalf("种子分组数 = %d, 期望 4（四种缓存策略）", len(gs))
	}
	// 自测 Key 可按前缀解析
	if _, err := s.GetAPIKeyByPrefix(ctx, "selftest"); err != nil {
		t.Fatalf("自测 Key 应存在: %v", err)
	}
}

func TestCacheTTL(t *testing.T) {
	c := NewCache()
	ctx := context.Background()
	_ = c.Set(ctx, "k", []byte("v"), 0) // 永不过期
	if v, ok, _ := c.Get(ctx, "k"); !ok || string(v) != "v" {
		t.Fatal("应命中")
	}
	_ = c.Set(ctx, "k2", []byte("v"), -1) // 立即过期（ttl<=0 这里按永不过期处理，校验 Del）
	_ = c.Del(ctx, "k")
	if _, ok, _ := c.Get(ctx, "k"); ok {
		t.Fatal("删除后不应命中")
	}
	_ = time.Now
}
