package s3

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"
)

// 集成测试：需真实 MinIO/S3。设 CG_TEST_S3_ENDPOINT 后运行；未设则跳过。
func TestPutGet(t *testing.T) {
	ep := os.Getenv("CG_TEST_S3_ENDPOINT")
	if ep == "" {
		t.Skip("未设 CG_TEST_S3_ENDPOINT，跳过 S3 集成测试")
	}
	ctx := context.Background()
	s, err := New(ctx, Options{
		Endpoint:  ep,
		AccessKey: envOr("CG_TEST_S3_AK", "minio"),
		SecretKey: envOr("CG_TEST_S3_SK", "minio123"),
		Bucket:    "claude-gate-test",
	})
	if err != nil {
		t.Fatalf("连接 S3 失败: %v", err)
	}

	body := []byte(`{"model":"m","messages":[{"role":"user","content":"你好"}]}`)
	trace := "s3test-" + time.Now().Format("150405.000000")
	key, err := s.Put(ctx, trace, "request", body)
	if err != nil {
		t.Fatal(err)
	}
	if key == "" {
		t.Fatal("应返回 key")
	}

	got, err := s.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("body 往返不一致:\n%s\n%s", got, body)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
