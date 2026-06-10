// Package redis 实现 store.Cache：API Key → Group 解析缓存与限流。
//
// 键前缀统一 cg:（如 cg:apikey:{prefix}，任务书 §10）；
// API Key 缓存 60s，配置变更时主动失效（任务书 §5.2）。
package redis
