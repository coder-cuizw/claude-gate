package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/claude-gate/claude-gate/internal/auth"
	"github.com/claude-gate/claude-gate/internal/cache"
	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/observ"
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
}

// Proxy 是端到端代理引擎，实现完整主链路（任务书 §3 / §5.1）：
//
//	认证 → 加载分组 → 改写 → 选 Adapter + 取 Key → 调上游 →
//	缓存计费改写 usage → 流式/非流式回写 → 明细与 body 落库
type Proxy struct {
	d ProxyDeps
}

var _ ProxyHandler = (*Proxy)(nil)

// NewProxy 构造代理引擎。
func NewProxy(d ProxyDeps) *Proxy {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	if d.Timeout <= 0 {
		d.Timeout = 600 * time.Second
	}
	return &Proxy{d: d}
}

// ServeMessages 处理 POST /v1/messages（流式与非流式合一）。
func (p *Proxy) ServeMessages(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	traceID := observ.TraceID(r.Context())
	ctx, cancel := context.WithTimeout(r.Context(), p.d.Timeout)
	defer cancel()

	rec := &observ.RequestRecord{TraceID: traceID, RequestAt: start}
	var reqBody, respBody []byte
	defer func() { p.finish(ctx, rec, reqBody, respBody) }()

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
	resp, err := ad.Send(ctx, req, key)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	rec.FirstTokenAt = &now
	rec.TTFTMs = uint32(time.Since(rec.RequestAt).Milliseconds())

	upUsage := resp.Usage.ToUsage()
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
	events, err := ad.SendStream(ctx, req, key)
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

		// message_delta 携带最终 usage：改写为计费值后再下发
		if ev.Event == "message_delta" {
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
		rec.Upstream = upUsage
		rec.Billed = rg.CacheStrategy.Compute(upUsage, cctx)
	}
	rec.IsSuccess = true
	return buf.Bytes(), nil
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
func (p *Proxy) finish(ctx context.Context, rec *observ.RequestRecord, reqBody, respBody []byte) {
	if rec.CompletedAt.IsZero() {
		rec.CompletedAt = time.Now()
	}
	rec.DurationMs = uint32(rec.CompletedAt.Sub(rec.RequestAt).Milliseconds())
	bg := context.WithoutCancel(ctx)

	shouldStore := !rec.IsSuccess || rand.Float64() < p.d.SampleSuccess
	if shouldStore && p.d.Bodies != nil {
		if k, err := p.d.Bodies.Put(bg, rec.TraceID, "request", reqBody); err == nil {
			rec.RequestBodyS3Key = k
		}
		if len(respBody) > 0 {
			if k, err := p.d.Bodies.Put(bg, rec.TraceID, "response", respBody); err == nil {
				rec.ResponseBodyS3Key = k
			}
		}
	}
	if p.d.Sink != nil {
		p.d.Sink.Write(bg, *rec)
	}
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
