package admin

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// 本文件实现最小化 HS256 JWT 的签发与校验，避免引入外部依赖。
// 管理后台登录后签发，所有 /api/admin/* 接口校验（任务书 §6）。

// Claims 是 JWT 载荷。
type Claims struct {
	Subject string `json:"sub"`
	Role    string `json:"role"`
	Expires int64  `json:"exp"`
}

var errInvalidToken = errors.New("无效的 token")

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func sign(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return b64(mac.Sum(nil))
}

// SignJWT 签发一个 HS256 token。
func SignJWT(c Claims, secret string) (string, error) {
	header := b64([]byte(`{"alg":"HS256","typ":"JWT"}`))
	body, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	payload := header + "." + b64(body)
	return payload + "." + sign(payload, secret), nil
}

// VerifyJWT 校验签名与过期，返回载荷。
func VerifyJWT(token, secret string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errInvalidToken
	}
	payload := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(sign(payload, secret)), []byte(parts[2])) {
		return nil, errInvalidToken
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errInvalidToken
	}
	var c Claims
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, errInvalidToken
	}
	if c.Expires > 0 && time.Now().Unix() > c.Expires {
		return nil, errors.New("token 已过期")
	}
	return &c, nil
}
