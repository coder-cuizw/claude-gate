package observ

import (
	"context"
	"time"

	"github.com/claude-gate/claude-gate/internal/domain"
)

// 本文件定义统计与请求明细的查询接口（任务书 §5.7）。
// 生产由 ClickHouse 物化视图支撑，内存实现用于离线自测。

// StatsQuery 是统计查询的过滤条件。
type StatsQuery struct {
	From      time.Time
	To        time.Time
	GroupID   int64 // 0 表示不限
	ChannelID int64 // 0 表示不限
}

// Overview 概览指标。
type Overview struct {
	RequestCount int64   `json:"request_count"`
	SuccessRate  float64 `json:"success_rate"`
	AvgTTFTMs    float64 `json:"avg_ttft_ms"`
	P95TTFTMs    float64 `json:"p95_ttft_ms"`
	P99TTFTMs    float64 `json:"p99_ttft_ms"`
	MaxTTFTMs    uint32  `json:"max_ttft_ms"`
	TotalTokens  int64   `json:"total_tokens"`
	ErrorCount   int64   `json:"error_count"`
}

// TimePoint 时序点。
type TimePoint struct {
	Timestamp string  `json:"timestamp"`
	Value     float64 `json:"value"`
	Series    string  `json:"series,omitempty"`
}

// ErrorBucket 错误分布。
type ErrorBucket struct {
	ErrorType string `json:"error_type"`
	Count     int64  `json:"count"`
}

// ChannelStat 按通道统计。
type ChannelStat struct {
	ChannelType domain.ChannelType `json:"channel_type"`
	RequestCount int64             `json:"request_count"`
	SuccessRate  float64           `json:"success_rate"`
	AvgTTFTMs    float64           `json:"avg_ttft_ms"`
}

// TraceQuery 明细列表查询条件。
type TraceQuery struct {
	Status      string // all / success / error
	ChannelType string
	GroupID     int64
	From        time.Time
	To          time.Time
	Page        int
	PageSize    int
}

// TraceList 分页明细列表。
type TraceList struct {
	Items    []RequestRecord `json:"items"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

// MetricsReader 统计与明细查询接口（任务书 §5.7）。
type MetricsReader interface {
	Overview(ctx context.Context, q StatsQuery) (Overview, error)
	Timeseries(ctx context.Context, q StatsQuery, metric, granularity string) ([]TimePoint, error)
	Errors(ctx context.Context, q StatsQuery) ([]ErrorBucket, error)
	ByChannel(ctx context.Context, q StatsQuery) ([]ChannelStat, error)
	ListTraces(ctx context.Context, q TraceQuery) (TraceList, error)
	GetTrace(ctx context.Context, traceID string) (*RequestRecord, error)
}
