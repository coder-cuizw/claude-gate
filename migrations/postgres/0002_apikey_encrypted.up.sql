-- 为客户 API Key 增加可逆加密的明文存储，支持管理后台"重复查看"密钥。
-- key_hash 仍用于热路径校验；key_encrypted 为 AES-256-GCM 密文，仅管理后台解密展示。
ALTER TABLE api_keys ADD COLUMN key_encrypted TEXT NOT NULL DEFAULT '';
