// Package auth 实现客户 API Key 的解析、校验，以及 API Key → Group 的解析（任务书 §5.2）。
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/claude-gate/claude-gate/internal/domain"
)

// keyPrefixLen 是用于库内索引查询的前缀长度。
const keyPrefixLen = 8

// KeyFormat：cg-<prefix>-<secret>，例如 cg-AB12cd34-3f9a...（hex）。
const keyScheme = "cg"

// GeneratedKey 是新建 API Key 的返回结果。明文 Plaintext 只在创建时出现一次。
type GeneratedKey struct {
	Plaintext string // 完整明文，仅返回一次
	Prefix    string // 入库索引用的前缀
	Hash      string // 入库存储的 hash
}

// GenerateKey 生成一把新的 API Key（前缀 + 随机密文），并给出入库所需的前缀与 hash。
func GenerateKey() (GeneratedKey, error) {
	prefix, err := randomHex(keyPrefixLen / 2) // 每字节 2 个 hex 字符
	if err != nil {
		return GeneratedKey{}, err
	}
	secret, err := randomHex(24)
	if err != nil {
		return GeneratedKey{}, err
	}
	plaintext := fmt.Sprintf("%s-%s-%s", keyScheme, prefix, secret)
	return GeneratedKey{
		Plaintext: plaintext,
		Prefix:    prefix,
		Hash:      HashSecret(secret),
	}, nil
}

// ParseAPIKey 从完整明文中解析出前缀与密文，并做格式校验。
func ParseAPIKey(raw string) (prefix, secret string, err error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "Bearer ")
	parts := strings.Split(raw, "-")
	if len(parts) != 3 || parts[0] != keyScheme {
		return "", "", domain.ErrInvalidAPIKey
	}
	if len(parts[1]) != keyPrefixLen || parts[2] == "" {
		return "", "", domain.ErrInvalidAPIKey
	}
	return parts[1], parts[2], nil
}

// HashSecret 计算密文的存储 hash。
//
// 说明：网关在 10k rpm 热路径上每请求都要校验 Key，因此这里用 SHA-256
// （配合 Redis 60s 缓存命中，避免每请求计算）。若安全要求更高，可替换为
// bcrypt/argon2——接口不变，只需改这里与 VerifySecret。
func HashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// VerifySecret 以恒定时间比较密文与存储 hash，防时序侧信道。
func VerifySecret(secret, hash string) bool {
	got := HashSecret(secret)
	return subtle.ConstantTimeCompare([]byte(got), []byte(hash)) == 1
}

// randomHex 返回 n 字节随机数的 hex 编码（长度 2n）。
func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("生成随机数失败: %w", err)
	}
	return hex.EncodeToString(b), nil
}
