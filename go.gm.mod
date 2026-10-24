// 国密轨独立模块清单：默认 go.mod 保持零国密依赖（N-01）。
// 使用：make build-gm / build-gm-cgo（内部走 -modfile=../go.gm.mod -tags gm）
// 首次使用先 make tidy-gm 生成 go.gm.sum。
module nvs-server

go 1.23

require (
	github.com/emmansun/gmsm v0.45.1
	github.com/aiyang-zh/zhenyi-base v1.1.5
)
