package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/store"
)

// 集成测试：需真实 PG。设 CG_TEST_PG_DSN 后运行；未设则跳过。
func testStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("CG_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("未设 CG_TEST_PG_DSN，跳过 PG 集成测试")
	}
	s, err := New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("连接 PG 失败: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

func TestChannelAndGroupRoundTrip(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	ch := &domain.UpstreamChannel{Name: "测试通道", Type: domain.ChannelOfficial, BaseURL: "https://x", Enabled: true,
		Config: map[string]any{"anthropic_version": "2023-06-01"}}
	if err := s.CreateChannel(ctx, ch); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.DeleteChannel(ctx, ch.ID) })
	if ch.ID == 0 {
		t.Fatal("应回填 ID")
	}

	got, err := s.GetChannel(ctx, ch.ID)
	if err != nil || got.Config["anthropic_version"] != "2023-06-01" {
		t.Fatalf("JSONB config 往返失败: %v %+v", err, got)
	}

	// 分组含四类 JSONB 策略，校验往返
	g := &domain.Group{Name: "测试分组" + time.Now().Format("150405.000"), ChannelID: ch.ID, Enabled: true,
		CacheStrategy:     domain.CacheStrategyConfig{Type: "percentage", Params: map[string]any{"cache_read_ratio": 0.9}},
		TransformerConfig: []domain.TransformerConfig{{Name: "model_mapper", Enabled: true}},
		RateLimit:         domain.RateLimitConfig{RPM: 600}}
	if err := s.CreateGroup(ctx, g); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.DeleteGroup(ctx, g.ID) })
	g2, err := s.GetGroup(ctx, g.ID)
	if err != nil || g2.CacheStrategy.Type != "percentage" || g2.RateLimit.RPM != 600 {
		t.Fatalf("分组 JSONB 往返失败: %v %+v", err, g2)
	}
	if len(g2.TransformerConfig) != 1 || g2.TransformerConfig[0].Name != "model_mapper" {
		t.Fatalf("transformer 往返失败: %+v", g2.TransformerConfig)
	}
}

func TestAPIKeyByPrefix(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	ch := &domain.UpstreamChannel{Name: "k通道", Type: domain.ChannelCustom, Enabled: true, Config: map[string]any{}}
	_ = s.CreateChannel(ctx, ch)
	t.Cleanup(func() { _ = s.DeleteChannel(ctx, ch.ID) })
	g := &domain.Group{Name: "kg" + time.Now().Format("150405.000"), ChannelID: ch.ID, Enabled: true, CacheStrategy: domain.CacheStrategyConfig{Type: "passthrough"}}
	_ = s.CreateGroup(ctx, g)
	t.Cleanup(func() { _ = s.DeleteGroup(ctx, g.ID) })

	prefix := "ptest" + time.Now().Format("05")
	k := &domain.APIKey{KeyPrefix: prefix, KeyHash: "h", KeyEncrypted: "enc", Name: "k", GroupID: g.ID, Enabled: true}
	if err := s.CreateAPIKey(ctx, k); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.DeleteAPIKey(ctx, k.ID) })

	got, err := s.GetAPIKeyByPrefix(ctx, prefix)
	if err != nil || got.GroupID != g.ID || got.KeyEncrypted != "enc" {
		t.Fatalf("按前缀查找/可逆密文往返失败: %v %+v", err, got)
	}
	if _, err := s.GetAPIKeyByPrefix(ctx, "no-such-prefix"); err != store.ErrNotFound {
		t.Fatalf("不存在前缀应 ErrNotFound, 得到 %v", err)
	}
}
