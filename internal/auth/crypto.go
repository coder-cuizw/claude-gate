package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
)

// 本文件提供 AES-256-GCM 的对称加解密，用于把"需要重复查看"的密文
// （客户 API Key 明文、上游凭证等）可逆地存库。
//
// 设计取舍：客户 API Key 默认只存 key_hash（不可逆），用于热路径校验；
// 但中转站运营场景常需运营者事后重新查看/分发 Key，因此额外存一份
// AES-256-GCM 加密的明文（key_encrypted），仅管理后台解密展示。
// 这是面向中转站产品的有意取舍，加密密钥经 CG_ENCRYPTION_KEY 配置（见 docs）。

// deriveKey 把任意口令派生为 32 字节 AES-256 密钥。
func deriveKey(secret string) [32]byte {
	return sha256.Sum256([]byte(secret))
}

// Encrypt 用口令对明文做 AES-256-GCM 加密，返回 base64(nonce|ciphertext)。
func Encrypt(plaintext, secret string) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("加密口令为空")
	}
	key := deriveKey(secret)
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	// Seal 把密文追加在 nonce 之后，便于解密时切分
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt 解密 Encrypt 的输出，口令不匹配或密文被篡改时返回错误。
func Decrypt(ciphertext, secret string) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("加密口令为空")
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("密文 base64 解码失败: %w", err)
	}
	key := deriveKey(secret)
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", fmt.Errorf("密文长度不足")
	}
	nonce, body := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return "", fmt.Errorf("解密失败（口令错误或密文被篡改）: %w", err)
	}
	return string(plain), nil
}
