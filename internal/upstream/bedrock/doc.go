// Package bedrock 实现 BedrockAdapter（任务书 §5.5 路线图 M5）。
//
// 走 aws-sdk-go-v2 调 Bedrock Runtime 上的 Claude（Anthropic schema），
// event stream → SSE。优先用官方 SDK，不手搓 SigV4 签名（任务书 §10）。
package bedrock
