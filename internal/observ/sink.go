package observ

import (
	"context"
	"time"

	"github.com/claude-gate/claude-gate/internal/domain"
)

// RequestRecord 是一次请求的明细记录，对应 ClickHouse request_logs 表。
//
// 同时保留计费 usage 与上游真实 usage（任务书 §4.2 / §5.3）。
type RequestRecord struct {
	TraceID       string             `json:"trace_id"`
	APIKeyID      int64              `json:"api_key_id"`
	GroupID       int64              `json:"group_id"`
	ChannelID     int64              `json:"channel_id"`
	ChannelType   domain.ChannelType `json:"channel_type"`
	UpstreamKeyID int64              `json:"upstream_key_id"`
	Model         string             `json:"model"`

	RequestAt    time.Time  `json:"request_at"`
	FirstTokenAt *time.Time `json:"first_token_at,omitempty"`
	CompletedAt  time.Time  `json:"completed_at"`
	TTFTMs       uint32     `json:"ttft_ms"`
	DurationMs   uint32     `json:"duration_ms"`

	StatusCode   int    `json:"status_code"`
	IsStreaming  bool   `json:"is_streaming"`
	IsSuccess    bool   `json:"is_success"`
	ErrorType    string `json:"error_type,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`

	Billed   domain.Usage `json:"billed_usage"`   // 返回给客户的计费值
	Upstream domain.Usage `json:"upstream_usage"` // 上游真实值

	RequestBodyS3Key  string `json:"request_body_s3_key,omitempty"`
	ResponseBodyS3Key string `json:"response_body_s3_key,omitempty"`
}

// Sink 是明细落库的统一出口。实现需保证异步、不反压主链路（任务书 §2.1）。
type Sink interface {
	// Write 入队一条明细记录；队列满时按"先弃成功采样、保留错误"降级。
	Write(ctx context.Context, rec RequestRecord)
	// Close 优雅关闭：flush 剩余缓冲。
	Close(ctx context.Context) error
}

// BodyStore 是请求/响应 body 落盘接口（S3/MinIO）。
type BodyStore interface {
	// Put 异步写入 body，返回 S3 key。
	Put(ctx context.Context, traceID, kind string, body []byte) (key string, err error)
	// Get 读取 body（详情/复现时使用），应设较短超时。
	Get(ctx context.Context, key string) ([]byte, error)
}
