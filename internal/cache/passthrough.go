package cache

// PassthroughStrategy 透传策略：直接返回上游真实 usage，不做任何改写。
//
// 配置示例：
//
//	type: passthrough
type PassthroughStrategy struct{}

// NewPassthrough 构造透传策略。
func NewPassthrough() *PassthroughStrategy { return &PassthroughStrategy{} }

// Name 返回策略名。
func (s *PassthroughStrategy) Name() string { return TypePassthrough }

// Compute 原样返回上游 usage（仍做非负防御）。
func (s *PassthroughStrategy) Compute(upstream Usage, _ Context) Usage {
	return upstream.Sanitize()
}
