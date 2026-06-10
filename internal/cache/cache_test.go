package cache

import (
	"encoding/json"
	"testing"

	"github.com/claude-gate/claude-gate/internal/domain"
)

// 上游真实 usage 样例，供各用例复用。
var sampleUpstream = Usage{
	InputTokens:         200,
	OutputTokens:        500,
	CacheCreationTokens: 1000,
	CacheReadTokens:     8000,
}

func TestPassthrough(t *testing.T) {
	s := NewPassthrough()
	if s.Name() != TypePassthrough {
		t.Fatalf("Name = %q, 期望 %q", s.Name(), TypePassthrough)
	}
	got := s.Compute(sampleUpstream, Context{})
	if got != sampleUpstream {
		t.Fatalf("透传结果 = %+v, 期望 %+v", got, sampleUpstream)
	}
}

func TestPassthroughSanitizesNegative(t *testing.T) {
	s := NewPassthrough()
	got := s.Compute(Usage{InputTokens: -5, OutputTokens: 10}, Context{})
	if got.InputTokens != 0 {
		t.Fatalf("负数 input 应被归零, 得到 %d", got.InputTokens)
	}
	if got.OutputTokens != 10 {
		t.Fatalf("output 应保留, 得到 %d", got.OutputTokens)
	}
}

func TestPercentage(t *testing.T) {
	s, err := NewPercentage(map[string]any{
		"cache_creation_ratio": 0.1,
		"cache_read_ratio":     0.9,
		"input_fixed_tokens":   1,
		"output_source":        "upstream",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := s.Compute(sampleUpstream, Context{TotalContextTokens: 10000})
	want := Usage{
		InputTokens:         1,
		OutputTokens:        500, // 来自 upstream
		CacheCreationTokens: 1000,
		CacheReadTokens:     9000,
	}
	if got != want {
		t.Fatalf("按比例结果 = %+v, 期望 %+v", got, want)
	}
}

func TestPercentageOutputFixed(t *testing.T) {
	s, _ := NewPercentage(map[string]any{
		"cache_creation_ratio": 0.2,
		"cache_read_ratio":     0.5,
		"input_fixed_tokens":   3,
		"output_source":        "fixed:42",
	})
	got := s.Compute(sampleUpstream, Context{TotalContextTokens: 100})
	if got.OutputTokens != 42 {
		t.Fatalf("output 固定值 = %d, 期望 42", got.OutputTokens)
	}
	if got.CacheCreationTokens != 20 || got.CacheReadTokens != 50 {
		t.Fatalf("比例计算错误: %+v", got)
	}
}

func TestPercentageFixedOutputInvalidFallsBack(t *testing.T) {
	s, _ := NewPercentage(map[string]any{"output_source": "fixed:abc"})
	got := s.Compute(sampleUpstream, Context{TotalContextTokens: 100})
	if got.OutputTokens != sampleUpstream.OutputTokens {
		t.Fatalf("非法 fixed 应回退上游 output, 得到 %d", got.OutputTokens)
	}
}

func TestPercentageRatioClamp(t *testing.T) {
	// 比例越界应被钳制到 [0,1]
	s, _ := NewPercentage(map[string]any{
		"cache_creation_ratio": 5.0,  // → 1.0
		"cache_read_ratio":     -2.0, // → 0.0
	})
	got := s.Compute(sampleUpstream, Context{TotalContextTokens: 1000})
	if got.CacheCreationTokens != 1000 {
		t.Fatalf("越界比例未钳制为 1.0: %d", got.CacheCreationTokens)
	}
	if got.CacheReadTokens != 0 {
		t.Fatalf("负比例未钳制为 0: %d", got.CacheReadTokens)
	}
}

func TestPercentageTotalZero(t *testing.T) {
	// 边界：total=0
	s, _ := NewPercentage(map[string]any{
		"cache_creation_ratio": 0.5,
		"cache_read_ratio":     0.5,
		"input_fixed_tokens":   7,
	})
	got := s.Compute(sampleUpstream, Context{TotalContextTokens: 0})
	if got.CacheCreationTokens != 0 || got.CacheReadTokens != 0 {
		t.Fatalf("total=0 时缓存字段应为 0: %+v", got)
	}
	if got.InputTokens != 7 {
		t.Fatalf("input 固定值应保留: %d", got.InputTokens)
	}
}

func TestPercentageNegativeTotalDefense(t *testing.T) {
	s, _ := NewPercentage(map[string]any{"cache_read_ratio": 0.9})
	got := s.Compute(sampleUpstream, Context{TotalContextTokens: -100})
	if got.CacheReadTokens != 0 {
		t.Fatalf("负 total 应按 0 处理: %d", got.CacheReadTokens)
	}
}

func TestFixed(t *testing.T) {
	s, err := NewFixed(map[string]any{
		"input_tokens":          1,
		"output_tokens":         0, // 走 upstream
		"cache_creation_tokens": 1000,
		"cache_read_tokens":     5000,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := s.Compute(sampleUpstream, Context{})
	want := Usage{
		InputTokens:         1,
		OutputTokens:        500, // upstream
		CacheCreationTokens: 1000,
		CacheReadTokens:     5000,
	}
	if got != want {
		t.Fatalf("固定值结果 = %+v, 期望 %+v", got, want)
	}
}

func TestFixedOutputExplicit(t *testing.T) {
	s, _ := NewFixed(map[string]any{"output_tokens": 99})
	got := s.Compute(sampleUpstream, Context{})
	if got.OutputTokens != 99 {
		t.Fatalf("显式 output 固定值 = %d, 期望 99", got.OutputTokens)
	}
}

func TestFormulaSpecExample(t *testing.T) {
	// 任务书 §5.3 公式示例
	s, err := NewFormula(map[string]any{
		"input":          "1",
		"cache_creation": "total * 0.1",
		"cache_read":     "total - cache_creation - input",
		"output":         "upstream_output",
	})
	if err != nil {
		t.Fatal(err)
	}
	got := s.Compute(sampleUpstream, Context{TotalContextTokens: 10000})
	want := Usage{
		InputTokens:         1,
		CacheCreationTokens: 1000,             // 10000 * 0.1
		CacheReadTokens:     10000 - 1000 - 1, // 引用先序结果
		OutputTokens:        500,              // upstream_output
	}
	if got != want {
		t.Fatalf("公式结果 = %+v, 期望 %+v", got, want)
	}
}

func TestFormulaAllUpstreamVars(t *testing.T) {
	s, _ := NewFormula(map[string]any{
		"input":          "upstream_input + upstream_cache_read",
		"cache_creation": "upstream_cache_creation",
		"cache_read":     "upstream_cache_read",
		"output":         "upstream_output * 2",
	})
	got := s.Compute(sampleUpstream, Context{TotalContextTokens: 1})
	if got.InputTokens != 200+8000 {
		t.Fatalf("input = %d, 期望 8200", got.InputTokens)
	}
	if got.OutputTokens != 1000 {
		t.Fatalf("output = %d, 期望 1000", got.OutputTokens)
	}
}

func TestFormulaNegativeResultSanitized(t *testing.T) {
	// 公式算出负数时，出口处应被归零
	s, _ := NewFormula(map[string]any{
		"cache_read": "0 - 9999",
	})
	got := s.Compute(sampleUpstream, Context{TotalContextTokens: 100})
	if got.CacheReadTokens != 0 {
		t.Fatalf("负数公式结果应归零, 得到 %d", got.CacheReadTokens)
	}
}

func TestFormulaEmptyFieldFallsBackToUpstream(t *testing.T) {
	// 仅给 input 表达式，其余字段回退到上游真实值
	s, _ := NewFormula(map[string]any{"input": "0"})
	got := s.Compute(sampleUpstream, Context{TotalContextTokens: 100})
	if got.InputTokens != 0 {
		t.Fatalf("input 表达式应生效, 得到 %d", got.InputTokens)
	}
	if got.OutputTokens != sampleUpstream.OutputTokens {
		t.Fatalf("空 output 表达式应回退上游, 得到 %d", got.OutputTokens)
	}
	if got.CacheReadTokens != sampleUpstream.CacheReadTokens {
		t.Fatalf("空 cache_read 表达式应回退上游, 得到 %d", got.CacheReadTokens)
	}
}

func TestFormulaInvalidExpressionReturnsError(t *testing.T) {
	_, err := NewFormula(map[string]any{"input": "total +"})
	if err == nil {
		t.Fatal("非法表达式应返回编译错误")
	}
}

func TestFormulaRuntimeErrorFallsBack(t *testing.T) {
	// 引用未知变量在编译期即报错；这里验证除零等运行期问题回退到 fallback。
	// total/0 在 expr 中会触发运行期 panic→error，evalOr 回退到 upstream。
	s, err := NewFormula(map[string]any{"input": "total / 0"})
	if err != nil {
		// 某些版本会在编译期发现常量除零；只要不 panic 即可
		return
	}
	got := s.Compute(sampleUpstream, Context{TotalContextTokens: 100})
	if got.InputTokens != sampleUpstream.InputTokens {
		t.Fatalf("运行期错误应回退上游 input, 得到 %d", got.InputTokens)
	}
}

func TestFormulaHugeNumbers(t *testing.T) {
	// 超大数值不应 panic / 溢出为负
	s, _ := NewFormula(map[string]any{
		"input":      "total * 1000",
		"cache_read": "total * total",
	})
	got := s.Compute(sampleUpstream, Context{TotalContextTokens: 2_000_000})
	if got.InputTokens < 0 || got.CacheReadTokens < 0 {
		t.Fatalf("超大数值不应为负: %+v", got)
	}
}

func TestNewFactory(t *testing.T) {
	cases := []struct {
		cfg  domain.CacheStrategyConfig
		name string
	}{
		{domain.CacheStrategyConfig{Type: "passthrough"}, TypePassthrough},
		{domain.CacheStrategyConfig{Type: ""}, TypePassthrough}, // 空类型默认透传
		{domain.CacheStrategyConfig{Type: "percentage"}, TypePercentage},
		{domain.CacheStrategyConfig{Type: "fixed"}, TypeFixed},
		{domain.CacheStrategyConfig{Type: "formula", Params: map[string]any{"input": "1"}}, TypeFormula},
	}
	for _, c := range cases {
		s, err := New(c.cfg)
		if err != nil {
			t.Fatalf("New(%q) 出错: %v", c.cfg.Type, err)
		}
		if s.Name() != c.name {
			t.Fatalf("New(%q).Name() = %q, 期望 %q", c.cfg.Type, s.Name(), c.name)
		}
	}
}

func TestNewFactoryUnknownType(t *testing.T) {
	_, err := New(domain.CacheStrategyConfig{Type: "nope"})
	if err == nil {
		t.Fatal("未知策略类型应返回错误")
	}
}

func TestNewFactoryInvalidFormula(t *testing.T) {
	_, err := New(domain.CacheStrategyConfig{Type: "formula", Params: map[string]any{"input": ")("}})
	if err == nil {
		t.Fatal("非法公式应在工厂层返回错误")
	}
}

// paramFloat 兼容多种数值类型。
func TestParamFloatTypes(t *testing.T) {
	if got := paramFloat(map[string]any{"x": 1.5}, "x", 0); got != 1.5 {
		t.Fatalf("float64: %v", got)
	}
	if got := paramFloat(map[string]any{"x": 3}, "x", 0); got != 3 {
		t.Fatalf("int: %v", got)
	}
	if got := paramFloat(map[string]any{"x": float32(2.5)}, "x", 0); got != 2.5 {
		t.Fatalf("float32: %v", got)
	}
	if got := paramFloat(map[string]any{"x": int64(8)}, "x", 0); got != 8 {
		t.Fatalf("int64: %v", got)
	}
	if got := paramFloat(map[string]any{"x": json.Number("4.25")}, "x", 0); got != 4.25 {
		t.Fatalf("json.Number: %v", got)
	}
	if got := paramFloat(map[string]any{"x": json.Number("bad")}, "x", 7); got != 7 {
		t.Fatalf("非法 json.Number 应回退默认值: %v", got)
	}
	if got := paramFloat(map[string]any{"x": "str"}, "x", 5); got != 5 {
		t.Fatalf("不支持类型应回退默认值: %v", got)
	}
	if got := paramFloat(map[string]any{}, "x", 9); got != 9 {
		t.Fatalf("默认值: %v", got)
	}
}

func TestParamStringDefault(t *testing.T) {
	if got := paramString(map[string]any{"x": 123}, "x", "def"); got != "def" {
		t.Fatalf("非字符串应回退默认值: %v", got)
	}
}

// NewFormula 对每个字段的编译错误都应返回错误。
func TestFormulaEachFieldCompileError(t *testing.T) {
	fields := []string{"input", "cache_creation", "cache_read", "output"}
	for _, f := range fields {
		_, err := NewFormula(map[string]any{f: "1 +"})
		if err == nil {
			t.Fatalf("字段 %q 的非法表达式应返回错误", f)
		}
	}
}

// evalOr 对 Inf 结果应回退到 fallback。
func TestFormulaInfResultFallsBack(t *testing.T) {
	s, err := NewFormula(map[string]any{"input": "1e308 * 1e308"})
	if err != nil {
		t.Fatal(err)
	}
	got := s.Compute(sampleUpstream, Context{TotalContextTokens: 1})
	if got.InputTokens != sampleUpstream.InputTokens {
		t.Fatalf("Inf 结果应回退上游 input, 得到 %d", got.InputTokens)
	}
}
