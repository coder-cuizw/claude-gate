// Package observ 提供可观测性基础设施：trace_id 贯穿、明细落库、body 落盘（任务书 §5.6 / §5.7）。
package observ

import (
	"context"
	"crypto/rand"

	"github.com/oklog/ulid/v2"
)

// ctxKey 是 context 中 trace_id 的私有键类型。
type ctxKey struct{}

// NewTraceID 生成一个 ULID 作为 trace_id（任务书 §5.1：请求第一时间生成）。
func NewTraceID() string {
	return ulid.MustNew(ulid.Now(), rand.Reader).String()
}

// WithTraceID 把 trace_id 写入 context，贯穿全链路。
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, ctxKey{}, traceID)
}

// TraceID 从 context 取出 trace_id，不存在时返回空串。
func TraceID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKey{}).(string); ok {
		return v
	}
	return ""
}
