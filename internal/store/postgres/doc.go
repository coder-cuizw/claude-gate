// Package postgres 实现 store.ConfigStore：基于 PostgreSQL 的配置数据读写。
//
// 表结构见 migrations/postgres。实现待接入 pgx 连接池后补全；
// 连接池上限按核数与内存核算后写入配置（任务书 §2.1）。
package postgres
