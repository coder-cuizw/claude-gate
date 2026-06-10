package cache

import (
	"math"
	"strconv"
	"strings"
)

// PercentageStrategy 按比例分配策略。
//
// 以入参总 tokens（Context.TotalContextTokens）为基数，按比例切分为
// cache_creation 与 cache_read，input 取固定值，output 取 upstream 或固定值。
//
// 配置示例：
//
//	type: percentage
//	params:
//	  cache_creation_ratio: 0.1     # 0.0 ~ 1.0
//	  cache_read_ratio: 0.9
//	  input_fixed_tokens: 1         # input 字段固定值
//	  output_source: upstream       # upstream / fixed:N
//
// 计算逻辑：
//
//	total          = TotalContextTokens
//	cache_creation = floor(total * cache_creation_ratio)
//	cache_read     = floor(total * cache_read_ratio)
//	input          = input_fixed_tokens
//	output         = 按 output_source
type PercentageStrategy struct {
	cacheCreationRatio float64
	cacheReadRatio     float64
	inputFixedTokens   int
	outputSource       string // "upstream" 或 "fixed:N"
}

// NewPercentage 从参数构造按比例策略，对比例做 [0,1] 钳制。
func NewPercentage(params map[string]any) (*PercentageStrategy, error) {
	s := &PercentageStrategy{
		cacheCreationRatio: clampRatio(paramFloat(params, "cache_creation_ratio", 0)),
		cacheReadRatio:     clampRatio(paramFloat(params, "cache_read_ratio", 0)),
		inputFixedTokens:   paramInt(params, "input_fixed_tokens", 1),
		outputSource:       paramString(params, "output_source", "upstream"),
	}
	return s, nil
}

// Name 返回策略名。
func (s *PercentageStrategy) Name() string { return TypePercentage }

// Compute 按比例分配计费 usage。
func (s *PercentageStrategy) Compute(upstream Usage, ctx Context) Usage {
	total := ctx.TotalContextTokens
	if total < 0 {
		total = 0
	}
	out := Usage{
		InputTokens:         s.inputFixedTokens,
		CacheCreationTokens: int(math.Floor(float64(total) * s.cacheCreationRatio)),
		CacheReadTokens:     int(math.Floor(float64(total) * s.cacheReadRatio)),
		OutputTokens:        resolveOutput(s.outputSource, upstream.OutputTokens),
	}
	return out.Sanitize()
}

// clampRatio 把比例钳制到 [0,1]。
func clampRatio(r float64) float64 {
	if r < 0 {
		return 0
	}
	if r > 1 {
		return 1
	}
	return r
}

// resolveOutput 解析 output_source：
//   - "upstream"  → 使用上游 output
//   - "fixed:N"   → 使用固定值 N
//   - 其它/解析失败 → 退回上游 output
func resolveOutput(source string, upstreamOutput int) int {
	if strings.HasPrefix(source, "fixed:") {
		n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(source, "fixed:")))
		if err == nil {
			return n
		}
	}
	return upstreamOutput
}
