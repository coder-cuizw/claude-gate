package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	srv := NewServer(nil, nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz = %d", rec.Code)
	}
}

func TestReadyzNotReady(t *testing.T) {
	srv := NewServer(nil, func() bool { return false })
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("未就绪应返回 503, 得到 %d", rec.Code)
	}
}

func TestMessagesTraceHeaderAndAuth(t *testing.T) {
	srv := NewServer(nil, nil)
	rec := httptest.NewRecorder()
	// 不带 Authorization → 401，且响应头必须带 X-Trace-Id
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/messages", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("缺少凭证应返回 401, 得到 %d", rec.Code)
	}
	if rec.Header().Get("X-Trace-Id") == "" {
		t.Fatal("响应必须带 X-Trace-Id")
	}
}
