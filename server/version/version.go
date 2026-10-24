// Package version 集中管理 NVS 版本号，供 handlers / main / 前端统一引用，
// 避免各文件硬编码版本不一致。
// 可通过构建参数覆盖：go build -ldflags "-X nvs-server/version.AppVersion=v2.3.1"
package version

// AppVersion 当前版本号（不带 v 前缀，展示时自行拼接）。
// 保持与 README「版本更新记录」最新版本一致。
var AppVersion = "3.17.0"
