// Package factory 按分组配置组装 Transformer 流水线（任务书 §5.4）。
//
// 单独成包是为了避免父包 transformer 反向依赖各子改写器（子改写器需引用
// 父包的 Transformer 接口，父包再引用子包会形成循环）。
package factory

import (
	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/transformer"
	"github.com/claude-gate/claude-gate/internal/transformer/modelmap"
	"github.com/claude-gate/claude-gate/internal/transformer/streaming"
	"github.com/claude-gate/claude-gate/internal/transformer/sysprompt"
	"github.com/claude-gate/claude-gate/internal/transformer/toolcall"
)

// 内置改写器名称（与分组配置 transformer_config[].name 对应）。
const (
	NameToolCallNormalizer   = "tool_call_normalizer"
	NameSystemPromptInjector = "system_prompt_injector"
	NameModelMapper          = "model_mapper"
	NameStreamingEventFixer  = "streaming_event_fixer"
)

// Build 按分组配置与模型映射组装流水线。
//
// 顺序由配置决定，前序输出是后序输入；未启用的环节跳过；失败策略由
// params.fail_mode 指定（fail-fast / skip），默认 fail-fast。新增改写器只需
// 在此 switch 注册，框架代码不动（任务书 §5.4 验收）。
func Build(configs []domain.TransformerConfig, modelMapping map[string]string) *transformer.Pipeline {
	p := transformer.NewPipeline()
	for _, c := range configs {
		if !c.Enabled {
			continue
		}
		var t transformer.Transformer
		switch c.Name {
		case NameToolCallNormalizer:
			t = toolcall.New()
		case NameStreamingEventFixer:
			t = streaming.New()
		case NameModelMapper:
			t = modelmap.New(modelMapping)
		case NameSystemPromptInjector:
			mode := sysprompt.Mode(paramString(c.Params, "mode", string(sysprompt.ModeInject)))
			t = sysprompt.New(mode, paramString(c.Params, "text", ""))
		default:
			continue // 未知改写器名忽略，不影响主链路
		}
		p.Use(t, failModeOf(c.Params))
	}
	return p
}

func failModeOf(params map[string]any) transformer.FailMode {
	if paramString(params, "fail_mode", "") == string(transformer.FailSkip) {
		return transformer.FailSkip
	}
	return transformer.FailFast
}

func paramString(params map[string]any, key, def string) string {
	if params == nil {
		return def
	}
	if v, ok := params[key].(string); ok && v != "" {
		return v
	}
	return def
}
