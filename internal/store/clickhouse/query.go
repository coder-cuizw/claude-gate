package clickhouse

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/observ"
)

// 本文件实现 observ.MetricsReader：全部聚合直接查 request_logs（统计量小，
// 生产大范围可改走物化视图 request_metrics_1m）。

func whereStats(q observ.StatsQuery) (string, []any) {
	var conds []string
	var args []any
	if !q.From.IsZero() {
		conds = append(conds, "request_at >= ?")
		args = append(args, q.From)
	}
	if !q.To.IsZero() {
		conds = append(conds, "request_at <= ?")
		args = append(args, q.To)
	}
	if q.GroupID != 0 {
		conds = append(conds, "group_id = ?")
		args = append(args, uint64(q.GroupID))
	}
	if q.ChannelID != 0 {
		conds = append(conds, "channel_id = ?")
		args = append(args, uint64(q.ChannelID))
	}
	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// Overview 概览指标。
func (s *Store) Overview(ctx context.Context, q observ.StatsQuery) (observ.Overview, error) {
	where, args := whereStats(q)
	var (
		cnt, success, errCnt, totalTok uint64
		avg, p95, p99                  float64
		maxTTFT                        uint32
	)
	err := s.conn.QueryRow(ctx, `SELECT
		count(),
		countIf(is_success=1),
		countIf(is_success=0),
		sum(input_tokens+output_tokens+cache_creation_tokens+cache_read_tokens),
		avg(ttft_ms), quantile(0.95)(ttft_ms), quantile(0.99)(ttft_ms), max(ttft_ms)
		FROM request_logs`+where, args...).Scan(&cnt, &success, &errCnt, &totalTok, &avg, &p95, &p99, &maxTTFT)
	if err != nil {
		return observ.Overview{}, err
	}
	ov := observ.Overview{RequestCount: int64(cnt), ErrorCount: int64(errCnt), TotalTokens: int64(totalTok), MaxTTFTMs: maxTTFT}
	if cnt > 0 {
		ov.SuccessRate = float64(success) / float64(cnt)
		ov.AvgTTFTMs = avg
		ov.P95TTFTMs = p95
		ov.P99TTFTMs = p99
	}
	return ov, nil
}

// Errors 错误类型分布。
func (s *Store) Errors(ctx context.Context, q observ.StatsQuery) ([]observ.ErrorBucket, error) {
	where, args := whereStats(q)
	if where == "" {
		where = " WHERE is_success=0 AND error_type!=''"
	} else {
		where += " AND is_success=0 AND error_type!=''"
	}
	rows, err := s.conn.Query(ctx, `SELECT error_type, count() FROM request_logs`+where+` GROUP BY error_type ORDER BY count() DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []observ.ErrorBucket
	for rows.Next() {
		var b observ.ErrorBucket
		var c uint64
		if err := rows.Scan(&b.ErrorType, &c); err != nil {
			return nil, err
		}
		b.Count = int64(c)
		out = append(out, b)
	}
	return out, rows.Err()
}

// ByChannel 按通道统计。
func (s *Store) ByChannel(ctx context.Context, q observ.StatsQuery) ([]observ.ChannelStat, error) {
	where, args := whereStats(q)
	rows, err := s.conn.Query(ctx, `SELECT channel_type, count(), countIf(is_success=1), avg(ttft_ms)
		FROM request_logs`+where+` GROUP BY channel_type ORDER BY count() DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []observ.ChannelStat
	for rows.Next() {
		var ct string
		var cnt, succ uint64
		var avg float64
		if err := rows.Scan(&ct, &cnt, &succ, &avg); err != nil {
			return nil, err
		}
		st := observ.ChannelStat{ChannelType: domain.ChannelType(ct), RequestCount: int64(cnt), AvgTTFTMs: avg}
		if cnt > 0 {
			st.SuccessRate = float64(succ) / float64(cnt)
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// Timeseries 按 granularity 分桶。
func (s *Store) Timeseries(ctx context.Context, q observ.StatsQuery, metric, granularity string) ([]observ.TimePoint, error) {
	if q.From.IsZero() {
		q.From = time.Now().Add(-time.Hour)
	}
	if q.To.IsZero() {
		q.To = time.Now()
	}
	interval, secs := intervalOf(granularity)
	where, args := whereStats(q)
	bucket := fmt.Sprintf("toStartOfInterval(request_at, INTERVAL %s) AS b", interval)

	var sel string
	switch metric {
	case "qps":
		sel = "count()"
	case "error_rate":
		sel = "countIf(is_success=0), count()"
	case "tokens":
		sel = "sum(input_tokens), sum(output_tokens), sum(cache_read_tokens)"
	default: // ttft_p95
		sel = "quantile(0.5)(ttft_ms), quantile(0.95)(ttft_ms), quantile(0.99)(ttft_ms)"
	}
	rows, err := s.conn.Query(ctx, `SELECT `+bucket+`, `+sel+` FROM request_logs`+where+` GROUP BY b ORDER BY b`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []observ.TimePoint
	for rows.Next() {
		var b time.Time
		label := func() string { return b.Format("01-02 15:04") }
		switch metric {
		case "qps":
			var c uint64
			if err := rows.Scan(&b, &c); err != nil {
				return nil, err
			}
			out = append(out, observ.TimePoint{Timestamp: label(), Value: round1(float64(c) / secs)})
		case "error_rate":
			var e, c uint64
			if err := rows.Scan(&b, &e, &c); err != nil {
				return nil, err
			}
			rate := 0.0
			if c > 0 {
				rate = float64(e) / float64(c) * 100
			}
			out = append(out, observ.TimePoint{Timestamp: label(), Value: round1(rate)})
		case "tokens":
			var in, o, cr uint64
			if err := rows.Scan(&b, &in, &o, &cr); err != nil {
				return nil, err
			}
			lb := label()
			out = append(out, observ.TimePoint{Timestamp: lb, Value: float64(in), Series: "输入"},
				observ.TimePoint{Timestamp: lb, Value: float64(o), Series: "输出"},
				observ.TimePoint{Timestamp: lb, Value: float64(cr), Series: "缓存读取"})
		default:
			var p50, p95, p99 float64
			if err := rows.Scan(&b, &p50, &p95, &p99); err != nil {
				return nil, err
			}
			lb := label()
			out = append(out, observ.TimePoint{Timestamp: lb, Value: round1(p50), Series: "P50"},
				observ.TimePoint{Timestamp: lb, Value: round1(p95), Series: "P95"},
				observ.TimePoint{Timestamp: lb, Value: round1(p99), Series: "P99"})
		}
	}
	return out, rows.Err()
}

const recordCols = `trace_id, api_key_id, group_id, channel_id, channel_type, upstream_key_id, model,
	request_at, first_token_at, completed_at, ttft_ms, duration_ms,
	status_code, is_streaming, is_success, error_type, error_message,
	input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
	upstream_input_tokens, upstream_output_tokens, upstream_cache_creation_tokens, upstream_cache_read_tokens,
	request_body_s3_key, response_body_s3_key`

// ListTraces 过滤分页明细（新→旧）。
func (s *Store) ListTraces(ctx context.Context, q observ.TraceQuery) (observ.TraceList, error) {
	var conds []string
	var args []any
	switch q.Status {
	case "success":
		conds = append(conds, "is_success=1")
	case "error":
		conds = append(conds, "is_success=0")
	}
	if q.ChannelType != "" && q.ChannelType != "all" {
		conds = append(conds, "channel_type = ?")
		args = append(args, q.ChannelType)
	}
	if q.GroupID != 0 {
		conds = append(conds, "group_id = ?")
		args = append(args, uint64(q.GroupID))
	}
	if !q.From.IsZero() {
		conds = append(conds, "request_at >= ?")
		args = append(args, q.From)
	}
	if !q.To.IsZero() {
		conds = append(conds, "request_at <= ?")
		args = append(args, q.To)
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	page, size := q.Page, q.PageSize
	if page < 1 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}

	var total uint64
	if err := s.conn.QueryRow(ctx, `SELECT count() FROM request_logs`+where, args...).Scan(&total); err != nil {
		return observ.TraceList{}, err
	}
	pageArgs := append(append([]any{}, args...), size, (page-1)*size)
	rows, err := s.conn.Query(ctx, `SELECT `+recordCols+` FROM request_logs`+where+` ORDER BY request_at DESC LIMIT ? OFFSET ?`, pageArgs...)
	if err != nil {
		return observ.TraceList{}, err
	}
	defer rows.Close()
	items := []observ.RequestRecord{}
	for rows.Next() {
		rec, err := scanRecord(rows)
		if err != nil {
			return observ.TraceList{}, err
		}
		items = append(items, *rec)
	}
	return observ.TraceList{Items: items, Total: int64(total), Page: page, PageSize: size}, rows.Err()
}

// GetTrace 按 trace_id 查单条。
func (s *Store) GetTrace(ctx context.Context, traceID string) (*observ.RequestRecord, error) {
	row := s.conn.QueryRow(ctx, `SELECT `+recordCols+` FROM request_logs WHERE trace_id = ? LIMIT 1`, traceID)
	return scanRecord(row)
}

type scanner interface{ Scan(...any) error }

func scanRecord(r scanner) (*observ.RequestRecord, error) {
	var rec observ.RequestRecord
	var apiKeyID, groupID, channelID, upKeyID uint64
	var ct string
	var streaming, success uint8
	var statusCode uint16
	var in, out, cc, cr, uin, uout, ucc, ucr uint32
	if err := r.Scan(
		&rec.TraceID, &apiKeyID, &groupID, &channelID, &ct, &upKeyID, &rec.Model,
		&rec.RequestAt, &rec.FirstTokenAt, &rec.CompletedAt, &rec.TTFTMs, &rec.DurationMs,
		&statusCode, &streaming, &success, &rec.ErrorType, &rec.ErrorMessage,
		&in, &out, &cc, &cr, &uin, &uout, &ucc, &ucr,
		&rec.RequestBodyS3Key, &rec.ResponseBodyS3Key,
	); err != nil {
		return nil, err
	}
	rec.APIKeyID, rec.GroupID, rec.ChannelID, rec.UpstreamKeyID = int64(apiKeyID), int64(groupID), int64(channelID), int64(upKeyID)
	rec.ChannelType = domain.ChannelType(ct)
	rec.StatusCode = int(statusCode)
	rec.IsStreaming, rec.IsSuccess = streaming == 1, success == 1
	rec.Billed = domain.Usage{InputTokens: int(in), OutputTokens: int(out), CacheCreationTokens: int(cc), CacheReadTokens: int(cr)}
	rec.Upstream = domain.Usage{InputTokens: int(uin), OutputTokens: int(uout), CacheCreationTokens: int(ucc), CacheReadTokens: int(ucr)}
	return &rec, nil
}

func intervalOf(g string) (string, float64) {
	switch g {
	case "1h":
		return "1 hour", 3600
	case "5m":
		return "5 minute", 300
	default:
		return "1 minute", 60
	}
}

func round1(f float64) float64 { return float64(int(f*10)) / 10 }
