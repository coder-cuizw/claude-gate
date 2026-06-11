// Command migrate 执行数据库迁移（任务书 §8）。
//
// 按文件名顺序应用 migrations/{postgres,clickhouse}/*.up.sql，并用 schema_migrations
// 表记录已应用版本，重复执行幂等。DSN 从 flag 或环境变量读取。
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/jackc/pgx/v5"
)

func main() {
	dir := flag.String("dir", "migrations", "迁移脚本根目录")
	only := flag.String("only", "all", "执行范围：all / pg / ch")
	pgDSN := flag.String("pg-dsn", os.Getenv("CG_POSTGRES_DSN"), "PostgreSQL DSN")
	chDSN := flag.String("ch-dsn", os.Getenv("CG_CLICKHOUSE_DSN"), "ClickHouse DSN")
	flag.Parse()

	ctx := context.Background()
	if *only == "all" || *only == "pg" {
		if *pgDSN == "" {
			log.Fatal("缺少 PostgreSQL DSN（--pg-dsn 或 CG_POSTGRES_DSN）")
		}
		if err := migratePG(ctx, *pgDSN, filepath.Join(*dir, "postgres")); err != nil {
			log.Fatalf("PostgreSQL 迁移失败: %v", err)
		}
	}
	if *only == "all" || *only == "ch" {
		if *chDSN == "" {
			log.Fatal("缺少 ClickHouse DSN（--ch-dsn 或 CG_CLICKHOUSE_DSN）")
		}
		if err := migrateCH(ctx, *chDSN, filepath.Join(*dir, "clickhouse")); err != nil {
			log.Fatalf("ClickHouse 迁移失败: %v", err)
		}
	}
	log.Println("迁移完成 ✓")
}

func upFiles(dir string) ([]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// migratePG 用 simple protocol 整文件执行（支持多语句），版本记录幂等。
func migratePG(ctx context.Context, dsn, dir string) error {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return err
	}
	cfg.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations(version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ DEFAULT NOW())`); err != nil {
		return err
	}
	files, err := upFiles(dir)
	if err != nil {
		return err
	}
	for _, f := range files {
		ver := filepath.Base(f)
		var exists bool
		_ = conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, ver).Scan(&exists)
		if exists {
			log.Printf("[pg] 跳过已应用 %s", ver)
			continue
		}
		sql, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if _, err := conn.Exec(ctx, string(sql)); err != nil {
			return err
		}
		if _, err := conn.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, ver); err != nil {
			return err
		}
		log.Printf("[pg] 应用 %s ✓", ver)
	}
	return nil
}

// migrateCH 分语句执行（ClickHouse 一次一条），版本记录幂等。
func migrateCH(ctx context.Context, dsn, dir string) error {
	opt, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return err
	}
	conn, err := clickhouse.Open(opt)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations(version String) ENGINE=TinyLog`); err != nil {
		return err
	}
	files, err := upFiles(dir)
	if err != nil {
		return err
	}
	for _, f := range files {
		ver := filepath.Base(f)
		var cnt uint64
		_ = conn.QueryRow(ctx, `SELECT count() FROM schema_migrations WHERE version=?`, ver).Scan(&cnt)
		if cnt > 0 {
			log.Printf("[ch] 跳过已应用 %s", ver)
			continue
		}
		sql, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		for _, stmt := range splitStatements(string(sql)) {
			if err := conn.Exec(ctx, stmt); err != nil {
				return err
			}
		}
		if err := conn.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES(?)`, ver); err != nil {
			return err
		}
		log.Printf("[ch] 应用 %s ✓", ver)
	}
	return nil
}

// splitStatements 按分号切分 SQL 语句，跳过空白与纯注释段。
func splitStatements(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ";") {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		hasSQL := false
		for _, line := range strings.Split(trimmed, "\n") {
			l := strings.TrimSpace(line)
			if l != "" && !strings.HasPrefix(l, "--") {
				hasSQL = true
				break
			}
		}
		if hasSQL {
			out = append(out, trimmed)
		}
	}
	return out
}
