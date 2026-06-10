package modelmap

import (
	"context"
	"testing"

	"github.com/claude-gate/claude-gate/internal/domain"
)

func TestModelMapperHit(t *testing.T) {
	m := New(map[string]string{"claude-3-5-sonnet": "anthropic.claude-3-5-sonnet-v2"})
	out, err := m.TransformRequest(context.Background(), &domain.MessagesRequest{Model: "claude-3-5-sonnet"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Model != "anthropic.claude-3-5-sonnet-v2" {
		t.Fatalf("映射未生效: %q", out.Model)
	}
}

func TestModelMapperMiss(t *testing.T) {
	m := New(map[string]string{"a": "b"})
	out, _ := m.TransformRequest(context.Background(), &domain.MessagesRequest{Model: "unknown"})
	if out.Model != "unknown" {
		t.Fatalf("未命中应保持原样: %q", out.Model)
	}
}

func TestModelMapperNilSafe(t *testing.T) {
	m := New(nil)
	if m.Name() != "model_mapper" {
		t.Fatalf("name = %q", m.Name())
	}
	out, err := m.TransformRequest(context.Background(), nil)
	if err != nil || out != nil {
		t.Fatalf("nil 请求应安全返回")
	}
}
