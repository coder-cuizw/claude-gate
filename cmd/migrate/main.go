// Command migrate 是数据库迁移工具（任务书 §8）。
//
// 它列出并校验 migrations/ 下的 SQL 迁移脚本。生产中通过 golang-migrate/migrate
// 对 PostgreSQL 与 ClickHouse 分别执行；此处提供轻量的列出/校验能力，
// 真正执行迁移由 Makefile 的 migrate 目标驱动（见 Makefile）。
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func main() {
	dir := flag.String("dir", "migrations", "迁移脚本根目录")
	flag.Parse()

	for _, sub := range []string{"postgres", "clickhouse"} {
		path := filepath.Join(*dir, sub)
		files, err := filepath.Glob(filepath.Join(path, "*.sql"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "扫描 %s 失败: %v\n", path, err)
			os.Exit(1)
		}
		sort.Strings(files)
		fmt.Printf("[%s] 共 %d 个迁移脚本:\n", sub, len(files))
		for _, f := range files {
			fmt.Printf("  - %s\n", filepath.Base(f))
		}
	}
}
