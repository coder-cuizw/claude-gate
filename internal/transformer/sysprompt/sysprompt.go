// Package sysprompt 实现 system_prompt_injector 改写器：按配置注入或剥离 system prompt。
package sysprompt

import (
	"context"
	"encoding/json"

	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/transformer"
)

// Mode 注入模式。
type Mode string

const (
	// ModeInject 在已有 system 之前注入一段前缀文本（若已有则拼接）。
	ModeInject Mode = "inject"
	// ModeOverride 用配置文本完全覆盖 system。
	ModeOverride Mode = "override"
	// ModeStrip 剥离 system（置空）。
	ModeStrip Mode = "strip"
)

// Injector system prompt 改写器。
type Injector struct {
	transformer.Base
	mode Mode
	text string
}

// New 构造 Injector。mode 非法时退化为 inject。
func New(mode Mode, text string) *Injector {
	switch mode {
	case ModeInject, ModeOverride, ModeStrip:
	default:
		mode = ModeInject
	}
	return &Injector{mode: mode, text: text}
}

// Name 返回改写器名。
func (i *Injector) Name() string { return "system_prompt_injector" }

// TransformRequest 按模式改写 system 字段。
//
// 为简化处理，统一把 system 规整为字符串形态（Anthropic 也接受 string）。
func (i *Injector) TransformRequest(_ context.Context, req *domain.MessagesRequest) (*domain.MessagesRequest, error) {
	if req == nil {
		return req, nil
	}
	clone := *req
	switch i.mode {
	case ModeStrip:
		clone.System = nil
	case ModeOverride:
		clone.System = mustJSONString(i.text)
	case ModeInject:
		existing := decodeSystemText(req.System)
		combined := i.text
		if existing != "" {
			combined = i.text + "\n\n" + existing
		}
		clone.System = mustJSONString(combined)
	}
	return &clone, nil
}

// decodeSystemText 把 system 字段（可能是 string 或 block 数组）粗略解码为文本。
func decodeSystemText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// 先尝试按字符串解析
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// 再尝试按 [{type:text, text:...}] 解析
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		out := ""
		for _, b := range blocks {
			if b.Type == "text" {
				if out != "" {
					out += "\n"
				}
				out += b.Text
			}
		}
		return out
	}
	return ""
}

// mustJSONString 把字符串编码为 JSON RawMessage；编码失败返回 null。
func mustJSONString(s string) json.RawMessage {
	b, err := json.Marshal(s)
	if err != nil {
		return json.RawMessage("null")
	}
	return b
}
