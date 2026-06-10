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
	name       string
	reqErr     error
	respErr    error
	streamErr  error
	mutateReq  func(*domain.MessagesRequest)
	mutateResp func(*domain.MessagesResponse)
	dropEvent  bool
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

func (f *fakeT) TransformResponse(_ context.Context, resp *domain.MessagesResponse) (*domain.MessagesResponse, error) {
	if f.respErr != nil {
		return nil, f.respErr
	}
	if f.mutateResp != nil {
		clone := *resp
		f.mutateResp(&clone)
		return &clone, nil
	}
	return resp, nil
}

func (f *fakeT) TransformStreamEvent(_ context.Context, ev *domain.StreamEvent) (*domain.StreamEvent, error) {
	if f.streamErr != nil {
		return nil, f.streamErr
	}
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
	// 响应路径：链式改写 model 字段
	p := NewPipeline().
		Use(&fakeT{name: "a", mutateResp: func(r *domain.MessagesResponse) { r.Model += "-a" }}, FailFast).
		Use(&fakeT{name: "b", mutateResp: func(r *domain.MessagesResponse) { r.Model += "-b" }}, FailSkip)
	out, err := p.ApplyResponse(context.Background(), &domain.MessagesResponse{ID: "msg_1", Model: "claude"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Model != "claude-a-b" {
		t.Fatalf("响应链式改写错误: %+v", out)
	}
}

func TestPipelineResponseFailFastAndSkip(t *testing.T) {
	// fail-fast：响应改写出错应中断
	pFast := NewPipeline().Use(&fakeT{name: "boom", respErr: errors.New("x")}, FailFast)
	if _, err := pFast.ApplyResponse(context.Background(), &domain.MessagesResponse{Model: "m"}); err == nil {
		t.Fatal("响应 fail-fast 应返回错误")
	}
	// skip：响应改写出错应跳过并继续
	pSkip := NewPipeline().
		Use(&fakeT{name: "boom", respErr: errors.New("x")}, FailSkip).
		Use(&fakeT{name: "b", mutateResp: func(r *domain.MessagesResponse) { r.Model += "-b" }}, FailFast)
	out, err := pSkip.ApplyResponse(context.Background(), &domain.MessagesResponse{Model: "m"})
	if err != nil || out.Model != "m-b" {
		t.Fatalf("响应 skip 处理错误: out=%+v err=%v", out, err)
	}
}

func TestPipelineStreamFailFastAndSkip(t *testing.T) {
	// fail-fast：流事件改写出错应中断
	pFast := NewPipeline().Use(&fakeT{name: "boom", streamErr: errors.New("x")}, FailFast)
	if _, err := pFast.ApplyStreamEvent(context.Background(), &domain.StreamEvent{Event: "ping"}); err == nil {
		t.Fatal("流事件 fail-fast 应返回错误")
	}
	// skip：流事件改写出错应跳过，保留原事件
	pSkip := NewPipeline().Use(&fakeT{name: "boom", streamErr: errors.New("x")}, FailSkip)
	out, err := pSkip.ApplyStreamEvent(context.Background(), &domain.StreamEvent{Event: "ping"})
	if err != nil || out == nil || out.Event != "ping" {
		t.Fatalf("流事件 skip 处理错误: out=%+v err=%v", out, err)
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
