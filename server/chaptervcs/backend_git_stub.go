//go:build !gitvcs

package chaptervcs

// ==================== git 后端占位（未以 -tags gitvcs 构建） ====================
// 这样默认构建不依赖 go-git（二进制体积与运维成本可控），
// 需要真实 git 仓库时用：go build -tags gitvcs，并设置 NVS_VCS_BACKEND=git。

import "errors"

func newGitBackend() (Backend, error) {
	return nil, errors.New("当前构建未启用 git 后端：请用 go build -tags gitvcs 重新构建")
}
