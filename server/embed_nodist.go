//go:build nodist

package main

import (
	"embed"
	"io/fs"
)

// 「无前端产物」构建（`-tags nodist`）：
//
// 为什么需要它：server/dist 是构建产物、不入库（.gitignore）。而 //go:embed all:dist
// 要求目录里至少有一个文件，否则编译期就报 `pattern all:dist: no matching files found`。
// 结果就是**全新 clone 里连 `go build ./...` / `go test ./...` 都跑不起来**，
// CI 的后端 job（只做编译与测试，不产出发布物）也会在这里挂掉。
//
// 这条标签轨只用于「编译 / vet / 测试」：嵌入一个提示页，让 fs.Sub 能成功、
// 页面打开时明确告诉你「这是占位页，请先构建前端」，而不是白屏或 500。
//
// 发布物（make build / release.yml / CI 的 embed-smoke）**必须不带** nodist 标签，
// 且要先 npm run build + 同步 server/dist，才能把真实前端打进单文件。
//
//go:embed nodist_placeholder/index.html
var distRoot embed.FS

// nodistBuild 让 main 在启动时打一条醒目提示（避免误把这个二进制当发布物部署）。
const nodistBuild = true

func getFrontendFS() (fs.FS, error) {
	return fs.Sub(distRoot, "nodist_placeholder")
}
