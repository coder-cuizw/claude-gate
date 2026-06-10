# claude-gate 常用开发命令
.PHONY: help build test test-cover vet run web-dev web-build shots up down migrate fmt tidy

help: ## 显示帮助
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

build: ## 编译网关与迁移工具
	go build -o bin/gateway ./cmd/gateway
	go build -o bin/migrate ./cmd/migrate

test: ## 运行全部 Go 测试
	go test ./...

test-cover: ## 运行测试并输出覆盖率
	go test ./... -cover

vet: ## 静态检查
	go vet ./...

fmt: ## 格式化
	gofmt -w ./internal ./cmd

tidy: ## 整理依赖
	go mod tidy

run: build ## 本地启动网关
	./bin/gateway

migrate: ## 列出迁移脚本
	go run ./cmd/migrate

web-dev: ## 前端开发模式
	cd web && pnpm install && pnpm dev

web-build: ## 前端构建
	cd web && pnpm install && pnpm build

shots: ## 生成各页面明暗截图
	cd web && pnpm preview & sleep 4; cd web && PLAYWRIGHT_BROWSERS_PATH=/opt/pw-browsers node scripts/screenshots.mjs

up: ## docker-compose 拉起全套依赖
	cd deploy && docker compose up -d

down: ## 停止并清理
	cd deploy && docker compose down
