package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nvs-server/config"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ─────────────────────────────────────────────────────────────
// 通用文件上传（博客图片/音视频等，仿 CSDN 编辑器）
// POST /api/upload
//   multipart field: file
//   允许：image/*（png/jpg/jpeg/gif/webp/svg），video/mp4/webm，audio/mpeg|mp3|wav|ogg
//   大小：图片 ≤10MB，音视频 ≤50MB
//   返回：{ url: "/uploads/2026/08/xxxx.ext", size, name }
// ─────────────────────────────────────────────────────────────

const (
	maxImageSize  = 10 * 1024 * 1024
	maxMediaSize  = 50 * 1024 * 1024
	uploadMaxFile = 50 * 1024 * 1024
)

var allowedUploadExt = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".svg": true, ".bmp": true,
	".mp4": true, ".webm": true,
	".mp3": true, ".wav": true, ".ogg": true, ".m4a": true,
}

func isImageExt(ext string) bool {
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".bmp":
		return true
	}
	return false
}

func UploadFile(c *gin.Context) {
	// 登录即可上传（作者/读者均可，配额在存储层控制）
	if currentUserID(c) == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}

	var file multipart.File
	var header *multipart.FileHeader
	var err error
	if file, header, err = c.Request.FormFile("file"); err != nil {
		utils.BadRequest(c, "请选择要上传的文件")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !allowedUploadExt[ext] {
		utils.BadRequest(c, "不支持的文件类型，仅支持图片(png/jpg/gif/webp/svg) / 视频(mp4/webm) / 音频(mp3/wav/ogg)")
		return
	}
	if header.Size > uploadMaxFile {
		utils.BadRequest(c, "文件不能超过 50MB")
		return
	}
	if isImageExt(ext) && header.Size > maxImageSize {
		utils.BadRequest(c, "图片不能超过 10MB")
		return
	}
	if header.Size > maxMediaSize {
		utils.BadRequest(c, "音视频文件不能超过 50MB")
		return
	}

	// 保存到 data/uploads/yyyy/mm/随机名.ext
	randBytes := make([]byte, 8)
	if _, err := rand.Read(randBytes); err != nil {
		utils.InternalError(c, "生成文件名失败")
		return
	}
	name := hex.EncodeToString(randBytes) + ext
	relDir := time.Now().Format("2006/01")
	absDir := filepath.Join(config.UploadDir, relDir)
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		utils.InternalError(c, "创建上传目录失败")
		return
	}
	absPath := filepath.Join(absDir, name)
	dst, err := os.Create(absPath)
	if err != nil {
		utils.InternalError(c, "保存文件失败")
		return
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		_ = os.Remove(absPath)
		utils.InternalError(c, "写入文件失败")
		return
	}

	url := fmt.Sprintf("/uploads/%s/%s", relDir, name)
	utils.Success(c, gin.H{
		"url":  url,
		"size": header.Size,
		"name": header.Filename,
		"ext":  ext,
	})
}
