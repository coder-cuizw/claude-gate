package mock

import (
	"context"
	"testing"
	"time"

	"github.com/claude-gate/claude-gate/internal/domain"
)

func req() *domain.MessagesRequest {
	return &domain.MessagesRequest{Model: "m", Messages: []domain.Message{{Role: "user", Content: []byte(`"hi"`)}}}
}

func TestUnaryHasUsage(t *testing.T) {
	resp, err := New(nil).Send(context.Background(), req(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Usage == nil || resp.Usage.InputTokens == 0 {
		t.Fatal("应返回 usage")
	}
}

// 流式产出完整事件序列。
func TestStreamSequence(t *testing.T) {
	ch, err := New(nil).SendStream(context.Background(), req(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var events []string
	for ev := range ch {
		events = append(events, ev.Event)
	}
	want := []string{"message_start", "content_block_start", "content_block_delta", "content_block_stop", "message_delta", "message_stop"}
	if len(events) != len(want) {
		t.Fatalf("事件数 = %d, 期望 %d: %v", len(events), len(want), events)
	}
}

// context 取消时流式 goroutine 必须及时退出并关闭 channel（任务书 §11.5：无 goroutine 泄漏）。
func TestStreamContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := New(nil).SendStream(ctx, req(), nil)
	if err != nil {
		t.Fatal(err)
	}
	cancel() // 立即取消

	done := make(chan struct{})
	go func() {
		for range ch { // 取消后应迅速 drain 到关闭
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("取消后 channel 未关闭，疑似 goroutine 泄漏")
	}
}
