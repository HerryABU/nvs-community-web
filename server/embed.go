//go:build !nodist

package main

import (
	"embed"
	"io/fs"
)

// 生产构建：把 server/dist 下的前端产物整体嵌进二进制（单文件部署的关键）。
//
// //go:embed 是**编译期**打包，所以必须先 `npm run build` 并把产物同步到 server/dist，
// 否则编译直接报 `pattern all:dist: no matching files found`（server/dist 不入库）。
//
// 若只是要编译 / vet / 测试业务代码（CI 的后端 job、新克隆跑 go test），
// 用 `-tags nodist` 走 embed_nodist.go —— 那条路不依赖前端产物是否存在。
//
//go:embed all:dist
var distRoot embed.FS

// nodistBuild 标记当前二进制是否「不含真实前端产物」，用于启动日志提醒。
const nodistBuild = false

// getFrontendFS 返回可直接喂给 http.FileServer 的前端资源 FS。
func getFrontendFS() (fs.FS, error) {
	return fs.Sub(distRoot, "dist")
}
