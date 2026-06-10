// Package gateway 实现网关 HTTP 入口与代理逻辑（任务书 §5.1）。
//
// 对外接口：
//
//	POST /v1/messages   Anthropic 兼容入口（流式与非流式合一）
//	GET  /v1/models     模型列表
//	GET  /healthz       健康检查
//	GET  /readyz        就绪检查
//
// 入口协议恒为 Anthropic 格式，不感知下游通道类型；通道差异收敛在 Adapter 内部。
package gateway

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/claude-gate/claude-gate/internal/domain"
	"github.com/claude-gate/claude-gate/internal/observ"
)

// Server 是网关 HTTP 服务。
type Server struct {
	logger *slog.Logger
	ready  func() bool // 就绪探针：依赖（PG/CH/Redis）是否就绪
}

// NewServer 构造网关服务。ready 为 nil 时默认始终就绪。
func NewServer(logger *slog.Logger, ready func() bool) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	if ready == nil {
		ready = func() bool { return true }
	}
	return &Server{logger: logger, ready: ready}
}

// Handler 返回组装好路由的 http.Handler。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /readyz", s.handleReadyz)
	mux.HandleFunc("POST /v1/messages", s.withTrace(s.handleMessages))
	mux.HandleFunc("GET /v1/models", s.withTrace(s.handleModels))
	return mux
}

// withTrace 是中间件：第一时间生成 trace_id 写入 context 与响应头（任务书 §5.1）。
func (s *Server) withTrace(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		traceID := observ.NewTraceID()
		ctx := observ.WithTraceID(r.Context(), traceID)
		w.Header().Set("X-Trace-Id", traceID)
		next(w, r.WithContext(ctx))
	}
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "time": time.Now().UTC()})
}

func (s *Server) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	if !s.ready() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "not_ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ready"})
}

// handleMessages 是代理主入口骨架。
//
// 完整链路（M1 主链路，需接入存储/上游后启用）：
//
//	认证(auth) → 加载分组(GroupResolver) → 改写(transformer) →
//	选 Adapter(registry) + 取 Key(keypool) → 调上游 → 缓存计费(cache) →
//	流式/非流式回写 → 明细落库(observ)
func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	traceID := observ.TraceID(r.Context())
	if r.Header.Get("Authorization") == "" {
		writeError(w, traceID, domain.ErrMissingAPIKey)
		return
	}
	// 主链路依赖存储/上游通道的真实接入，骨架阶段统一返回结构化错误，
	// 保证"任何阶段失败都能返回结构化错误 JSON"（任务书 §5.1 验收）。
	writeError(w, traceID, domain.ErrInternal.WithMessage(
		"代理主链路依赖 PG/ClickHouse/Redis/上游通道接入，请先完成 M1 存储与上游配置"))
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	// 模型列表来源于 model_mappings 表，骨架阶段返回空列表结构。
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": []any{}})
}

// writeJSON 写出 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError 把 error 转为结构化错误 JSON（Anthropic 风格 error 信封）。
func writeError(w http.ResponseWriter, traceID string, err error) {
	de, ok := domain.AsError(err)
	if !ok {
		de = domain.ErrInternal
	}
	writeJSON(w, de.HTTPStatus, map[string]any{
		"type": "error",
		"error": map[string]any{
			"type":    de.Code,
			"message": de.UserMessage,
		},
		"trace_id": traceID,
	})
}
