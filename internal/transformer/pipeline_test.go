package transformer

import (
	"context"
	"errors"
	"testing"

	"github.com/claude-gate/claude-gate/internal/domain"
)

// fakeT 是用于测试的可控改写器。
type fakeT struct {
	Base
	name      string
	reqErr    error
	mutateReq func(*domain.MessagesRequest)
	dropEvent bool
}

func (f *fakeT) Name() string { return f.name }

func (f *fakeT) TransformRequest(_ context.Context, req *domain.MessagesRequest) (*domain.MessagesRequest, error) {
	if f.reqErr != nil {
		return nil, f.reqErr
	}
	if f.mutateReq != nil {
		clone := *req
		f.mutateReq(&clone)
		return &clone, nil
	}
	return req, nil
}

func (f *fakeT) TransformStreamEvent(_ context.Context, ev *domain.StreamEvent) (*domain.StreamEvent, error) {
	if f.dropEvent {
		return nil, nil
	}
	return ev, nil
}

func TestPipelineOrderedRequest(t *testing.T) {
	p := NewPipeline().
		Use(&fakeT{name: "a", mutateReq: func(r *domain.MessagesRequest) { r.Model += "-a" }}, FailFast).
		Use(&fakeT{name: "b", mutateReq: func(r *domain.MessagesRequest) { r.Model += "-b" }}, FailFast)
	if p.Len() != 2 {
		t.Fatalf("Len = %d", p.Len())
	}
	out, err := p.ApplyRequest(context.Background(), &domain.MessagesRequest{Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Model != "m-a-b" {
		t.Fatalf("顺序错误: %q", out.Model)
	}
}

func TestPipelineFailFast(t *testing.T) {
	p := NewPipeline().
		Use(&fakeT{name: "boom", reqErr: errors.New("x")}, FailFast).
		Use(&fakeT{name: "b", mutateReq: func(r *domain.MessagesRequest) { r.Model += "-b" }}, FailFast)
	_, err := p.ApplyRequest(context.Background(), &domain.MessagesRequest{Model: "m"})
	if err == nil {
		t.Fatal("fail-fast 应返回错误")
	}
}

func TestPipelineFailSkip(t *testing.T) {
	p := NewPipeline().
		Use(&fakeT{name: "boom", reqErr: errors.New("x")}, FailSkip).
		Use(&fakeT{name: "b", mutateReq: func(r *domain.MessagesRequest) { r.Model += "-b" }}, FailFast)
	out, err := p.ApplyRequest(context.Background(), &domain.MessagesRequest{Model: "m"})
	if err != nil {
		t.Fatalf("skip 模式不应中断: %v", err)
	}
	if out.Model != "m-b" {
		t.Fatalf("跳过失败改写器后结果错误: %q", out.Model)
	}
}

func TestPipelineStreamEventDrop(t *testing.T) {
	p := NewPipeline().Use(&fakeT{name: "drop", dropEvent: true}, FailFast)
	out, err := p.ApplyStreamEvent(context.Background(), &domain.StreamEvent{Event: "ping"})
	if err != nil {
		t.Fatal(err)
	}
	if out != nil {
		t.Fatal("事件应被丢弃")
	}
}

func TestPipelineResponsePath(t *testing.T) {
	// 响应路径：默认 Base 实现原样透传，验证不报错且保留内容
	p := NewPipeline().
		Use(&fakeT{name: "a"}, FailFast).
		Use(&fakeT{name: "b"}, FailSkip)
	resp := &domain.MessagesResponse{ID: "msg_1", Model: "claude"}
	out, err := p.ApplyResponse(context.Background(), resp)
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != "msg_1" {
		t.Fatalf("响应被破坏: %+v", out)
	}
}

func TestPipelineDefaultFailMode(t *testing.T) {
	// Use 传空 FailMode 应默认 fail-fast
	p := NewPipeline().Use(&fakeT{name: "boom", reqErr: errors.New("x")}, "")
	_, err := p.ApplyRequest(context.Background(), &domain.MessagesRequest{Model: "m"})
	if err == nil {
		t.Fatal("空 FailMode 应默认 fail-fast 并返回错误")
	}
}
