package streaming

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/claude-gate/claude-gate/internal/domain"
)

func TestFixerBackfillsEventName(t *testing.T) {
	f := New()
	ev := &domain.StreamEvent{Data: json.RawMessage(`{"type":"message_delta"}`)}
	out, err := f.TransformStreamEvent(context.Background(), ev)
	if err != nil {
		t.Fatal(err)
	}
	if out.Event != "message_delta" {
		t.Fatalf("event 名应被补全, 得到 %q", out.Event)
	}
}

func TestFixerDropsEmpty(t *testing.T) {
	f := New()
	out, _ := f.TransformStreamEvent(context.Background(), &domain.StreamEvent{})
	if out != nil {
		t.Fatal("空事件应被丢弃")
	}
}

func TestFixerKeepsValid(t *testing.T) {
	f := New()
	if f.Name() != "streaming_event_fixer" {
		t.Fatalf("name = %q", f.Name())
	}
	ev := &domain.StreamEvent{Event: "ping", Data: json.RawMessage(`{}`)}
	out, _ := f.TransformStreamEvent(context.Background(), ev)
	if out == nil || out.Event != "ping" {
		t.Fatal("合法事件应原样保留")
	}
}

func TestFixerNilEvent(t *testing.T) {
	f := New()
	out, _ := f.TransformStreamEvent(context.Background(), nil)
	if out != nil {
		t.Fatal("nil 事件应返回 nil")
	}
}
