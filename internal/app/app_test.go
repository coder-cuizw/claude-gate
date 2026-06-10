package app

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/claude-gate/claude-gate/internal/config"
	"github.com/claude-gate/claude-gate/internal/store/memory"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	a, err := BuildMemory(config.Default(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("装配失败: %v", err)
	}
	return a
}

// 端到端：非流式代理走通透传分组（mock 通道），返回标准 Anthropic 响应。
func TestEndToEndUnary(t *testing.T) {
	a := newTestApp(t)
	rec := httptest.NewRecorder()
	body := `{"model":"claude-3-5-sonnet-20241022","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+memory.SelfTestAPIKey)
	a.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("状态码 = %d, body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("X-Trace-Id") == "" {
		t.Fatal("缺少 X-Trace-Id 响应头")
	}
	var resp struct {
		Usage struct {
			InputTokens int `json:"input_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	// mock usage：input = 100 + 1*50 = 150；透传策略不改写
	if resp.Usage.InputTokens != 150 {
		t.Fatalf("透传 usage input = %d, 期望 150", resp.Usage.InputTokens)
	}
}

// 端到端：流式代理产出标准 SSE，且最终 message_delta 携带 usage。
func TestEndToEndStream(t *testing.T) {
	a := newTestApp(t)
	rec := httptest.NewRecorder()
	body := `{"model":"m","stream":true,"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+memory.SelfTestAPIKey)
	a.Handler.ServeHTTP(rec, req)

	out := rec.Body.String()
	for _, want := range []string{"event: message_start", "event: content_block_delta", "event: message_delta", "event: message_stop"} {
		if !strings.Contains(out, want) {
			t.Fatalf("流式输出缺少 %q\n%s", want, out)
		}
	}
	if !strings.Contains(out, "cache_read_input_tokens") {
		t.Fatal("message_delta 未携带改写后的 usage")
	}
}

// 未带凭证应 401。
func TestMissingAuth(t *testing.T) {
	a := newTestApp(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader("{}"))
	a.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("无凭证应 401, 得到 %d", rec.Code)
	}
}

func login(t *testing.T, a *App) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(`{"email":"admin@claude-gate.io","password":"admin123"}`))
	a.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("登录失败: %d %s", rec.Code, rec.Body.String())
	}
	var r struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &r)
	if r.Token == "" {
		t.Fatal("未拿到 token")
	}
	return r.Token
}

// 管理 API：登录 + 鉴权 + 列表。
func TestAdminAuthAndList(t *testing.T) {
	a := newTestApp(t)
	token := login(t, a)

	// 无 token 拒绝
	rec := httptest.NewRecorder()
	a.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/admin/channels", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("无 token 应 401, 得到 %d", rec.Code)
	}

	// 带 token 放行
	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/channels", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	a.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("带 token 应 200, 得到 %d", rec.Code)
	}
	var chs []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &chs)
	if len(chs) != 4 {
		t.Fatalf("通道数 = %d, 期望 4（kiro/official/relay/custom）", len(chs))
	}
}

// 客户 Key 支持重复查看（reveal 解密）。
func TestRevealAPIKey(t *testing.T) {
	a := newTestApp(t)
	token := login(t, a)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/api-keys/1/reveal", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	a.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("reveal 应 200, 得到 %d %s", rec.Code, rec.Body.String())
	}
	var r struct {
		Plaintext string `json:"plaintext"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &r)
	if r.Plaintext != memory.SelfTestAPIKey {
		t.Fatalf("reveal 明文 = %q, 期望 %q", r.Plaintext, memory.SelfTestAPIKey)
	}
}

// 统计概览可返回（含演示种子明细）。
func TestStatsOverview(t *testing.T) {
	a := newTestApp(t)
	token := login(t, a)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/stats/overview", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	a.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("overview 应 200, 得到 %d", rec.Code)
	}
	var ov struct {
		RequestCount int64 `json:"request_count"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &ov)
	if ov.RequestCount == 0 {
		t.Fatal("演示种子下 request_count 不应为 0")
	}
}
