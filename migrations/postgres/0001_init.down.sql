-- 回滚 claude-gate 配置数据初始化
DROP TABLE IF EXISTS model_mappings;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS groups;
DROP TABLE IF EXISTS upstream_keys;
DROP TABLE IF EXISTS upstream_channels;
DROP TABLE IF EXISTS users;
