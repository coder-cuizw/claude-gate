// Package clickhouse 用 clickhouse-go 实现明细落库（observ.Sink）与统计查询
// （observ.MetricsReader）。
//
// 写入遵循任务书 §2.1：带缓冲队列 + 后台 worker 批量 flush（按条数或时间），
// 队列满时降级——保留错误、丢弃成功采样，绝不反压代理主链路。
package clickhouse

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/claude-gate/claude-gate/internal/observ"
)

// Store 实现 observ.Sink 与 observ.MetricsReader。
type Store struct {
	conn   driver.Conn
	logger *slog.Logger

	buf      chan observ.RequestRecord
	done     chan struct{}
	wg       sync.WaitGroup
	dropped  atomic.Int64
	batchN   int
	flushDur time.Duration
}

var (
	_ observ.Sink          = (*Store)(nil)
	_ observ.MetricsReader = (*Store)(nil)
)

// Options 配置 ClickHouse 连接与批写参数。
type Options struct {
	Addr          string // host:9000（native）
	Database      string
	Username      string
	Password      string
	QueueSize     int           // 缓冲队列容量，默认 8192
	BatchSize     int           // 批量条数阈值，默认 1000
	FlushInterval time.Duration // 批量时间阈值，默认 1s
	Logger        *slog.Logger
}

// New 连接 ClickHouse 并启动后台批写 worker。
func New(ctx context.Context, o Options) (*Store, error) {
	if o.QueueSize <= 0 {
		o.QueueSize = 8192
	}
	if o.BatchSize <= 0 {
		o.BatchSize = 1000
	}
	if o.FlushInterval <= 0 {
		o.FlushInterval = time.Second
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{o.Addr},
		Auth: clickhouse.Auth{Database: o.Database, Username: o.Username, Password: o.Password},
	})
	if err != nil {
		return nil, fmt.Errorf("打开 ClickHouse 失败: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := conn.Ping(pingCtx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ClickHouse 探活失败: %w", err)
	}
	s := &Store{
		conn:     conn,
		logger:   o.Logger,
		buf:      make(chan observ.RequestRecord, o.QueueSize),
		done:     make(chan struct{}),
		batchN:   o.BatchSize,
		flushDur: o.FlushInterval,
	}
	s.wg.Add(1)
	go s.run()
	return s, nil
}

// Write 入队一条明细（非阻塞）。队列满时：错误强行短暂等待入队，成功直接丢弃并计数。
func (s *Store) Write(_ context.Context, rec observ.RequestRecord) {
	select {
	case s.buf <- rec:
	default:
		if !rec.IsSuccess {
			select {
			case s.buf <- rec:
			case <-time.After(50 * time.Millisecond):
				s.dropped.Add(1)
			}
			return
		}
		s.dropped.Add(1) // 成功采样在队列满时优先丢弃（任务书 §2.1 降级策略）
	}
}

// Close 停止 worker 并 flush 剩余缓冲。
func (s *Store) Close(_ context.Context) error {
	close(s.done)
	s.wg.Wait()
	if d := s.dropped.Load(); d > 0 {
		s.logger.Warn("ClickHouse 批写降级丢弃计数", "dropped", d)
	}
	return s.conn.Close()
}

// Ping 供 readyz 探活。
func (s *Store) Ping(ctx context.Context) error { return s.conn.Ping(ctx) }

func (s *Store) run() {
	defer s.wg.Done()
	batch := make([]observ.RequestRecord, 0, s.batchN)
	ticker := time.NewTicker(s.flushDur)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := s.flush(batch); err != nil {
			s.logger.Error("ClickHouse 批写失败", "err", err, "n", len(batch))
		}
		batch = batch[:0]
	}
	for {
		select {
		case rec := <-s.buf:
			batch = append(batch, rec)
			if len(batch) >= s.batchN {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-s.done:
			// 退出前把队列里剩余的全部 drain 并 flush
			for {
				select {
				case rec := <-s.buf:
					batch = append(batch, rec)
					if len(batch) >= s.batchN {
						flush()
					}
				default:
					flush()
					return
				}
			}
		}
	}
}

const insertSQL = `INSERT INTO request_logs (
	trace_id, api_key_id, group_id, channel_id, channel_type, upstream_key_id, model,
	request_at, first_token_at, completed_at, ttft_ms, duration_ms,
	status_code, is_streaming, is_success, error_type, error_message,
	input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
	upstream_input_tokens, upstream_output_tokens, upstream_cache_creation_tokens, upstream_cache_read_tokens,
	request_body_s3_key, response_body_s3_key)`

func (s *Store) flush(recs []observ.RequestRecord) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	batch, err := s.conn.PrepareBatch(ctx, insertSQL)
	if err != nil {
		return err
	}
	for _, r := range recs {
		if err := batch.Append(
			r.TraceID, uint64(r.APIKeyID), uint64(r.GroupID), uint64(r.ChannelID), string(r.ChannelType), uint64(r.UpstreamKeyID), r.Model,
			r.RequestAt, r.FirstTokenAt, r.CompletedAt, r.TTFTMs, r.DurationMs,
			uint16(r.StatusCode), b2u8(r.IsStreaming), b2u8(r.IsSuccess), r.ErrorType, r.ErrorMessage,
			uint32(r.Billed.InputTokens), uint32(r.Billed.OutputTokens), uint32(r.Billed.CacheCreationTokens), uint32(r.Billed.CacheReadTokens),
			uint32(r.Upstream.InputTokens), uint32(r.Upstream.OutputTokens), uint32(r.Upstream.CacheCreationTokens), uint32(r.Upstream.CacheReadTokens),
			r.RequestBodyS3Key, r.ResponseBodyS3Key,
		); err != nil {
			return err
		}
	}
	return batch.Send()
}

func b2u8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}
