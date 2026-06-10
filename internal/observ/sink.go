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
	TraceID      string
	APIKeyID     int64
	GroupID      int64
	ChannelID    int64
	ChannelType  domain.ChannelType
	UpstreamKeyID int64
	Model        string

	RequestAt    time.Time
	FirstTokenAt *time.Time
	CompletedAt  time.Time
	TTFTMs       uint32
	DurationMs   uint32

	StatusCode   int
	IsStreaming  bool
	IsSuccess    bool
	ErrorType    string
	ErrorMessage string

	Billed   domain.Usage // 返回给客户的计费值
	Upstream domain.Usage // 上游真实值

	RequestBodyS3Key  string
	ResponseBodyS3Key string
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
