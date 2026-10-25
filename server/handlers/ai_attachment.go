package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nvs-server/aiclient"
	"nvs-server/config"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ─────────────────────────────────────────────────────────────
// AI 工作台附件（图片 / 文档）
// POST /api/ai/attachment   multipart field: file
//
// 设计要点：**平台不做本地解析**。图片与文档原样保存，
// 发送对话时由后端读回并转成上游 API 的 content part：
//   - 图片 → image_url（data:image/...;base64）
//   - 文档 → file（data:application/pdf;base64，filename 原样带上）
// 由模型侧读取 PDF/docx，平台只负责搬运。
//
// 返回：{ kind: "image"|"file", name, url, mime, size }
// ─────────────────────────────────────────────────────────────

const (
	aiImageMaxSize = 10 << 20 // 10MB
	aiFileMaxSize  = 20 << 20 // 20MB
)

// aiAttachmentMIME 允许的附件类型 → MIME
// 不使用 mime.TypeByExtension：Windows 上依赖注册表，取不到就返回空，容易误判
var aiAttachmentMIME = map[string]string{
	// 图片
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg",
	".webp": "image/webp", ".gif": "image/gif",
	// 文档（原样给模型读，不由平台解析）
	".pdf":  "application/pdf",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".doc":  "application/msword",
	".txt":  "text/plain", ".md": "text/markdown", ".csv": "text/csv", ".json": "application/json",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
}

func isAiImageExt(ext string) bool {
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif":
		return true
	}
	return false
}

// AiAttachmentUpload 附件上传
func AiAttachmentUpload(c *gin.Context) {
	if currentUserID(c) == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	if !models.IsAiEnabled() {
		utils.BadRequest(c, "AI 功能未开启")
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		utils.BadRequest(c, "请选择要上传的文件")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	mimeType, ok := aiAttachmentMIME[ext]
	if !ok {
		utils.BadRequest(c, "暂不支持该文件类型（支持：图片 png/jpg/webp/gif；文档 pdf/docx/doc/txt/md/csv/json/xlsx/pptx）")
		return
	}
	sizeLimit := int64(aiFileMaxSize)
	if isAiImageExt(ext) {
		sizeLimit = aiImageMaxSize
	}
	if header.Size > sizeLimit {
		utils.BadRequest(c, fmt.Sprintf("文件过大（上限 %d MB）", sizeLimit>>20))
		return
	}

	randBytes := make([]byte, 8)
	if _, err := rand.Read(randBytes); err != nil {
		utils.InternalError(c, "生成文件名失败")
		return
	}
	relDir := filepath.ToSlash(filepath.Join("ai", time.Now().Format("2006/01")))
	absDir := filepath.Join(config.UploadDir, relDir)
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		utils.InternalError(c, "创建上传目录失败")
		return
	}
	name := hex.EncodeToString(randBytes) + ext
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

	kind := "file"
	if isAiImageExt(ext) {
		kind = "image"
	}
	utils.Success(c, gin.H{
		"kind": kind,
		"name": header.Filename,
		"url":  "/uploads/" + relDir + "/" + name,
		"mime": mimeType,
		"size": header.Size,
	})
}

// AiChatAttachment 对话附件引用：前端先 POST /api/ai/attachment 拿到 url，
// 发消息时把这些引用回传，后端读回文件原样内联给上游。
type AiChatAttachment struct {
	URL  string `json:"url"`
	Name string `json:"name"`
	Mime string `json:"mime"`
	Kind string `json:"kind"` // image / file
}

// buildAttachmentParts 把附件引用转成上游 content parts（读文件 → base64 data URL）。
// 返回图片与文档两组：图片走 image_url，文档走 file（原样让模型读）。
func buildAttachmentParts(items []AiChatAttachment) ([]aiclient.ImageURLPart, []aiclient.FilePart, error) {
	images := make([]aiclient.ImageURLPart, 0, len(items))
	files := make([]aiclient.FilePart, 0, len(items))
	for _, it := range items {
		dataURL, err := aiAttachmentDataURL(it.URL)
		if err != nil {
			return nil, nil, err
		}
		if it.Kind == "image" {
			images = append(images, aiclient.ImageURLPart{URL: dataURL})
			continue
		}
		name := strings.TrimSpace(it.Name)
		if name == "" {
			name = filepath.Base(it.URL)
		}
		files = append(files, aiclient.FilePart{Filename: name, FileData: dataURL})
	}
	return images, files, nil
}

// attachmentNames 附件名列表（给落库的可见文本用）
func attachmentNames(items []AiChatAttachment) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		n := strings.TrimSpace(it.Name)
		if n == "" {
			n = filepath.Base(it.URL)
		}
		out = append(out, n)
	}
	return out
}

// aiAttachmentDataURL 把已上传的附件读回来转成 data URL（供上游 content part 内联）。
// 只允许读 /uploads/ai/ 下的文件，避免被构造路径读到别的东西。
func aiAttachmentDataURL(url string) (string, error) {
	const prefix = "/uploads/ai/"
	if !strings.HasPrefix(url, prefix) {
		return "", fmt.Errorf("非法的附件地址")
	}
	// 用 Clean("/"+rel) 把任何 ../ 抹平（锚定在 / 下，出不去），同时保留 yyyy/mm 子目录
	rel := filepath.Clean("/" + strings.TrimPrefix(url, prefix))
	abs := filepath.Join(config.UploadDir, "ai", rel)

	ext := strings.ToLower(filepath.Ext(abs))
	mimeType, ok := aiAttachmentMIME[ext]
	if !ok {
		return "", fmt.Errorf("不支持的附件类型")
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", fmt.Errorf("附件已失效，请重新上传")
	}
	if len(data) > aiFileMaxSize {
		return "", fmt.Errorf("附件过大")
	}
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}
