// Package version 集中管理 NVS 版本号，供 handlers / main / 前端统一引用，
// 避免各文件硬编码版本不一致。
//
// 可通过构建参数覆盖（示例中的版本号请勿写成真实值，脚本靠“取第一处版本号”解析，
// 注释里出现具体版本会盖掉真实值）：
//
//	go build -ldflags "-X nvs-server/version.AppVersion=<版本>"
package version

// AppVersion 当前版本号（不带 v 前缀，展示时自行拼接）。
// 保持与 README「版本更新记录」最新版本一致。
//
// ⚠️ 机器解析约定：本行必须是文件里**唯一**的 `var AppVersion = "x.y.z"`，
// Makefile 与 GitHub Actions 都按 `^var AppVersion` 取版本，改这里就够。
var AppVersion = "4.3.2"
