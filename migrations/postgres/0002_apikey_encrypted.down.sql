-- 回滚：移除客户 API Key 的可逆加密明文列
ALTER TABLE api_keys DROP COLUMN IF EXISTS key_encrypted;
