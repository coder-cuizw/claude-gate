package redis

import (
	"context"
	"os"
	"testing"
	"time"
)

// 集成测试：需真实 Redis。设 CG_TEST_REDIS_ADDR 后运行；未设则跳过。
func TestCacheRoundTrip(t *testing.T) {
	addr := os.Getenv("CG_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("未设 CG_TEST_REDIS_ADDR，跳过 Redis 集成测试")
	}
	ctx := context.Background()
	c, err := New(ctx, addr, "", 0)
	if err != nil {
		t.Fatalf("连接 Redis 失败: %v", err)
	}
	defer c.Close()

	key := "cg:test:" + time.Now().Format("150405.000")
	defer c.Del(ctx, key)

	if _, ok, _ := c.Get(ctx, key); ok {
		t.Fatal("初始不应命中")
	}
	if err := c.Set(ctx, key, []byte("hello"), 60); err != nil {
		t.Fatal(err)
	}
	v, ok, err := c.Get(ctx, key)
	if err != nil || !ok || string(v) != "hello" {
		t.Fatalf("应命中 hello: %v %v %q", err, ok, v)
	}
	if err := c.Del(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := c.Get(ctx, key); ok {
		t.Fatal("删除后不应命中")
	}

	// TTL 过期
	short := key + ":ttl"
	_ = c.Set(ctx, short, []byte("x"), 1)
	time.Sleep(1200 * time.Millisecond)
	if _, ok, _ := c.Get(ctx, short); ok {
		t.Fatal("TTL 到期后不应命中")
	}
}
