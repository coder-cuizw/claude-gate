// Package domain 定义 claude-gate 的领域模型与错误类型。
//
// 所有跨模块共享的核心结构体（Usage、Group、Channel 等）以及统一错误类型
// 都集中在本包，避免各模块之间产生循环依赖。
package domain

import (
	"errors"
	"fmt"
	"net/http"
)

// Error 是 claude-gate 统一的领域错误类型。
//
// 设计目标：既能向客户返回结构化、稳定的错误码与 HTTP 状态，
// 又能在内部保留完整的错误链路用于排查（Internal 字段不会暴露给客户）。
type Error struct {
	// Code 是稳定的机器可读错误码，例如 "invalid_api_key"。
	Code string
	// HTTPStatus 是返回给客户端的 HTTP 状态码。
	HTTPStatus int
	// UserMessage 是可安全暴露给客户的提示信息。
	UserMessage string
	// Internal 是内部原始错误，仅用于日志，绝不返回给客户。
	Internal error
}

// Error 实现 error 接口，输出包含内部链路，便于日志排查。
func (e *Error) Error() string {
	if e.Internal != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.UserMessage, e.Internal)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.UserMessage)
}

// Unwrap 暴露内部错误，支持 errors.Is / errors.As 链路追踪。
func (e *Error) Unwrap() error { return e.Internal }

// Wrap 在保留错误码与状态的前提下，挂接一个新的内部原因。
func (e *Error) Wrap(cause error) *Error {
	clone := *e
	clone.Internal = cause
	return &clone
}

// WithMessage 返回一个替换了用户提示信息的副本。
func (e *Error) WithMessage(msg string) *Error {
	clone := *e
	clone.UserMessage = msg
	return &clone
}

// AsError 尝试把任意 error 还原为 *domain.Error，便于上层统一处理。
func AsError(err error) (*Error, bool) {
	var de *Error
	if errors.As(err, &de) {
		return de, true
	}
	return nil, false
}

// 预定义的常见错误。具体业务可在此基础上 Wrap 内部原因。
var (
	// 认证 / 鉴权相关
	ErrMissingAPIKey  = &Error{Code: "missing_api_key", HTTPStatus: http.StatusUnauthorized, UserMessage: "缺少 Authorization 凭证"}
	ErrInvalidAPIKey  = &Error{Code: "invalid_api_key", HTTPStatus: http.StatusUnauthorized, UserMessage: "API Key 不存在或无效"}
	ErrAPIKeyExpired  = &Error{Code: "api_key_expired", HTTPStatus: http.StatusUnauthorized, UserMessage: "API Key 已过期"}
	ErrAPIKeyDisabled = &Error{Code: "api_key_disabled", HTTPStatus: http.StatusForbidden, UserMessage: "API Key 已被禁用"}
	ErrGroupDisabled  = &Error{Code: "group_disabled", HTTPStatus: http.StatusForbidden, UserMessage: "分组已被禁用"}

	// 限流 / 背压相关
	ErrRateLimited    = &Error{Code: "rate_limited", HTTPStatus: http.StatusTooManyRequests, UserMessage: "请求超过限流阈值"}
	ErrTooManyInFlight = &Error{Code: "too_many_in_flight", HTTPStatus: http.StatusTooManyRequests, UserMessage: "并发请求数超过上限，请稍后重试"}

	// 上游 / 通道相关
	ErrNoUpstreamKey   = &Error{Code: "no_upstream_key", HTTPStatus: http.StatusServiceUnavailable, UserMessage: "没有可用的上游凭证"}
	ErrUpstreamFailure = &Error{Code: "upstream_failure", HTTPStatus: http.StatusBadGateway, UserMessage: "上游通道请求失败"}
	ErrAdapterNotFound = &Error{Code: "adapter_not_found", HTTPStatus: http.StatusBadGateway, UserMessage: "未找到对应的上游适配器"}

	// 请求 / 协议相关
	ErrInvalidRequest = &Error{Code: "invalid_request", HTTPStatus: http.StatusBadRequest, UserMessage: "请求体格式不合法"}
	ErrTimeout        = &Error{Code: "timeout", HTTPStatus: http.StatusGatewayTimeout, UserMessage: "请求超时"}

	// 内部错误
	ErrInternal = &Error{Code: "internal_error", HTTPStatus: http.StatusInternalServerError, UserMessage: "网关内部错误"}
)
