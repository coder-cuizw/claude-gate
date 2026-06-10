package cache

import (
	"fmt"
	"math"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
)

// FormulaStrategy 公式引擎策略。
//
// 每个计费字段都由一条表达式给出，表达式在构造时编译、运行时求值（热路径）。
// 求值按 input → cache_creation → cache_read → output 顺序进行，
// 后求值的字段可引用先求值字段的结果。
//
// 可用变量：
//   - total                    入参总 tokens（Context.TotalContextTokens）
//   - upstream_input           上游 input
//   - upstream_output          上游 output
//   - upstream_cache_creation  上游 cache_creation
//   - upstream_cache_read      上游 cache_read
//   - input / cache_creation / cache_read / output  先序已求值的计费字段
//
// 配置示例：
//
//	type: formula
//	params:
//	  input: "1"
//	  cache_creation: "total * 0.1"
//	  cache_read: "total - cache_creation - input"
//	  output: "upstream_output"
type FormulaStrategy struct {
	inputExpr         *vm.Program
	cacheCreationExpr *vm.Program
	cacheReadExpr     *vm.Program
	outputExpr        *vm.Program
}

// formulaEnv 是编译/求值用的变量环境，全部以 float64 承载，最终向下取整。
type formulaEnv struct {
	Total                 float64 `expr:"total"`
	UpstreamInput         float64 `expr:"upstream_input"`
	UpstreamOutput        float64 `expr:"upstream_output"`
	UpstreamCacheCreation float64 `expr:"upstream_cache_creation"`
	UpstreamCacheRead     float64 `expr:"upstream_cache_read"`
	Input                 float64 `expr:"input"`
	CacheCreation         float64 `expr:"cache_creation"`
	CacheRead             float64 `expr:"cache_read"`
	Output                float64 `expr:"output"`
}

// NewFormula 从参数构造公式策略。任一表达式编译失败都会返回错误，
// 便于前端公式编辑器做实时校验（任务书 §7 公式策略编辑器）。
func NewFormula(params map[string]any) (*FormulaStrategy, error) {
	s := &FormulaStrategy{}
	var err error
	if s.inputExpr, err = compileExpr(paramString(params, "input", "")); err != nil {
		return nil, fmt.Errorf("input 表达式非法: %w", err)
	}
	if s.cacheCreationExpr, err = compileExpr(paramString(params, "cache_creation", "")); err != nil {
		return nil, fmt.Errorf("cache_creation 表达式非法: %w", err)
	}
	if s.cacheReadExpr, err = compileExpr(paramString(params, "cache_read", "")); err != nil {
		return nil, fmt.Errorf("cache_read 表达式非法: %w", err)
	}
	if s.outputExpr, err = compileExpr(paramString(params, "output", "")); err != nil {
		return nil, fmt.Errorf("output 表达式非法: %w", err)
	}
	return s, nil
}

// compileExpr 编译一条表达式；空串返回 nil（表示该字段回退到上游值）。
func compileExpr(src string) (*vm.Program, error) {
	if src == "" {
		return nil, nil
	}
	return expr.Compile(src, expr.Env(formulaEnv{}), expr.AsFloat64())
}

// Name 返回策略名。
func (s *FormulaStrategy) Name() string { return TypeFormula }

// Compute 顺序求值四个字段，空表达式回退到对应上游值。
func (s *FormulaStrategy) Compute(upstream Usage, ctx Context) Usage {
	total := ctx.TotalContextTokens
	if total < 0 {
		total = 0
	}
	env := formulaEnv{
		Total:                 float64(total),
		UpstreamInput:         float64(upstream.InputTokens),
		UpstreamOutput:        float64(upstream.OutputTokens),
		UpstreamCacheCreation: float64(upstream.CacheCreationTokens),
		UpstreamCacheRead:     float64(upstream.CacheReadTokens),
	}

	out := Usage{}
	// input
	out.InputTokens = evalOr(s.inputExpr, &env, upstream.InputTokens)
	env.Input = float64(out.InputTokens)
	// cache_creation
	out.CacheCreationTokens = evalOr(s.cacheCreationExpr, &env, upstream.CacheCreationTokens)
	env.CacheCreation = float64(out.CacheCreationTokens)
	// cache_read
	out.CacheReadTokens = evalOr(s.cacheReadExpr, &env, upstream.CacheReadTokens)
	env.CacheRead = float64(out.CacheReadTokens)
	// output
	out.OutputTokens = evalOr(s.outputExpr, &env, upstream.OutputTokens)

	return out.Sanitize()
}

// evalOr 运行表达式并向下取整；program 为 nil 或运行出错时返回 fallback。
func evalOr(program *vm.Program, env *formulaEnv, fallback int) int {
	if program == nil {
		return fallback
	}
	v, err := expr.Run(program, env)
	if err != nil {
		return fallback
	}
	f, ok := v.(float64)
	if !ok {
		return fallback
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return fallback
	}
	return int(math.Floor(f))
}
