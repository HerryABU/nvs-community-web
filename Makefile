# NVS 前后端构建入口（双轨：默认国际轨 / 国密轨 gm）
# 用法：
#   make build            默认国际轨（零国密依赖）
#   make build-gm         纯 Go 国密轨（-tags gm，依赖 go.gm.mod）
#   make build-gm-cgo     CGO 国密轨（-tags "gm tongsuo"，需系统安装 Tongsuo --enable-ntls）
#   make test / test-gm   对应轨的测试
#   make gm-audit         输出国密自检报告（Markdown）
#   make credential-age   凭据年龄报表
#   make scan-secrets     扫描前端产物中的疑似密钥
#   make dist             构建前端并同步到 server/dist（embed 源）
SHELL := /bin/bash
GO ?= go
VERSION := $(shell grep -oE '[0-9]+\.[0-9]+\.[0-9]+' server/version/version.go | head -1)

.PHONY: build build-gm build-gm-cgo test test-gm dist gm-audit credential-age scan-secrets tidy-gm clean

build:
	cd server && CGO_ENABLED=0 $(GO) build -trimpath -ldflags "-s -w" -o ../nvs-server.exe .

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

dist:
	cd web && node node_modules/vite/bin/vite.js build
	robocopy dist server/dist /MIR /XF *.gz *.br || true

gm-audit:
	cd server && $(GO) run . gm-audit

credential-age:
	cd server && $(GO) run . credential-age

scan-secrets:
	bash scripts/scan-frontend-secrets.sh server/dist

clean:
	rm -f nvs-server.exe nvs-server-gm.exe nvs-server-gm-cgo.exe
