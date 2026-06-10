package toolcall

import (
	"context"
	"testing"

	"github.com/claude-gate/claude-gate/internal/domain"
)

func TestNormalizerFillsEmptySchema(t *testing.T) {
	n := New()
	req := &domain.MessagesRequest{Tools: []domain.Tool{{Name: "get_weather"}}}
	out, err := n.TransformRequest(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Tools[0].InputSchema) == 0 {
		t.Fatal("缺失 input_schema 应被补齐")
	}
	// 原请求不应被修改（写时复制）
	if len(req.Tools[0].InputSchema) != 0 {
		t.Fatal("不应改动原始请求")
	}
}

func TestNormalizerKeepsExistingSchema(t *testing.T) {
	n := New()
	schema := []byte(`{"type":"object","properties":{"q":{}}}`)
	req := &domain.MessagesRequest{Tools: []domain.Tool{{Name: "x", InputSchema: schema}}}
	out, _ := n.TransformRequest(context.Background(), req)
	if string(out.Tools[0].InputSchema) != string(schema) {
		t.Fatal("已有 schema 不应被覆盖")
	}
}

func TestNormalizerNoTools(t *testing.T) {
	n := New()
	if n.Name() != "tool_call_normalizer" {
		t.Fatalf("name = %q", n.Name())
	}
	out, err := n.TransformRequest(context.Background(), &domain.MessagesRequest{})
	if err != nil || len(out.Tools) != 0 {
		t.Fatal("无工具时应原样返回")
	}
}
