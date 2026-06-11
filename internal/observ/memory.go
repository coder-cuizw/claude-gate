package observ

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/claude-gate/claude-gate/internal/domain"
)

// MemoryStore 是 Sink / BodyStore / MetricsReader 的内存实现，用于离线自测与演示。
//
// 它保存最近 maxRecords 条明细，并就地计算各种聚合，免依赖 ClickHouse。
type MemoryStore struct {
	mu         sync.RWMutex
	records    []RequestRecord
	bodies     map[string][]byte
	maxRecords int
}

var (
	_ Sink          = (*MemoryStore)(nil)
	_ BodyStore     = (*MemoryStore)(nil)
	_ MetricsReader = (*MemoryStore)(nil)
)

// NewMemoryStore 构造内存观测存储。
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{bodies: map[string][]byte{}, maxRecords: 5000}
}

// Write 入队一条明细（内存实现直接追加并按上限淘汰最旧）。
func (m *MemoryStore) Write(_ context.Context, rec RequestRecord) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, rec)
	if len(m.records) > m.maxRecords {
		m.records = m.records[len(m.records)-m.maxRecords:]
	}
}

// Close 无需 flush。
func (m *MemoryStore) Close(_ context.Context) error { return nil }

// Put 保存 body，返回 key。
func (m *MemoryStore) Put(_ context.Context, traceID, kind string, body []byte) (string, error) {
	key := fmt.Sprintf("requests/%s/%s/%s.json", time.Now().Format("2006-01-02"), traceID, kind)
	m.mu.Lock()
	m.bodies[key] = body
	m.mu.Unlock()
	return key, nil
}

// Get 读取 body。
func (m *MemoryStore) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.bodies[key]
	if !ok {
		return nil, fmt.Errorf("body 不存在: %s", key)
	}
	return b, nil
}

// filtered 返回符合查询条件的记录快照。
func (m *MemoryStore) filtered(q StatsQuery) []RequestRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]RequestRecord, 0, len(m.records))
	for _, r := range m.records {
		if !q.From.IsZero() && r.RequestAt.Before(q.From) {
			continue
		}
		if !q.To.IsZero() && r.RequestAt.After(q.To) {
			continue
		}
		if q.GroupID != 0 && r.GroupID != q.GroupID {
			continue
		}
		if q.ChannelID != 0 && r.ChannelID != q.ChannelID {
			continue
		}
		out = append(out, r)
	}
	return out
}

// Overview 计算概览指标。
func (m *MemoryStore) Overview(_ context.Context, q StatsQuery) (Overview, error) {
	recs := m.filtered(q)
	var ov Overview
	if len(recs) == 0 {
		return ov, nil
	}
	ttfts := make([]float64, 0, len(recs))
	var success, totalTTFT int64
	for _, r := range recs {
		ov.RequestCount++
		if r.IsSuccess {
			success++
		} else {
			ov.ErrorCount++
		}
		ov.TotalTokens += int64(r.Billed.Total())
		ttfts = append(ttfts, float64(r.TTFTMs))
		totalTTFT += int64(r.TTFTMs)
		if r.TTFTMs > ov.MaxTTFTMs {
			ov.MaxTTFTMs = r.TTFTMs
		}
	}
	ov.SuccessRate = float64(success) / float64(ov.RequestCount)
	ov.AvgTTFTMs = float64(totalTTFT) / float64(ov.RequestCount)
	sort.Float64s(ttfts)
	ov.P95TTFTMs = percentile(ttfts, 0.95)
	ov.P99TTFTMs = percentile(ttfts, 0.99)
	return ov, nil
}

// Timeseries 按 granularity 分桶计算时序。
func (m *MemoryStore) Timeseries(_ context.Context, q StatsQuery, metric, granularity string) ([]TimePoint, error) {
	recs := m.filtered(q)
	bucket := granularityDur(granularity)
	if q.From.IsZero() {
		q.From = time.Now().Add(-time.Hour)
	}
	if q.To.IsZero() {
		q.To = time.Now()
	}
	type agg struct {
		count, errs int
		ttft        []float64
		in, out, cr int64
	}
	buckets := map[int64]*agg{}
	for _, r := range recs {
		b := r.RequestAt.Truncate(bucket).Unix()
		a := buckets[b]
		if a == nil {
			a = &agg{}
			buckets[b] = a
		}
		a.count++
		if !r.IsSuccess {
			a.errs++
		}
		a.ttft = append(a.ttft, float64(r.TTFTMs))
		a.in += int64(r.Billed.InputTokens)
		a.out += int64(r.Billed.OutputTokens)
		a.cr += int64(r.Billed.CacheReadTokens)
	}
	keys := make([]int64, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	var out []TimePoint
	label := func(unix int64) string { return time.Unix(unix, 0).Format("01-02 15:04") }
	for _, k := range keys {
		a := buckets[k]
		sort.Float64s(a.ttft)
		switch metric {
		case "qps":
			out = append(out, TimePoint{Timestamp: label(k), Value: round1(float64(a.count) / bucket.Seconds())})
		case "error_rate":
			rate := 0.0
			if a.count > 0 {
				rate = float64(a.errs) / float64(a.count) * 100
			}
			out = append(out, TimePoint{Timestamp: label(k), Value: round1(rate)})
		case "tokens":
			out = append(out, TimePoint{Timestamp: label(k), Value: float64(a.in), Series: "输入"})
			out = append(out, TimePoint{Timestamp: label(k), Value: float64(a.out), Series: "输出"})
			out = append(out, TimePoint{Timestamp: label(k), Value: float64(a.cr), Series: "缓存读取"})
		default: // ttft_p95
			out = append(out, TimePoint{Timestamp: label(k), Value: round1(percentile(a.ttft, 0.5)), Series: "P50"})
			out = append(out, TimePoint{Timestamp: label(k), Value: round1(percentile(a.ttft, 0.95)), Series: "P95"})
			out = append(out, TimePoint{Timestamp: label(k), Value: round1(percentile(a.ttft, 0.99)), Series: "P99"})
		}
	}
	return out, nil
}

// Errors 计算错误类型分布。
func (m *MemoryStore) Errors(_ context.Context, q StatsQuery) ([]ErrorBucket, error) {
	recs := m.filtered(q)
	counts := map[string]int64{}
	for _, r := range recs {
		if !r.IsSuccess && r.ErrorType != "" {
			counts[r.ErrorType]++
		}
	}
	out := make([]ErrorBucket, 0, len(counts))
	for k, v := range counts {
		out = append(out, ErrorBucket{ErrorType: k, Count: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	return out, nil
}

// ByChannel 按通道类型统计。
func (m *MemoryStore) ByChannel(_ context.Context, q StatsQuery) ([]ChannelStat, error) {
	recs := m.filtered(q)
	type acc struct {
		count, success int64
		ttft           int64
	}
	by := map[domain.ChannelType]*acc{}
	for _, r := range recs {
		a := by[r.ChannelType]
		if a == nil {
			a = &acc{}
			by[r.ChannelType] = a
		}
		a.count++
		if r.IsSuccess {
			a.success++
		}
		a.ttft += int64(r.TTFTMs)
	}
	out := make([]ChannelStat, 0, len(by))
	for ct, a := range by {
		s := ChannelStat{ChannelType: ct, RequestCount: a.count}
		if a.count > 0 {
			s.SuccessRate = float64(a.success) / float64(a.count)
			s.AvgTTFTMs = float64(a.ttft) / float64(a.count)
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RequestCount > out[j].RequestCount })
	return out, nil
}

// ListTraces 过滤并分页返回明细（新→旧）。
func (m *MemoryStore) ListTraces(_ context.Context, q TraceQuery) (TraceList, error) {
	m.mu.RLock()
	all := make([]RequestRecord, len(m.records))
	copy(all, m.records)
	m.mu.RUnlock()

	// 过滤
	filtered := all[:0]
	for _, r := range all {
		if q.Status == "success" && !r.IsSuccess {
			continue
		}
		if q.Status == "error" && r.IsSuccess {
			continue
		}
		if q.ChannelType != "" && q.ChannelType != "all" && string(r.ChannelType) != q.ChannelType {
			continue
		}
		if q.GroupID != 0 && r.GroupID != q.GroupID {
			continue
		}
		filtered = append(filtered, r)
	}
	// 新→旧
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].RequestAt.After(filtered[j].RequestAt) })

	total := int64(len(filtered))
	page, size := q.Page, q.PageSize
	if page < 1 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	start := (page - 1) * size
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + size
	if end > len(filtered) {
		end = len(filtered)
	}
	return TraceList{Items: filtered[start:end], Total: total, Page: page, PageSize: size}, nil
}

// GetTrace 按 trace_id 查明细。
func (m *MemoryStore) GetTrace(_ context.Context, traceID string) (*RequestRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for i := range m.records {
		if m.records[i].TraceID == traceID {
			cp := m.records[i]
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("trace 不存在: %s", traceID)
}

// SeedDemo 注入一批合成历史明细，让大盘开箱即有数据（仅演示/自测用）。
func (m *MemoryStore) SeedDemo(channelByType map[domain.ChannelType]int64, groupID int64) {
	rng := rand.New(rand.NewSource(20260610))
	types := []domain.ChannelType{domain.ChannelCustom, domain.ChannelOfficial, domain.ChannelKiro, domain.ChannelRelay}
	errTypes := []string{"upstream_timeout", "rate_limited", "upstream_5xx", "context_length_exceeded", "invalid_request"}
	now := time.Now()
	for i := 0; i < 800; i++ {
		ct := types[rng.Intn(len(types))]
		success := rng.Float64() > 0.1
		ttft := uint32(200 + rng.Intn(2200))
		at := now.Add(-time.Duration(rng.Intn(3600)) * time.Second)
		in := 500 + rng.Intn(40000)
		rec := RequestRecord{
			TraceID:     fmt.Sprintf("seed-%04d", i),
			GroupID:     groupID,
			ChannelID:   channelByType[ct],
			ChannelType: ct,
			Model:       "claude-3-5-sonnet-20241022",
			RequestAt:   at,
			CompletedAt: at.Add(time.Duration(ttft+uint32(rng.Intn(8000))) * time.Millisecond),
			TTFTMs:      ttft,
			DurationMs:  ttft + uint32(rng.Intn(8000)),
			IsStreaming: rng.Float64() > 0.4,
			IsSuccess:   success,
			StatusCode:  map[bool]int{true: 200, false: 500}[success],
			Billed:      domain.Usage{InputTokens: 1, OutputTokens: rng.Intn(2000), CacheCreationTokens: in / 10, CacheReadTokens: in},
			Upstream:    domain.Usage{InputTokens: in, OutputTokens: rng.Intn(2000), CacheCreationTokens: in / 8, CacheReadTokens: in},
		}
		if !success {
			rec.ErrorType = errTypes[rng.Intn(len(errTypes))]
		}
		m.Write(context.Background(), rec)
	}
}

// ---- 辅助 ----

func percentile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(q * float64(len(sorted)-1))
	return sorted[idx]
}

func granularityDur(g string) time.Duration {
	switch g {
	case "1h":
		return time.Hour
	case "5m":
		return 5 * time.Minute
	default:
		return time.Minute
	}
}

func round1(f float64) float64 { return float64(int(f*10)) / 10 }
