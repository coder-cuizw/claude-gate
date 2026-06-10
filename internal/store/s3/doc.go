// Package s3 实现 observ.BodyStore：请求/响应 body 落 S3/MinIO。
//
// 路径规则 requests/{YYYY-MM-DD}/{trace_id}/{request|response|meta}.json（任务书 §5.6）；
// 写入走独立 goroutine pool + 队列，失败重试 3 次后丢弃并打 metric；配 TTL/生命周期清理。
package s3
