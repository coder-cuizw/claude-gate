package domain

import "time"

// ChannelType 表示上游通道类型，决定使用哪个 Adapter。
type ChannelType string

const (
	ChannelKiro     ChannelType = "kiro"     // 逆向私有通道，当前先做透传
	ChannelOfficial ChannelType = "official" // Claude 官方 API
	ChannelRelay    ChannelType = "relay"    // Anthropic 兼容第三方中转
	ChannelCustom   ChannelType = "custom"   // 自定义 / 本地 mock 合成通道
	// 注：Bedrock / Vertex 已按需移除；后续接入只需新增 Adapter 并在 registry 注册。
)

// KeyStatus 表示上游 Key 的可用状态。
type KeyStatus string

const (
	KeyActive   KeyStatus = "active"
	KeyDisabled KeyStatus = "disabled"
)

// User 管理后台用户。
type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"` // admin / viewer
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UpstreamChannel 上游通道配置。Config 按 Type 不同而结构不同。
type UpstreamChannel struct {
	ID        int64          `json:"id"`
	Name      string         `json:"name"`
	Type      ChannelType    `json:"type"`
	BaseURL   string         `json:"base_url"`
	Config    map[string]any `json:"config"`
	Enabled   bool           `json:"enabled"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// UpstreamKey 上游凭证。CredentialEncrypted 为 AES 加密存储，内容随通道而异。
//
// 定位：claude-gate 只做中间层，不做号池管理。上游 Key 由使用方在外部开好后
// 直接填入；网关仅在同通道多把 Key 间做简单轮询转发，不维护刷新令牌、不做
// 冷却/健康调度等账号池生命周期管理。
type UpstreamKey struct {
	ID                  int64      `json:"id"`
	ChannelID           int64      `json:"channel_id"`
	Name                string     `json:"name"`
	CredentialEncrypted string     `json:"-"`
	Status              KeyStatus  `json:"status"` // active / disabled
	LastError           string     `json:"last_error,omitempty"`
	LastUsedAt          *time.Time `json:"last_used_at,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
}

// CacheStrategyConfig 缓存计费策略配置，对应 groups.cache_strategy JSONB。
type CacheStrategyConfig struct {
	Type   string         `json:"type"` // passthrough / percentage / fixed / formula
	Params map[string]any `json:"params,omitempty"`
}

// TransformerConfig 单个 Transformer 在分组内的配置。
type TransformerConfig struct {
	Name    string         `json:"name"`
	Enabled bool           `json:"enabled"`
	Params  map[string]any `json:"params,omitempty"`
}

// RateLimitConfig 分组限流配置。
type RateLimitConfig struct {
	RPM int `json:"rpm"`
	TPM int `json:"tpm"`
}

// RetryConfig 分组重试配置。
type RetryConfig struct {
	MaxRetries int `json:"max_retries"`
	BackoffMs  int `json:"backoff_ms"`
}

// Group 分组，是 claude-gate 最核心的配置单元。
type Group struct {
	ID                int64               `json:"id"`
	Name              string              `json:"name"`
	Description       string              `json:"description"`
	ChannelID         int64               `json:"channel_id"`
	CacheStrategy     CacheStrategyConfig `json:"cache_strategy"`
	TransformerConfig []TransformerConfig `json:"transformer_config"`
	RateLimit         RateLimitConfig     `json:"rate_limit_config"`
	Retry             RetryConfig         `json:"retry_config"`
	Enabled           bool                `json:"enabled"`
	CreatedAt         time.Time           `json:"created_at"`
	UpdatedAt         time.Time           `json:"updated_at"`
}

// APIKey 客户 API Key。
//
// 存两份密文：
//   - KeyHash：不可逆 hash，用于热路径校验（见 auth.VerifySecret）；
//   - KeyEncrypted：AES-256-GCM 可逆密文，仅供管理后台"重复查看"明文时解密
//     （中转站运营常需事后重看/分发 Key，是面向该产品的有意取舍，见 internal/auth/crypto.go）。
type APIKey struct {
	ID           int64      `json:"id"`
	KeyPrefix    string     `json:"key_prefix"`
	KeyHash      string     `json:"-"`
	KeyEncrypted string     `json:"-"`
	Name         string     `json:"name"`
	GroupID      int64      `json:"group_id"`
	Enabled      bool       `json:"enabled"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// ModelMapping 模型别名映射。
type ModelMapping struct {
	ID            int64  `json:"id"`
	ChannelID     int64  `json:"channel_id"`
	ClientModel   string `json:"client_model"`
	UpstreamModel string `json:"upstream_model"`
}
