package cache

// FixedStrategy 固定值策略：所有字段返回配置的固定值。
//
// 约定：output_tokens 配置为 0 时表示"走上游真实值"（因为固定输出为 0 token
// 在实际计费中无意义）。其余字段固定为 0 是合法的。
//
// 配置示例：
//
//	type: fixed
//	params:
//	  input_tokens: 1
//	  output_tokens: 0      # 0 表示走 upstream
//	  cache_creation_tokens: 1000
//	  cache_read_tokens: 5000
type FixedStrategy struct {
	inputTokens         int
	outputTokens        int
	cacheCreationTokens int
	cacheReadTokens     int
}

// NewFixed 从参数构造固定值策略。
func NewFixed(params map[string]any) (*FixedStrategy, error) {
	return &FixedStrategy{
		inputTokens:         paramInt(params, "input_tokens", 0),
		outputTokens:        paramInt(params, "output_tokens", 0),
		cacheCreationTokens: paramInt(params, "cache_creation_tokens", 0),
		cacheReadTokens:     paramInt(params, "cache_read_tokens", 0),
	}, nil
}

// Name 返回策略名。
func (s *FixedStrategy) Name() string { return TypeFixed }

// Compute 返回固定计费 usage；output 为 0 时回退到上游真实输出。
func (s *FixedStrategy) Compute(upstream Usage, _ Context) Usage {
	output := s.outputTokens
	if output == 0 {
		output = upstream.OutputTokens
	}
	out := Usage{
		InputTokens:         s.inputTokens,
		OutputTokens:        output,
		CacheCreationTokens: s.cacheCreationTokens,
		CacheReadTokens:     s.cacheReadTokens,
	}
	return out.Sanitize()
}
