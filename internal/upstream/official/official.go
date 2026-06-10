// Package official 实现 OfficialAdapter：直连 Anthropic 官方 API。
//
// 这是"标准通道"参照实现：原生 Messages 协议、标准 SSE，转换复杂度低，
// 用来先打通主链路（任务书 §5.5 路线图 M1）。
package official

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

const defaultBaseURL = "https://api.anthropic.com"

// Adapter 官方通道适配器。
type Adapter struct {
	baseURL          string
	anthropicVersion string
	client           *http.Client
}

// New 按通道配置构造官方适配器。
func New(ch *domain.UpstreamChannel) *Adapter {
	base := defaultBaseURL
	if ch != nil && ch.BaseURL != "" {
		base = strings.TrimRight(ch.BaseURL, "/")
	}
	version := "2023-06-01"
	if ch != nil {
		if v, ok := ch.Config["anthropic_version"].(string); ok && v != "" {
			version = v
		}
	}
	return &Adapter{
		baseURL:          base,
		anthropicVersion: version,
		// 连接池复用 keep-alive（任务书 §2.1 连接池复用）。
		client: &http.Client{
			Timeout: 0, // 由调用方 context 控制超时
			Transport: &http.Transport{
				MaxIdleConns:        256,
				MaxIdleConnsPerHost: 256,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// Name 返回通道类型名。
func (a *Adapter) Name() string { return string(domain.ChannelOfficial) }

// buildHTTPRequest 构造发往官方的 HTTP 请求。
func (a *Adapter) buildHTTPRequest(ctx context.Context, req *domain.MessagesRequest, key *domain.UpstreamKey, stream bool) (*http.Request, error) {
	payload := *req
	payload.Stream = stream
	body, err := json.Marshal(&payload)
	if err != nil {
		return nil, domain.ErrInvalidRequest.Wrap(err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", a.anthropicVersion)
	if key != nil {
		// 注：生产中此处应解密 CredentialEncrypted 得到明文 API Key。
		httpReq.Header.Set("x-api-key", key.CredentialEncrypted)
	}
	if stream {
		httpReq.Header.Set("Accept", "text/event-stream")
	}
	return httpReq, nil
}

// Send 发送非流式请求。
func (a *Adapter) Send(ctx context.Context, req *domain.MessagesRequest, key *domain.UpstreamKey) (*domain.MessagesResponse, error) {
	httpReq, err := a.buildHTTPRequest(ctx, req, key, false)
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
//
// 返回的 channel 在流结束或 ctx 取消时关闭；解析在独立 goroutine 进行，
// 通过 ctx.Done 退出，避免 goroutine 泄漏（任务书 §10 goroutine 管理）。
func (a *Adapter) SendStream(ctx context.Context, req *domain.MessagesRequest, key *domain.UpstreamKey) (<-chan domain.StreamEvent, error) {
	httpReq, err := a.buildHTTPRequest(ctx, req, key, true)
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
				// 空行表示一个事件结束
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
