package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/claude-gate/claude-gate/internal/auth"
	"github.com/claude-gate/claude-gate/internal/cache"
	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/observ"
	"github.com/claude-gate/claude-gate/internal/ratelimit"
	"github.com/claude-gate/claude-gate/internal/store"
	"github.com/claude-gate/claude-gate/internal/transformer"
	"github.com/claude-gate/claude-gate/internal/transformer/factory"
	"github.com/claude-gate/claude-gate/internal/upstream"
)

const maxBodyBytes = 16 << 20 // 单请求体上限 16MB

// ProxyHandler 是网关代理入口（供 Server 路由）。
type ProxyHandler interface {
	ServeMessages(w http.ResponseWriter, r *http.Request)
	ServeModels(w http.ResponseWriter, r *http.Request)
}

// ProxyDeps 是代理引擎的依赖（全部接口注入，便于测试）。
type ProxyDeps struct {
	Resolver      auth.GroupResolver
	Registry      *upstream.Registry
	Pool          upstream.KeyPool
	ConfigStore   store.ConfigStore
	Sink          observ.Sink
	Bodies        observ.BodyStore
	Logger        *slog.Logger
	SampleSuccess float64       // 成功请求 body 落盘采样率
	Timeout       time.Duration // 全链路超时

	// 并发治理与限流（任务书 §2.1 / §5.2）
	GlobalMaxInFlight  int               // 全局并发上限，超出快速 429（背压，不落库）
	PerChannelInFlight int               // 每通道并发上限
	Limiter            ratelimit.Limiter // 分组 rpm/tpm 限流；nil 表示不限流
	WorkerPoolSize     int               // 异步落库 worker 数（body+明细），默认 16
	S3WriteRetry       int               // body 落盘失败重试次数
}

// Proxy 是端到端代理引擎，实现完整主链路（任务书 §3 / §5.1）：
//
//	认证 → 加载分组 → 改写 → 选 Adapter + 取 Key → 调上游 →
//	缓存计费改写 usage → 流式/非流式回写 → 明细与 body 落库
type Proxy struct {
	d           ProxyDeps
	globalSem   chan struct{}  // 全局并发信号量
	chanSems    sync.Map       // channelID -> chan struct{}，每通道并发信号量
	finishQueue chan finishJob // 异步落库队列
	finishWG    sync.WaitGroup
	dropped     atomic.Int64
}

// finishJob 是一次请求收尾（body 落盘 + 明细落库）的异步任务。
type finishJob struct {
	rec      *observ.RequestRecord
	reqBody  []byte
	respBody []byte
}

var _ ProxyHandler = (*Proxy)(nil)

// NewProxy 构造代理引擎并启动异步落库 worker pool。
func NewProxy(d ProxyDeps) *Proxy {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	if d.Timeout <= 0 {
		d.Timeout = 600 * time.Second
	}
	p := &Proxy{d: d}
	if d.GlobalMaxInFlight > 0 {
		p.globalSem = make(chan struct{}, d.GlobalMaxInFlight)
	}
	n := d.WorkerPoolSize
	if n <= 0 {
		n = 16
	}
	p.finishQueue = make(chan finishJob, n*64)
	for i := 0; i < n; i++ {
		p.finishWG.Add(1)
		go p.finishWorker()
	}
	return p
}

func (p *Proxy) finishWorker() {
	defer p.finishWG.Done()
	for job := range p.finishQueue {
		p.doFinish(job.rec, job.reqBody, job.respBody)
	}
}

// Close 停止 worker pool 并等待落库队列排空（flush）。
func (p *Proxy) Close(ctx context.Context) error {
	close(p.finishQueue)
	done := make(chan struct{})
	go func() { p.finishWG.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
	}
	if d := p.dropped.Load(); d > 0 {
		p.d.Logger.Warn("落库队列降级丢弃计数", "dropped", d)
	}
	return nil
}

// acquireChannel 取每通道并发槽位；ok=false 表示已达上限。
func (p *Proxy) acquireChannel(channelID int64) (release func(), ok bool) {
	if p.d.PerChannelInFlight <= 0 {
		return func() {}, true
	}
	v, _ := p.chanSems.LoadOrStore(channelID, make(chan struct{}, p.d.PerChannelInFlight))
	sem := v.(chan struct{})
	select {
	case sem <- struct{}{}:
		return func() { <-sem }, true
	default:
		return func() {}, false
	}
}

// ServeMessages 处理 POST /v1/messages（流式与非流式合一）。
func (p *Proxy) ServeMessages(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	traceID := observ.TraceID(r.Context())

	// 0. 全局并发背压：超上限立即 429，不进入主链路、不落库，避免雪崩放大压力（§2.1）
	if p.globalSem != nil {
		select {
		case p.globalSem <- struct{}{}:
			defer func() { <-p.globalSem }()
		default:
			writeError(w, traceID, domain.ErrTooManyInFlight)
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), p.d.Timeout)
	defer cancel()

	rec := &observ.RequestRecord{TraceID: traceID, RequestAt: start}
	var reqBody, respBody []byte
	var tpmKey string
	defer func() { p.submitFinish(rec, reqBody, respBody) }()
	// tpm 事后记账：按上游真实 token 累加到当前分钟窗口
	defer func() {
		if tpmKey != "" && p.d.Limiter != nil {
			if n := rec.Upstream.Total(); n > 0 {
				_, _ = p.d.Limiter.Incr(context.WithoutCancel(ctx), tpmKey, n, 65*time.Second)
			}
		}
	}()

	fail := func(err error) {
		de, _ := domain.AsError(err)
		if de == nil {
			de = domain.ErrInternal
		}
		rec.IsSuccess = false
		rec.StatusCode = de.HTTPStatus
		rec.ErrorType = de.Code
		rec.ErrorMessage = de.UserMessage
		writeError(w, traceID, de)
	}

	// 1. 认证
	apiKey := bearerToken(r.Header.Get("Authorization"))
	if apiKey == "" {
		fail(domain.ErrMissingAPIKey)
		return
	}
	resolved, err := p.d.Resolver.Resolve(ctx, apiKey)
	if err != nil {
		fail(err)
		return
	}
	rec.APIKeyID = resolved.APIKeyID
	rec.GroupID = resolved.Group.ID
	rec.ChannelID = resolved.Channel.ID
	rec.ChannelType = resolved.Channel.Type

	// 1.1 每通道并发上限
	releaseChan, ok := p.acquireChannel(resolved.Channel.ID)
	if !ok {
		fail(domain.ErrTooManyInFlight.WithMessage("通道并发已满，请稍后重试"))
		return
	}
	defer releaseChan()

	// 1.2 分组 rpm 限流（固定窗口）
	if rpm := resolved.Group.RateLimit.RPM; rpm > 0 && p.d.Limiter != nil {
		key := fmt.Sprintf("cg:rl:rpm:%d:%d", resolved.Group.ID, time.Now().Unix()/60)
		if cur, err := p.d.Limiter.Incr(ctx, key, 1, 65*time.Second); err == nil && cur > rpm {
			fail(domain.ErrRateLimited.WithMessage("请求频率超过分组 rpm 限制"))
			return
		}
		// tpm 事前检查：当前分钟已消耗 token 超阈值则拒（事后在 finish 中累加）
		if tpm := resolved.Group.RateLimit.TPM; tpm > 0 {
			tkey := fmt.Sprintf("cg:rl:tpm:%d:%d", resolved.Group.ID, time.Now().Unix()/60)
			if used, err := p.d.Limiter.Incr(ctx, tkey, 0, 65*time.Second); err == nil && used >= tpm {
				fail(domain.ErrRateLimited.WithMessage("已超过分组 tpm 限制"))
				return
			}
			tpmKey = tkey // 供 defer 事后累加
		}
	}

	// 2. 读取并留存原始 body
	reqBody, err = io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		fail(domain.ErrInvalidRequest.Wrap(err))
		return
	}
	var req domain.MessagesRequest
	if err := json.Unmarshal(reqBody, &req); err != nil {
		fail(domain.ErrInvalidRequest.Wrap(err))
		return
	}
	rec.Model = req.Model

	// 3. 请求改写流水线
	mappings := p.modelMapping(ctx, resolved.Channel.ID)
	pipe := factory.Build(resolved.Group.TransformerConfig, mappings)
	reqT, err := pipe.ApplyRequest(ctx, &req)
	if err != nil {
		fail(domain.ErrInvalidRequest.Wrap(err))
		return
	}
	req = *reqT

	// 4. 选 Adapter + 取 Key
	adapter, err := p.d.Registry.Build(resolved.Channel)
	if err != nil {
		fail(err)
		return
	}
	key, err := p.d.Pool.Acquire(ctx, resolved.Channel.ID)
	if err != nil {
		fail(err)
		return
	}
	if key != nil {
		rec.UpstreamKeyID = key.ID
	}

	cctx := cache.Context{Model: req.Model, TotalContextTokens: estimateTokens(&req)}
	streaming := req.Stream || strings.Contains(r.Header.Get("Accept"), "event-stream")
	rec.IsStreaming = streaming

	// 5. 调上游 + 计费改写 + 回写
	if streaming {
		respBody, err = p.serveStream(ctx, w, adapter, &req, key, resolved, cctx, pipe, rec)
	} else {
		respBody, err = p.serveUnary(ctx, w, adapter, &req, key, resolved, cctx, pipe, rec)
	}

	result := upstream.Result{Success: err == nil, StatusCode: rec.StatusCode}
	if err != nil {
		result.Err = err
		// 流式可能已写出部分内容，仅在尚未写 header 时回写错误
		if !streaming {
			fail(err)
		} else {
			rec.IsSuccess = false
			de, _ := domain.AsError(err)
			if de != nil {
				rec.StatusCode, rec.ErrorType, rec.ErrorMessage = de.HTTPStatus, de.Code, de.UserMessage
			}
		}
	}
	p.d.Pool.Release(key, result)
}

// serveUnary 处理非流式：调用上游 → 计费改写 → 响应改写 → 回写 JSON。
func (p *Proxy) serveUnary(ctx context.Context, w http.ResponseWriter, ad upstream.Adapter, req *domain.MessagesRequest, key *domain.UpstreamKey, rg *auth.ResolvedGroup, cctx cache.Context, pipe *transformer.Pipeline, rec *observ.RequestRecord) ([]byte, error) {
	resp, err := p.callUnary(ctx, ad, req, key, rg.Group.Retry)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	rec.FirstTokenAt = &now
	rec.TTFTMs = uint32(time.Since(rec.RequestAt).Milliseconds())

	upUsage := resp.Usage.ToUsage()
	// 计费 total 优先用上游真实输入侧 token（精确计费），上游未返回时回退字节估算
	cctx.TotalContextTokens = contextTokens(upUsage, req)
	billed := rg.CacheStrategy.Compute(upUsage, cctx)
	resp.Usage = toRawUsage(billed)
	rec.Upstream, rec.Billed = upUsage, billed

	respT, err := pipe.ApplyResponse(ctx, resp)
	if err != nil {
		return nil, domain.ErrInternal.Wrap(err)
	}
	out, _ := json.Marshal(respT)
	rec.IsSuccess = true
	rec.StatusCode = http.StatusOK
	writeJSON(w, http.StatusOK, json.RawMessage(out))
	return out, nil
}

// serveStream 处理流式：解析上游 SSE → 累积 usage → 在 message_delta 改写为计费值 → 即时 flush。
func (p *Proxy) serveStream(ctx context.Context, w http.ResponseWriter, ad upstream.Adapter, req *domain.MessagesRequest, key *domain.UpstreamKey, rg *auth.ResolvedGroup, cctx cache.Context, pipe *transformer.Pipeline, rec *observ.RequestRecord) ([]byte, error) {
	events, err := p.callStream(ctx, ad, req, key, rg.Group.Retry)
	if err != nil {
		return nil, err
	}
	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	rec.StatusCode = http.StatusOK

	var buf bytes.Buffer
	var upUsage domain.Usage
	firstToken := false

	for ev := range events {
		evT, err := pipe.ApplyStreamEvent(ctx, &ev)
		if err != nil || evT == nil {
			continue // 改写失败或被丢弃的事件跳过
		}
		ev = *evT

		// 累积上游 usage（input 来自 message_start，output/cache 来自 message_delta）
		mergeEventUsage(ev.Data, &upUsage)

		if !firstToken && ev.Event == "content_block_delta" {
			now := time.Now()
			rec.FirstTokenAt = &now
			rec.TTFTMs = uint32(time.Since(rec.RequestAt).Milliseconds())
			firstToken = true
		}

		// message_delta 携带最终 usage：按上游真实输入侧 token 计费并改写后下发
		if ev.Event == "message_delta" {
			cctx.TotalContextTokens = contextTokens(upUsage, req)
			billed := rg.CacheStrategy.Compute(upUsage, cctx)
			rec.Upstream, rec.Billed = upUsage, billed
			ev.Data = rewriteEventUsage(ev.Data, billed)
		}

		line := encodeSSE(ev)
		buf.WriteString(line)
		if _, err := io.WriteString(w, line); err != nil {
			return buf.Bytes(), nil // 客户端断开，停止但记录已写部分
		}
		if flusher != nil {
			flusher.Flush()
		}
	}
	if rec.Billed == (domain.Usage{}) && upUsage != (domain.Usage{}) {
		cctx.TotalContextTokens = contextTokens(upUsage, req)
		rec.Upstream = upUsage
		rec.Billed = rg.CacheStrategy.Compute(upUsage, cctx)
	}
	rec.IsSuccess = true
	return buf.Bytes(), nil
}

// callUnary 调用上游非流式接口，对可重试错误按分组 retry 配置重试。
func (p *Proxy) callUnary(ctx context.Context, ad upstream.Adapter, req *domain.MessagesRequest, key *domain.UpstreamKey, retry domain.RetryConfig) (*domain.MessagesResponse, error) {
	var lastErr error
	for i := 0; i <= maxRetries(retry); i++ {
		if i > 0 && !backoff(ctx, retry.BackoffMs) {
			return nil, domain.ErrTimeout
		}
		resp, err := ad.Send(ctx, req, key)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !retriable(err) {
			break
		}
	}
	return nil, lastErr
}

// callStream 建立上游流式连接，对建连阶段的可重试错误重试（尚未写出 header）。
func (p *Proxy) callStream(ctx context.Context, ad upstream.Adapter, req *domain.MessagesRequest, key *domain.UpstreamKey, retry domain.RetryConfig) (<-chan domain.StreamEvent, error) {
	var lastErr error
	for i := 0; i <= maxRetries(retry); i++ {
		if i > 0 && !backoff(ctx, retry.BackoffMs) {
			return nil, domain.ErrTimeout
		}
		ch, err := ad.SendStream(ctx, req, key)
		if err == nil {
			return ch, nil
		}
		lastErr = err
		if !retriable(err) {
			break
		}
	}
	return nil, lastErr
}

func maxRetries(r domain.RetryConfig) int {
	if r.MaxRetries < 0 {
		return 0
	}
	return r.MaxRetries
}

// backoff 等待 ms 毫秒，ctx 取消则返回 false。
func backoff(ctx context.Context, ms int) bool {
	if ms <= 0 {
		return true
	}
	select {
	case <-time.After(time.Duration(ms) * time.Millisecond):
		return true
	case <-ctx.Done():
		return false
	}
}

// retriable 仅对上游失败/超时类错误重试（认证、请求格式等不重试）。
func retriable(err error) bool {
	de, ok := domain.AsError(err)
	if !ok {
		return false
	}
	return de.Code == domain.ErrUpstreamFailure.Code || de.Code == domain.ErrTimeout.Code
}

// contextTokens 计算计费 total：优先上游真实输入侧 token，回退字节估算。
func contextTokens(up domain.Usage, req *domain.MessagesRequest) int {
	if t := up.InputTokens + up.CacheReadTokens + up.CacheCreationTokens; t > 0 {
		return t
	}
	return estimateTokens(req)
}

// ReplayResult 是一次请求复现的结果（任务书 §5.6）。
type ReplayResult struct {
	TraceID       string             `json:"trace_id"`
	TargetGroupID int64              `json:"target_group_id"`
	ChannelType   domain.ChannelType `json:"channel_type"`
	DryRun        bool               `json:"dry_run"`
	Request       json.RawMessage    `json:"request"`
	Response      json.RawMessage    `json:"response,omitempty"`
	BilledUsage   domain.Usage       `json:"billed_usage"`
	UpstreamUsage domain.Usage       `json:"upstream_usage"`
	Error         string             `json:"error,omitempty"`
}

// Replay 把一段原始请求体重放到指定分组（可指向不同通道做对比复现）。
// dryRun=true 时只解析与改写、不真正发送（任务书 §5.6）。
func (p *Proxy) Replay(ctx context.Context, groupID int64, body []byte, overrideModel string, dryRun bool) (*ReplayResult, error) {
	group, err := p.d.ConfigStore.GetGroup(ctx, groupID)
	if err != nil {
		return nil, domain.ErrInvalidRequest.WithMessage("目标分组不存在")
	}
	channel, err := p.d.ConfigStore.GetChannel(ctx, group.ChannelID)
	if err != nil {
		return nil, domain.ErrInvalidRequest.WithMessage("分组通道不存在")
	}
	strategy, err := cache.New(group.CacheStrategy)
	if err != nil {
		return nil, domain.ErrInternal.Wrap(err)
	}
	var req domain.MessagesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, domain.ErrInvalidRequest.Wrap(err)
	}
	if overrideModel != "" {
		req.Model = overrideModel
	}
	pipe := factory.Build(group.TransformerConfig, p.modelMapping(ctx, channel.ID))
	reqT, err := pipe.ApplyRequest(ctx, &req)
	if err != nil {
		return nil, domain.ErrInvalidRequest.Wrap(err)
	}
	req = *reqT

	newTrace := observ.NewTraceID()
	reqJSON, _ := json.Marshal(req)
	res := &ReplayResult{TraceID: newTrace, TargetGroupID: groupID, ChannelType: channel.Type, DryRun: dryRun, Request: reqJSON}
	if dryRun {
		return res, nil
	}

	adapter, err := p.d.Registry.Build(channel)
	if err != nil {
		return nil, err
	}
	key, err := p.d.Pool.Acquire(ctx, channel.ID)
	if err != nil {
		return nil, err
	}
	req.Stream = false
	resp, err := adapter.Send(ctx, &req, key)
	if err != nil {
		res.Error = err.Error()
		p.d.Pool.Release(key, upstream.Result{Success: false, Err: err})
		return res, nil
	}
	p.d.Pool.Release(key, upstream.Result{Success: true, StatusCode: http.StatusOK})

	upUsage := resp.Usage.ToUsage()
	billed := strategy.Compute(upUsage, cache.Context{Model: req.Model, TotalContextTokens: estimateTokens(&req)})
	resp.Usage = toRawUsage(billed)
	respJSON, _ := json.Marshal(resp)
	res.Response, res.UpstreamUsage, res.BilledUsage = respJSON, upUsage, billed

	if p.d.Sink != nil {
		p.d.Sink.Write(context.WithoutCancel(ctx), observ.RequestRecord{
			TraceID: newTrace, GroupID: groupID, ChannelID: channel.ID, ChannelType: channel.Type,
			Model: req.Model, RequestAt: time.Now(), CompletedAt: time.Now(),
			StatusCode: http.StatusOK, IsSuccess: true, Upstream: upUsage, Billed: billed,
		})
	}
	return res, nil
}

// ServeModels 返回模型列表（来源于 model_mappings 与内置已知模型）。
func (p *Proxy) ServeModels(w http.ResponseWriter, r *http.Request) {
	seen := map[string]bool{}
	var data []map[string]any
	add := func(id string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		data = append(data, map[string]any{"id": id, "type": "model", "display_name": id})
	}
	for _, m := range []string{
		"claude-sonnet-4-20250514", "claude-3-7-sonnet-20250219",
		"claude-3-5-sonnet-20241022", "claude-3-5-haiku-20241022", "claude-3-opus-20240229",
	} {
		add(m)
	}
	if p.d.ConfigStore != nil {
		if chs, err := p.d.ConfigStore.ListChannels(r.Context()); err == nil {
			for _, ch := range chs {
				if ms, err := p.d.ConfigStore.ListModelMappings(r.Context(), ch.ID); err == nil {
					for _, m := range ms {
						add(m.ClientModel)
					}
				}
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
}

// finish 收尾：按采样落 body，写明细。使用脱离取消的 context，避免客户端断开丢记录。
// submitFinish 把收尾任务投递到异步 worker pool（不阻塞请求 goroutine）。
// 队列满时降级：错误请求同步落库（不丢），成功采样直接丢弃并计数（任务书 §2.1）。
func (p *Proxy) submitFinish(rec *observ.RequestRecord, reqBody, respBody []byte) {
	job := finishJob{rec: rec, reqBody: reqBody, respBody: respBody}
	select {
	case p.finishQueue <- job:
	default:
		if !rec.IsSuccess {
			p.doFinish(rec, reqBody, respBody)
		} else {
			p.dropped.Add(1)
		}
	}
}

// doFinish 在 worker 中执行：按采样落 body（带重试），再写明细。
func (p *Proxy) doFinish(rec *observ.RequestRecord, reqBody, respBody []byte) {
	if rec.CompletedAt.IsZero() {
		rec.CompletedAt = time.Now()
	}
	rec.DurationMs = uint32(rec.CompletedAt.Sub(rec.RequestAt).Milliseconds())
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	shouldStore := !rec.IsSuccess || rand.Float64() < p.d.SampleSuccess
	if shouldStore && p.d.Bodies != nil {
		if k, ok := p.putBody(ctx, rec.TraceID, "request", reqBody); ok {
			rec.RequestBodyS3Key = k
		}
		if len(respBody) > 0 {
			if k, ok := p.putBody(ctx, rec.TraceID, "response", respBody); ok {
				rec.ResponseBodyS3Key = k
			}
		}
	}
	if p.d.Sink != nil {
		p.d.Sink.Write(ctx, *rec)
	}
}

// putBody 写 body 并按 S3WriteRetry 重试，全部失败则丢弃（任务书 §5.6）。
func (p *Proxy) putBody(ctx context.Context, traceID, kind string, body []byte) (string, bool) {
	retries := p.d.S3WriteRetry
	if retries < 0 {
		retries = 0
	}
	for i := 0; i <= retries; i++ {
		if k, err := p.d.Bodies.Put(ctx, traceID, kind, body); err == nil {
			return k, true
		}
	}
	return "", false
}

func (p *Proxy) modelMapping(ctx context.Context, channelID int64) map[string]string {
	out := map[string]string{}
	if p.d.ConfigStore == nil {
		return out
	}
	ms, err := p.d.ConfigStore.ListModelMappings(ctx, channelID)
	if err != nil {
		return out
	}
	for _, m := range ms {
		out[m.ClientModel] = m.UpstreamModel
	}
	return out
}

// ---- 辅助函数 ----

func bearerToken(h string) string {
	if h == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return strings.TrimSpace(h)
}

func toRawUsage(u domain.Usage) *domain.RawUsage {
	return &domain.RawUsage{
		InputTokens:              u.InputTokens,
		OutputTokens:             u.OutputTokens,
		CacheCreationInputTokens: u.CacheCreationTokens,
		CacheReadInputTokens:     u.CacheReadTokens,
	}
}

// estimateTokens 粗略估算入参 tokens（system + messages），约 4 字节/token。
func estimateTokens(req *domain.MessagesRequest) int {
	n := len(req.System)
	for _, m := range req.Messages {
		n += len(m.Content)
	}
	if n == 0 {
		return 0
	}
	return n / 4
}

func encodeSSE(ev domain.StreamEvent) string {
	var b strings.Builder
	if ev.Event != "" {
		b.WriteString("event: ")
		b.WriteString(ev.Event)
		b.WriteString("\n")
	}
	b.WriteString("data: ")
	b.Write(ev.Data)
	b.WriteString("\n\n")
	return b.String()
}

func mergeEventUsage(data json.RawMessage, u *domain.Usage) {
	if len(data) == 0 {
		return
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return
	}
	applyRawUsage(m["usage"], u)
	if msg, ok := m["message"]; ok {
		var mm map[string]json.RawMessage
		if json.Unmarshal(msg, &mm) == nil {
			applyRawUsage(mm["usage"], u)
		}
	}
}

func applyRawUsage(raw json.RawMessage, u *domain.Usage) {
	if len(raw) == 0 {
		return
	}
	var ru domain.RawUsage
	if json.Unmarshal(raw, &ru) != nil {
		return
	}
	if ru.InputTokens > 0 {
		u.InputTokens = ru.InputTokens
	}
	if ru.OutputTokens > 0 {
		u.OutputTokens = ru.OutputTokens
	}
	if ru.CacheCreationInputTokens > 0 {
		u.CacheCreationTokens = ru.CacheCreationInputTokens
	}
	if ru.CacheReadInputTokens > 0 {
		u.CacheReadTokens = ru.CacheReadInputTokens
	}
}

func rewriteEventUsage(data json.RawMessage, billed domain.Usage) json.RawMessage {
	var m map[string]json.RawMessage
	if json.Unmarshal(data, &m) != nil {
		return data
	}
	b, _ := json.Marshal(toRawUsage(billed))
	m["usage"] = b
	out, err := json.Marshal(m)
	if err != nil {
		return data
	}
	return out
}
