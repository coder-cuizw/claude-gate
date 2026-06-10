-- claude-gate 配置数据初始化（任务书 §4.1）
-- 用户与权限
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(32) NOT NULL DEFAULT 'admin',  -- admin / viewer
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 上游通道
CREATE TABLE upstream_channels (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    type VARCHAR(32) NOT NULL,           -- kiro / official / bedrock / vertex / relay / custom
    base_url VARCHAR(512),
    config JSONB NOT NULL DEFAULT '{}',  -- 通道专属配置，按 type 不同而不同
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 上游 Key 池（凭证形态随通道类型而异）
CREATE TABLE upstream_keys (
    id BIGSERIAL PRIMARY KEY,
    channel_id BIGINT NOT NULL REFERENCES upstream_channels(id) ON DELETE CASCADE,
    name VARCHAR(128),
    credential_encrypted TEXT NOT NULL,   -- AES 加密存储；内容随通道而异
    status VARCHAR(32) DEFAULT 'active',  -- active / disabled / cooldown
    cooldown_until TIMESTAMPTZ,
    last_error TEXT,
    last_used_at TIMESTAMPTZ,
    refreshed_at TIMESTAMPTZ,             -- 刷新型凭证（如 Kiro）的最近刷新时间
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 分组（核心配置）
CREATE TABLE groups (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(128) NOT NULL UNIQUE,
    description TEXT,
    channel_id BIGINT NOT NULL REFERENCES upstream_channels(id),
    cache_strategy JSONB NOT NULL,        -- {type, params}
    transformer_config JSONB DEFAULT '[]',-- [{name, enabled, params}]
    rate_limit_config JSONB DEFAULT '{}', -- {rpm, tpm}
    retry_config JSONB DEFAULT '{}',
    enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- 客户 API Key
CREATE TABLE api_keys (
    id BIGSERIAL PRIMARY KEY,
    key_prefix VARCHAR(16) NOT NULL UNIQUE,
    key_hash VARCHAR(255) NOT NULL,
    name VARCHAR(128),
    group_id BIGINT NOT NULL REFERENCES groups(id),
    enabled BOOLEAN DEFAULT TRUE,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 模型别名映射
CREATE TABLE model_mappings (
    id BIGSERIAL PRIMARY KEY,
    channel_id BIGINT NOT NULL REFERENCES upstream_channels(id) ON DELETE CASCADE,
    client_model VARCHAR(128) NOT NULL,
    upstream_model VARCHAR(128) NOT NULL,
    UNIQUE(channel_id, client_model)
);

CREATE INDEX idx_api_keys_prefix ON api_keys(key_prefix);
CREATE INDEX idx_upstream_keys_channel_status ON upstream_keys(channel_id, status);
