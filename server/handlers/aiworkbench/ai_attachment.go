package aiworkbench

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
	"nvs-server/doctext"
	"nvs-server/handlers/hctx"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	aim "nvs-server/models/ai"
)

// ─────────────────────────────────────────────────────────────
// AI 工作台附件（图片 / 文档）
// POST /api/ai/attachment   multipart field: file
//
// 设计要点：**图片原样发，文档优先抽正文**。
// 图片与文档先存到 uploads/ai/ 下，发送对话时由后端读回：
//   - 图片 → image_url（data:image/...;base64），多模态直送
//   - 文档 → 优先在平台侧抽取正文，作为「参考文本」注入提示词
//     （任何上游都能用；原样发 file part 只有 Claude 读得懂，
//     DeepSeek/Qwen/GLM 等 OpenAI 兼容上游会 400）
//     抽不出正文时（扫描件 PDF / 旧版 .doc）才回退为原样 file part。
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
	if hctx.CurrentUserID(c) == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	if !aim.IsAiEnabled() {
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

// buildAttachmentParts 把附件引用转成上游 content parts。
//
// 图片 → image_url（data:image/...;base64），多模态直送。
// 文档 → **优先在平台侧抽取正文**，作为「参考文本」注入提示词：
//
//	只有 Anthropic（Claude document block）读得懂 OpenAI 的 file content part，
//	DeepSeek / Qwen / GLM / Moonshot 等 OpenAI 兼容上游会因不认识的 content
//	type 直接 400。所以文档先在平台侧解析成文本，任何上游都能用。
//	抽不出正文（扫描件 PDF / 旧版 .doc / 加密文档）才回退为原样 file part，
//	保留 Claude 那条路。
func buildAttachmentParts(items []AiChatAttachment) ([]aiclient.ImageURLPart, []aiclient.FilePart, []string, error) {
	images := make([]aiclient.ImageURLPart, 0, len(items))
	files := make([]aiclient.FilePart, 0, len(items))
	docTexts := make([]string, 0, len(items))
	for _, it := range items {
		data, mime, err := aiAttachmentBytes(it.URL)
		if err != nil {
			return nil, nil, nil, err
		}
		if it.Kind == "image" {
			images = append(images, aiclient.ImageURLPart{URL: dataURLFrom(mime, data)})
			continue
		}
		name := strings.TrimSpace(it.Name)
		if name == "" {
			name = filepath.Base(it.URL)
		}
		// 文档：先试着抽正文，抽不出来才原样发文件
		if res, xerr := doctext.Extract(data, filepath.Ext(name)); xerr == nil && strings.TrimSpace(res.Text) != "" {
			docTexts = append(docTexts, formatDocText(name, res))
			continue
		}
		files = append(files, aiclient.FilePart{Filename: name, FileData: dataURLFrom(mime, data)})
	}
	return images, files, docTexts, nil
}

// formatDocText 把抽取出的正文包成明确的「参考文本」区块，
// 让模型知道这是参考资料而不是要续写的正文。
func formatDocText(name string, res doctext.Result) string {
	var sb strings.Builder
	sb.WriteString("【附件正文：" + name + "】\n")
	sb.WriteString(res.Text)
	sb.WriteString("\n【附件正文结束：" + name + "】")
	return sb.String()
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

// aiAttachmentBytes 把已上传的附件读回来（原始字节 + MIME）。
// 只允许读 /uploads/ai/ 下的文件，避免被构造路径读到别的东西。
func aiAttachmentBytes(url string) ([]byte, string, error) {
	const prefix = "/uploads/ai/"
	if !strings.HasPrefix(url, prefix) {
		return nil, "", fmt.Errorf("非法的附件地址")
	}
	// 用 Clean("/"+rel) 把任何 ../ 抹平（锚定在 / 下，出不去），同时保留 yyyy/mm 子目录
	rel := filepath.Clean("/" + strings.TrimPrefix(url, prefix))
	abs := filepath.Join(config.UploadDir, "ai", rel)

	ext := strings.ToLower(filepath.Ext(abs))
	mimeType, ok := aiAttachmentMIME[ext]
	if !ok {
		return nil, "", fmt.Errorf("不支持的附件类型")
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, "", fmt.Errorf("附件已失效，请重新上传")
	}
	if len(data) > aiFileMaxSize {
		return nil, "", fmt.Errorf("附件过大")
	}
	return data, mimeType, nil
}

// dataURLFrom 字节 + MIME → data URL（供上游 content part 内联）。
func dataURLFrom(mime string, data []byte) string {
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}
