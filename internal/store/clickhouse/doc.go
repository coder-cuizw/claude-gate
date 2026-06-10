// Package clickhouse 实现请求明细写入与聚合查询。
//
// 明细批量写入（每 1s 或每 1000 条 flush，任务书 §2.1）；
// 聚合查询走物化视图 request_metrics_1m，不直接扫 request_logs（任务书 §5.7）。
// 表结构见 migrations/clickhouse。
package clickhouse
