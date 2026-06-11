package clickhouse

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/observ"
)

// 集成测试：需真实 ClickHouse。设 CG_TEST_CH_ADDR 后运行；未设则跳过。
func TestSinkAndMetrics(t *testing.T) {
	addr := os.Getenv("CG_TEST_CH_ADDR")
	if addr == "" {
		t.Skip("未设 CG_TEST_CH_ADDR，跳过 ClickHouse 集成测试")
	}
	ctx := context.Background()
	s, err := New(ctx, Options{Addr: addr, Database: "claude_gate", BatchSize: 1000, FlushInterval: 100 * time.Millisecond})
	if err != nil {
		t.Fatalf("连接 CH 失败: %v", err)
	}
	defer s.Close(ctx)

	const gid = int64(990001) // 唯一分组隔离本测试数据
	now := time.Now()
	for i := 0; i < 6; i++ {
		success := i%3 != 0
		s.Write(ctx, observ.RequestRecord{
			TraceID: "chtest-" + time.Now().Format("150405.000000") + string(rune('a'+i)),
			GroupID: gid, ChannelID: 1, ChannelType: domain.ChannelCustom, Model: "m",
			RequestAt: now, CompletedAt: now, TTFTMs: uint32(100 + i*100), DurationMs: 2000,
			StatusCode: 200, IsStreaming: i%2 == 0, IsSuccess: success,
			ErrorType: map[bool]string{true: "", false: "upstream_timeout"}[success],
			Billed:    domain.Usage{InputTokens: 1, OutputTokens: 10, CacheReadTokens: 500},
			Upstream:  domain.Usage{InputTokens: 100, OutputTokens: 10, CacheReadTokens: 500},
		})
	}
	time.Sleep(500 * time.Millisecond) // 等后台批写 flush

	ov, err := s.Overview(ctx, observ.StatsQuery{GroupID: gid})
	if err != nil {
		t.Fatal(err)
	}
	if ov.RequestCount != 6 {
		t.Fatalf("Overview count = %d, 期望 6", ov.RequestCount)
	}
	if ov.ErrorCount != 2 { // i=0,3 失败
		t.Fatalf("ErrorCount = %d, 期望 2", ov.ErrorCount)
	}
	if ov.TotalTokens == 0 {
		t.Fatal("TotalTokens 不应为 0")
	}

	list, err := s.ListTraces(ctx, observ.TraceQuery{GroupID: gid, Page: 1, PageSize: 10})
	if err != nil || list.Total != 6 {
		t.Fatalf("ListTraces total = %d, err=%v", list.Total, err)
	}
	// 详情往返：billed/upstream usage、流式标志
	one, err := s.GetTrace(ctx, list.Items[0].TraceID)
	if err != nil || one.Billed.CacheReadTokens != 500 || one.Upstream.InputTokens != 100 {
		t.Fatalf("GetTrace 往返失败: %v %+v", err, one)
	}

	bc, err := s.ByChannel(ctx, observ.StatsQuery{GroupID: gid})
	if err != nil || len(bc) == 0 || bc[0].ChannelType != domain.ChannelCustom {
		t.Fatalf("ByChannel 失败: %v %+v", err, bc)
	}
}
