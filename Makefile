# NVS 前后端构建入口（双轨：默认国际轨 / 国密轨 gm）
# 用法：
#   make build            默认国际轨（零国密依赖）
#   make build-gm         纯 Go 国密轨（-tags gm，依赖 go.gm.mod）
#   make build-gitvcs     版本控制启用真实 git 仓库后端（-tags gitvcs，运行时 NVS_VCS_BACKEND=git）
#   make test / test-gm   对应轨的测试
#   make gm-audit         输出国密自检报告（Markdown）
#   make credential-age   凭据年龄报表
#   make scan-secrets     扫描前端产物中的疑似密钥
#   make dist             构建前端并同步到 server/dist（embed 源）
SHELL := /bin/bash
GO ?= go
# 只认 `var AppVersion` 那一行：version.go 的注释里出现过的版本号会盖掉真实值（曾经取到过 2.3.1）
VERSION := $(shell grep -E '^var AppVersion' server/version/version.go | grep -oE '[0-9]+\.[0-9]+\.[0-9]+')

.PHONY: build build-gitvcs build-gm build-gm-cgo test test-gitvcs test-gm dist dist-check hooks check check-all gm-audit credential-age scan-secrets tidy-gm clean version

build:
	cd server && CGO_ENABLED=0 $(GO) build -trimpath -ldflags "-s -w" -o ../nvs-server.exe .

# 章节版本控制走真实 git 仓库（go-git）：构建带 gitvcs 标签，运行时设 NVS_VCS_BACKEND=git
build-gitvcs:
	cd server && CGO_ENABLED=0 $(GO) build -tags gitvcs -trimpath -ldflags "-s -w" -o ../nvs-server-gitvcs.exe .

test-gitvcs:
	cd server && $(GO) test -tags gitvcs ./... -count=1

build-gm:
	@test -f go.gm.mod || (echo "缺少 go.gm.mod"; exit 1)
	make tidy-gm
	cd server && $(GO) build -modfile=../go.gm.mod -tags gm -trimpath -o ../nvs-server-gm.exe .

build-gm-cgo:
	@test -f go.gm.mod || (echo "缺少 go.gm.mod"; exit 1)
	@echo "需要 TONGSUO_HOME 指向已编译（--enable-ntls）的 Tongsuo"
	make tidy-gm
	cd server && CGO_ENABLED=1 $(GO) build -modfile=../go.gm.mod -tags "gm tongsuo" -trimpath -o ../nvs-server-gm-cgo.exe .

test:
	cd server && $(GO) test ./... -count=1

test-gm:
	cd server && $(GO) test -modfile=../go.gm.mod -tags gm ./... -count=1

# 首次/依赖变更后执行：为 go.gm.mod 生成 go.gm.sum（默认 go.mod 不受影响）
tidy-gm:
	cd server && $(GO) mod tidy -modfile=../go.gm.mod

# 构建前端并同步到 embed 源（Windows 下用 robocopy；其它平台用 cp）
dist:
	cd web && node node_modules/vite/bin/vite.js build
	@if command -v robocopy >/dev/null 2>&1; then \
		robocopy dist server/dist /MIR /XF *.gz *.br >/dev/null || true; \
	else \
		rm -rf server/dist && cp -a dist server/dist; \
	fi
	@test -f server/dist/index.html && echo "dist ok: server/dist/index.html"

# 产物自检：embed 目录必须有 index.html 且资源数量合理（防"空 embed 打进 exe"）
dist-check:
	@test -f server/dist/index.html || (echo "server/dist/index.html 缺失：先 make dist"; exit 1)
	@n=$$(ls server/dist/assets 2>/dev/null | wc -l); echo "assets=$$n"; test "$$n" -gt 20 || (echo "资源数量异常（$$n）"; exit 1)

# 启用仓库内 hooks（新克隆后执行一次）
hooks:
	@bash .githooks/install.sh

# 本地等价 CI（推送前自己跑一遍，和 CI 的 backend + frontend 对齐）
check:
	cd server && $(GO) build ./... && $(GO) vet ./... && $(GO) test ./... -count=1
	cd web && npm run build

# check + embed 冒烟（最接近 CI 的完整本地校验）
check-all: check dist dist-check
	bash scripts/scan-frontend-secrets.sh server/dist
	@echo "本地完整校验通过（发布前建议再跑一次 make build）"

# 打 tag 发布由 GitHub Actions 完成（.github/workflows/release.yml），本地只校验版本
version:
	@echo "版本：$(VERSION)"
	@cd web && test "$$(node -p 'require("./package.json").version')" = "$(VERSION)" && echo "version.go 与 package.json 一致"

gm-audit:
	cd server && $(GO) run . gm-audit

credential-age:
	cd server && $(GO) run . credential-age

scan-secrets:
	bash scripts/scan-frontend-secrets.sh server/dist

clean:
	rm -f nvs-server.exe nvs-server-gm.exe nvs-server-gm-cgo.exe
