package domain

// Usage 表示一次请求的 token 计量。
//
// claude-gate 在一次请求中会同时持有两份 Usage：
//   - 上游真实值（upstream）：用于成本核算与审计；
//   - 计费值（billed）：经缓存计费策略改写后返回给客户。
//
// 两份值都会写入 ClickHouse 明细（见 migrations/clickhouse）。
type Usage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CacheCreationTokens int `json:"cache_creation_tokens"`
	CacheReadTokens     int `json:"cache_read_tokens"`
}

// Total 返回四个字段之和，常用于日志与粗略估算。
func (u Usage) Total() int {
	return u.InputTokens + u.OutputTokens + u.CacheCreationTokens + u.CacheReadTokens
}

// Sanitize 把所有字段中的负数归零。
//
// 计费策略允许出现公式计算结果为负（例如 total - a - b），
// 但返回给客户的 usage 不应为负数，统一在出口处做防御。
func (u Usage) Sanitize() Usage {
	clamp := func(v int) int {
		if v < 0 {
			return 0
		}
		return v
	}
	return Usage{
		InputTokens:         clamp(u.InputTokens),
		OutputTokens:        clamp(u.OutputTokens),
		CacheCreationTokens: clamp(u.CacheCreationTokens),
		CacheReadTokens:     clamp(u.CacheReadTokens),
	}
}
