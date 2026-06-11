// Package httpproxy 提供通用的 Anthropic 兼容 HTTP 透传适配器。
//
// official / kiro（当前透传）/ relay 等"协议基本兼容 Anthropic"的通道都复用它，
// 只在认证头与默认 base_url 上有差异。通道私有协议的深度转换（如 Kiro 后续按
// 真实报错适配）再在各自包内覆盖。
package httpproxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/claude-gate/claude-gate/internal/domain"
)

// Options 配置透传适配器。
type Options struct {
	ChannelType      domain.ChannelType // 用于 Name() 与日志
	BaseURL          string             // 上游地址（不含 /v1/messages）
	AuthHeader       string             // 凭证写入的请求头，如 x-api-key / Authorization
	AuthScheme       string             // 凭证前缀，如 "" 或 "Bearer"
	AnthropicVersion string             // anthropic-version 头，可空
	ExtraHeaders     map[string]string  // 额外固定头
}

// Adapter 是通用透传适配器，实现 upstream.Adapter。
type Adapter struct {
	opts   Options
	client *http.Client
}

// New 按选项构造透传适配器，复用 keep-alive 连接池（任务书 §2.1）。
func New(opts Options) *Adapter {
	opts.BaseURL = strings.TrimRight(opts.BaseURL, "/")
	if opts.AuthHeader == "" {
		opts.AuthHeader = "Authorization"
		opts.AuthScheme = "Bearer"
	}
	return &Adapter{
		opts: opts,
		client: &http.Client{
			Timeout: 0, // 由调用方 context 控制
			Transport: &http.Transport{
				MaxIdleConns:        256,
				MaxIdleConnsPerHost: 256,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// Name 返回通道类型名。
func (a *Adapter) Name() string { return string(a.opts.ChannelType) }

func (a *Adapter) buildRequest(ctx context.Context, req *domain.MessagesRequest, key *domain.UpstreamKey, stream bool) (*http.Request, error) {
	payload := *req
	payload.Stream = stream
	body, err := json.Marshal(&payload)
	if err != nil {
		return nil, domain.ErrInvalidRequest.Wrap(err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.opts.BaseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if a.opts.AnthropicVersion != "" {
		httpReq.Header.Set("anthropic-version", a.opts.AnthropicVersion)
	}
	for k, v := range a.opts.ExtraHeaders {
		httpReq.Header.Set(k, v)
	}
	// 优先用运行时解密的明文凭证；未解密时回退原值（兼容未加密的种子数据）
	if key != nil {
		cred := key.Credential
		if cred == "" {
			cred = key.CredentialEncrypted
		}
		if cred != "" {
			val := cred
			if a.opts.AuthScheme != "" {
				val = a.opts.AuthScheme + " " + val
			}
			httpReq.Header.Set(a.opts.AuthHeader, val)
		}
	}
	if stream {
		httpReq.Header.Set("Accept", "text/event-stream")
	}
	return httpReq, nil
}

// Send 发送非流式请求。
func (a *Adapter) Send(ctx context.Context, req *domain.MessagesRequest, key *domain.UpstreamKey) (*domain.MessagesResponse, error) {
	httpReq, err := a.buildRequest(ctx, req, key, false)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, domain.ErrUpstreamFailure.Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, domain.ErrUpstreamFailure.WithMessage(fmt.Sprintf("上游返回 %d", resp.StatusCode))
	}
	var out domain.MessagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, domain.ErrUpstreamFailure.Wrap(err)
	}
	return &out, nil
}

// SendStream 发送流式请求，把标准 SSE 解析为 StreamEvent 流。
// 返回的 channel 在流结束或 ctx 取消时关闭，避免 goroutine 泄漏。
func (a *Adapter) SendStream(ctx context.Context, req *domain.MessagesRequest, key *domain.UpstreamKey) (<-chan domain.StreamEvent, error) {
	httpReq, err := a.buildRequest(ctx, req, key, true)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, domain.ErrUpstreamFailure.Wrap(err)
	}
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, domain.ErrUpstreamFailure.WithMessage(fmt.Sprintf("上游返回 %d", resp.StatusCode))
	}

	out := make(chan domain.StreamEvent, 16)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		var ev domain.StreamEvent
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case strings.HasPrefix(line, "event:"):
				ev.Event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "data:"):
				ev.Data = json.RawMessage(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			case line == "":
				if ev.Event != "" || len(ev.Data) > 0 {
					select {
					case out <- ev:
					case <-ctx.Done():
						return
					}
					ev = domain.StreamEvent{}
				}
			}
		}
	}()
	return out, nil
}
