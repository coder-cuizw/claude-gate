-- claude-gate 请求明细 + 聚合（任务书 §4.2）
CREATE TABLE IF NOT EXISTS request_logs (
    trace_id String,
    api_key_id UInt64,
    group_id UInt64,
    channel_id UInt64,
    channel_type LowCardinality(String),   -- kiro / official / relay / custom
    upstream_key_id UInt64,
    model String,

    -- 时间
    request_at DateTime64(3),
    first_token_at Nullable(DateTime64(3)),
    completed_at DateTime64(3),
    ttft_ms UInt32,
    duration_ms UInt32,

    -- 状态
    status_code UInt16,
    is_streaming UInt8,
    is_success UInt8,
    error_type LowCardinality(String),
    error_message String,

    -- Usage（返回给客户的计费值）
    input_tokens UInt32,
    output_tokens UInt32,
    cache_creation_tokens UInt32,
    cache_read_tokens UInt32,

    -- Usage（上游真实值，用于成本核算与审计）
    upstream_input_tokens UInt32,
    upstream_output_tokens UInt32,
    upstream_cache_creation_tokens UInt32,
    upstream_cache_read_tokens UInt32,

    -- 复现凭证
    request_body_s3_key String,
    response_body_s3_key String,

    created_date Date MATERIALIZED toDate(request_at)
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(created_date)
ORDER BY (request_at, group_id, trace_id)
TTL created_date + INTERVAL 90 DAY;

-- 1 分钟聚合物化视图（聚合查询走此视图，不直接扫 request_logs）
CREATE MATERIALIZED VIEW IF NOT EXISTS request_metrics_1m
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(minute)
ORDER BY (minute, group_id, channel_id, model)
AS SELECT
    toStartOfMinute(request_at) AS minute,
    group_id, channel_id, model,
    count() AS request_count,
    sumIf(1, is_success = 1) AS success_count,
    sumIf(1, is_success = 0) AS error_count,
    avgState(ttft_ms) AS ttft_state,
    quantilesState(0.5, 0.95, 0.99)(ttft_ms) AS ttft_quantiles,
    max(ttft_ms) AS max_ttft_ms,
    sum(input_tokens) AS total_input_tokens,
    sum(output_tokens) AS total_output_tokens,
    sum(cache_creation_tokens) AS total_cache_creation_tokens,
    sum(cache_read_tokens) AS total_cache_read_tokens
FROM request_logs
GROUP BY minute, group_id, channel_id, model;
