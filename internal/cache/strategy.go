// Package cache 实现缓存计费策略引擎（任务书 §5.3 ⭐ 核心）。
//
// 职责：基于上游真实 usage，按分组配置的策略，计算返回给客户的计费 usage。
// 系统始终保留两份 usage——上游真实值用于成本核算，计费值返回给客户。
//
// 内置四种策略：
//   - passthrough：透传上游真实值
//   - percentage ：按 TotalContextTokens 的比例分配
//   - fixed      ：全部使用固定值
//   - formula    ：表达式引擎，可自由组合各变量
//
// 策略通过 New 工厂从 domain.CacheStrategyConfig 构造，配置变更可热加载，
// 无需重启进程。
package cache

import (
	"encoding/json"
	"fmt"

	"github.com/claude-gate/claude-gate/internal/domain"
)

// Usage 是计量结果，复用领域模型，避免重复定义。
type Usage = domain.Usage

// Context 是计费计算的上下文输入。
type Context struct {
	// Model 本次请求使用的模型名。
	Model string
	// TotalContextTokens 入参总 tokens（system + messages），
	// 是 percentage / formula 策略里 total 变量的来源。
	TotalContextTokens int
	// Params 预留的运行时上下文，策略一般从自身配置取参，此处仅作扩展。
	Params map[string]any
}

// Strategy 是缓存计费策略统一接口。
//
// 实现必须是无状态、并发安全的：同一个 Strategy 实例会被多请求并发调用。
type Strategy interface {
	// Name 返回策略类型名，与配置中的 type 一致。
	Name() string
	// Compute 基于上游真实 usage 与上下文，计算返回给客户的计费 usage。
	// 返回值保证非负（内部已做 Sanitize）。
	Compute(upstream Usage, ctx Context) Usage
}

// New 按配置构造策略实例。配置不合法时返回错误。
//
// 这是策略的唯一入口；新增策略类型只需在此注册并实现 Strategy 接口。
func New(cfg domain.CacheStrategyConfig) (Strategy, error) {
	switch cfg.Type {
	case "", TypePassthrough:
		return NewPassthrough(), nil
	case TypePercentage:
		return NewPercentage(cfg.Params)
	case TypeFixed:
		return NewFixed(cfg.Params)
	case TypeFormula:
		return NewFormula(cfg.Params)
	default:
		return nil, fmt.Errorf("cache: 未知的策略类型 %q", cfg.Type)
	}
}

// 策略类型常量。
const (
	TypePassthrough = "passthrough"
	TypePercentage  = "percentage"
	TypeFixed       = "fixed"
	TypeFormula     = "formula"
)

// ---- 参数解析辅助 ----

// paramFloat 从参数 map 读取 float64，缺失时返回默认值。
// 兼容 JSON 反序列化得到的 float64 / int / int64 等数值类型。
func paramFloat(params map[string]any, key string, def float64) float64 {
	v, ok := params[key]
	if !ok {
		return def
	}
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		f, err := n.Float64()
		if err == nil {
			return f
		}
	}
	return def
}

// paramInt 从参数 map 读取 int，缺失时返回默认值。
func paramInt(params map[string]any, key string, def int) int {
	return int(paramFloat(params, key, float64(def)))
}

// paramString 从参数 map 读取字符串，缺失时返回默认值。
func paramString(params map[string]any, key, def string) string {
	if v, ok := params[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}
