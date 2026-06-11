package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/claude-gate/claude-gate/internal/auth"
	"github.com/claude-gate/claude-gate/internal/cache"
	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/ratelimit"
	"github.com/claude-gate/claude-gate/internal/upstream"
)

// stubAdapter 可注入失败次数与阻塞，用于测并发/重试。
type stubAdapter struct {
	failN   int
	calls   int
	started chan struct{}
	block   chan struct{}
	usage   domain.RawUsage
}

func (a *stubAdapter) Name() string { return "stub" }

func (a *stubAdapter) Send(ctx context.Context, _ *domain.MessagesRequest, _ *domain.UpstreamKey) (*domain.MessagesResponse, error) {
	a.calls++
	if a.calls <= a.failN {
		return nil, domain.ErrUpstreamFailure
	}
	if a.started != nil {
		a.started <- struct{}{}
	}
	if a.block != nil {
		<-a.block
	}
	u := a.usage
	return &domain.MessagesResponse{ID: "x", Type: "message", Role: "assistant", Content: json.RawMessage(`[]`), Usage: &u}, nil
}

func (a *stubAdapter) SendStream(ctx context.Context, _ *domain.MessagesRequest, _ *domain.UpstreamKey) (<-chan domain.StreamEvent, error) {
	ch := make(chan domain.StreamEvent)
	close(ch)
	return ch, nil
}

type stubResolver struct{ rg *auth.ResolvedGroup }

func (s stubResolver) Resolve(context.Context, string) (*auth.ResolvedGroup, error) { return s.rg, nil }

type stubPool struct{}

func (stubPool) Acquire(context.Context, int64) (*domain.UpstreamKey, error) {
	return &domain.UpstreamKey{ID: 1, Status: domain.KeyActive}, nil
}
func (stubPool) Release(*domain.UpstreamKey, upstream.Result) {}

type govOpts struct {
	global  int
	rpm     int
	retry   int
	backoff int
	strat   domain.CacheStrategyConfig
}

func newGovProxy(t *testing.T, ad upstream.Adapter, o govOpts) *Proxy {
	t.Helper()
	if o.strat.Type == "" {
		o.strat = domain.CacheStrategyConfig{Type: "passthrough"}
	}
	strat, err := cache.New(o.strat)
	if err != nil {
		t.Fatal(err)
	}
	rg := &auth.ResolvedGroup{
		APIKeyID:      1,
		Group:         &domain.Group{ID: 1, Enabled: true, CacheStrategy: o.strat, RateLimit: domain.RateLimitConfig{RPM: o.rpm}, Retry: domain.RetryConfig{MaxRetries: o.retry, BackoffMs: o.backoff}},
		Channel:       &domain.UpstreamChannel{ID: 1, Type: "stub", Enabled: true},
		CacheStrategy: strat,
	}
	reg := upstream.NewRegistry()
	reg.Register("stub", func(*domain.UpstreamChannel) (upstream.Adapter, error) { return ad, nil })
	return NewProxy(ProxyDeps{
		Resolver: stubResolver{rg}, Registry: reg, Pool: stubPool{},
		Limiter: ratelimit.NewMemory(), GlobalMaxInFlight: o.global, Timeout: 5 * time.Second,
	})
}

func govReq() *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`))
	r.Header.Set("Authorization", "Bearer cg-xxxxxxxx-yyyy")
	return withTraceReq(r)
}

// withTraceReq 模拟 withTrace 中间件注入 trace_id。
func withTraceReq(r *http.Request) *http.Request {
	return r // ServeMessages 内部用 observ.TraceID，空也可
}

// 全局并发背压：占满后新请求立即 429，不进入上游。
func TestGlobalBackpressure429(t *testing.T) {
	ad := &stubAdapter{started: make(chan struct{}, 1), block: make(chan struct{}), usage: domain.RawUsage{InputTokens: 10}}
	p := newGovProxy(t, ad, govOpts{global: 1})

	go p.ServeMessages(httptest.NewRecorder(), govReq())
	<-ad.started // 确保请求 A 已占住全局信号量并进入上游

	rec := httptest.NewRecorder()
	p.ServeMessages(rec, govReq())
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("并发占满后应 429，得到 %d", rec.Code)
	}
	close(ad.block)
	if ad.calls != 1 {
		t.Fatalf("被背压的请求不应到达上游，calls=%d", ad.calls)
	}
}

// rpm 限流：同窗口超过 rpm 的请求 429。
func TestRPMLimit(t *testing.T) {
	ad := &stubAdapter{usage: domain.RawUsage{InputTokens: 10}}
	p := newGovProxy(t, ad, govOpts{rpm: 1})

	rec1 := httptest.NewRecorder()
	p.ServeMessages(rec1, govReq())
	if rec1.Code != http.StatusOK {
		t.Fatalf("首个请求应 200，得到 %d", rec1.Code)
	}
	rec2 := httptest.NewRecorder()
	p.ServeMessages(rec2, govReq())
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("超过 rpm 应 429，得到 %d", rec2.Code)
	}
}

// 重试：上游前 N 次失败后成功。
func TestRetrySucceedsAfterFailures(t *testing.T) {
	ad := &stubAdapter{failN: 2, usage: domain.RawUsage{InputTokens: 10}}
	p := newGovProxy(t, ad, govOpts{retry: 3})

	rec := httptest.NewRecorder()
	p.ServeMessages(rec, govReq())
	if rec.Code != http.StatusOK {
		t.Fatalf("重试后应 200，得到 %d", rec.Code)
	}
	if ad.calls != 3 {
		t.Fatalf("应调用 3 次（失败2+成功1），实际 %d", ad.calls)
	}
}

// 计费 total 基于上游真实输入侧 token，而非请求字节估算。
func TestBillingUsesUpstreamTokens(t *testing.T) {
	ad := &stubAdapter{usage: domain.RawUsage{InputTokens: 1000, OutputTokens: 50}}
	p := newGovProxy(t, ad, govOpts{strat: domain.CacheStrategyConfig{
		Type: "percentage", Params: map[string]any{"cache_read_ratio": 1.0, "input_fixed_tokens": 1, "output_source": "upstream"},
	}})

	rec := httptest.NewRecorder()
	p.ServeMessages(rec, govReq())
	var resp struct {
		Usage struct {
			InputTokens          int `json:"input_tokens"`
			CacheReadInputTokens int `json:"cache_read_input_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析失败: %v, body=%s", err, rec.Body.String())
	}
	// total = 上游 input 1000；cache_read = floor(1000 * 1.0) = 1000；input 固定 1
	if resp.Usage.CacheReadInputTokens != 1000 {
		t.Fatalf("计费 cache_read 应基于上游 1000 token，得到 %d", resp.Usage.CacheReadInputTokens)
	}
	if resp.Usage.InputTokens != 1 {
		t.Fatalf("input 应固定 1，得到 %d", resp.Usage.InputTokens)
	}
}
