// Package admin 实现管理后台 API（任务书 §6 / §5.7）。
//
// 鉴权：JWT，登录接口签发，所有 /api/admin/* 校验。
// 资源 CRUD：users / channels / upstream-keys / groups / api-keys / model-mappings；
// 统计：overview / timeseries / errors / by-channel；
// traces 列表与详情；Replay 复现接口（支持跨通道复现）。
// 实现待接入存储后补全。
package admin
