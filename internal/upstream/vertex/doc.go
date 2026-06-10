// Package vertex 实现 VertexAdapter（任务书 §5.5 路线图 M5）。
//
// 走 GCP 鉴权调 Vertex AI 上的 Claude（Anthropic schema），流式 → SSE。
// 优先用官方 SDK / Anthropic Vertex SDK，不手搓 OAuth 签名（任务书 §10）。
package vertex
