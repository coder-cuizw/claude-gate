package sysprompt

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/claude-gate/claude-gate/internal/domain"
)

func decode(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("system 不是字符串: %s", raw)
	}
	return s
}

func TestInjectPrepend(t *testing.T) {
	i := New(ModeInject, "你是受控助手")
	req := &domain.MessagesRequest{System: json.RawMessage(`"原始提示"`)}
	out, err := i.TransformRequest(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	got := decode(t, out.System)
	if !strings.HasPrefix(got, "你是受控助手") || !strings.Contains(got, "原始提示") {
		t.Fatalf("注入结果错误: %q", got)
	}
}

func TestInjectIntoEmpty(t *testing.T) {
	i := New(ModeInject, "前缀")
	out, _ := i.TransformRequest(context.Background(), &domain.MessagesRequest{})
	if decode(t, out.System) != "前缀" {
		t.Fatalf("空 system 注入错误: %q", decode(t, out.System))
	}
}

func TestOverride(t *testing.T) {
	i := New(ModeOverride, "覆盖文本")
	out, _ := i.TransformRequest(context.Background(), &domain.MessagesRequest{System: json.RawMessage(`"旧"`)})
	if decode(t, out.System) != "覆盖文本" {
		t.Fatalf("覆盖错误: %q", decode(t, out.System))
	}
}

func TestStrip(t *testing.T) {
	i := New(ModeStrip, "")
	out, _ := i.TransformRequest(context.Background(), &domain.MessagesRequest{System: json.RawMessage(`"旧"`)})
	if len(out.System) != 0 {
		t.Fatalf("剥离后 system 应为空: %s", out.System)
	}
}

func TestInjectParsesBlockArraySystem(t *testing.T) {
	i := New(ModeInject, "前缀")
	blocks := json.RawMessage(`[{"type":"text","text":"块文本"}]`)
	out, _ := i.TransformRequest(context.Background(), &domain.MessagesRequest{System: blocks})
	got := decode(t, out.System)
	if !strings.Contains(got, "块文本") {
		t.Fatalf("未能解析 block 数组 system: %q", got)
	}
}

func TestInvalidModeDefaultsToInject(t *testing.T) {
	i := New(Mode("garbage"), "x")
	if i.mode != ModeInject {
		t.Fatalf("非法模式应退化为 inject, 得到 %q", i.mode)
	}
	if i.Name() != "system_prompt_injector" {
		t.Fatalf("name = %q", i.Name())
	}
}
