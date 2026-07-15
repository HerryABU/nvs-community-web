package handlers

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nvs-server/config"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
	"github.com/microcosm-cc/bluemonday"
)

// POST /api/author/image — 作者上传图片（重新编码防图片马）
func UploadImage(c *gin.Context) {
	userID := c.GetUint("userID")

	file, header, err := c.Request.FormFile("image")
	if err != nil {
		utils.BadRequest(c, "请选择要上传的图片")
		return
	}
	defer file.Close()

	// 安全检查：限制文件大小（最大 10MB）
	const maxSize = 10 * 1024 * 1024
	if header.Size > maxSize {
		utils.BadRequest(c, "图片大小不能超过 10MB")
		return
	}

	// 安全检查：只允许图片类型
	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true}
	if !allowedExts[ext] {
		utils.BadRequest(c, "只支持 JPG/PNG/GIF 格式的图片")
		return
	}

	// 读取原始数据
	rawData, err := io.ReadAll(file)
	if err != nil {
		utils.InternalError(c, "读取图片失败")
		return
	}

	// 验证文件头魔术字节（magic bytes），防止伪造扩展名
	if !hasImageMagicBytes(rawData, ext) {
		log.Printf("[SECURITY] 文件头不匹配：声称扩展名 %s，但实际文件头为 %X", ext, rawData[:minInt(12, len(rawData))])
		utils.BadRequest(c, "文件内容与扩展名不匹配，可能是恶意文件")
		return
	}

	// 验证 MIME 类型（第二层防护）
	contentType := http.DetectContentType(rawData)
	if !strings.HasPrefix(contentType, "image/") {
		utils.BadRequest(c, "文件不是有效的图片格式")
		return
	}

	// 重新编码：解码 → 丢弃所有元数据和隐藏 payload → 重新编码
	img, format, err := image.Decode(bytes.NewReader(rawData))
	if err != nil {
		utils.BadRequest(c, "无法解码图片，文件可能已损坏")
		return
	}
	_ = format

	// 创建上传目录
	imgDir := filepath.Join(config.UploadDir, "images")
	os.MkdirAll(imgDir, 0755)

	// 先编码到内存 buffer，确定最终格式后再写文件（避免后缀不一致）
	var buf bytes.Buffer
	outExt := ".png"
	encodeErr := png.Encode(&buf, img)
	if encodeErr != nil {
		buf.Reset()
		encodeErr = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
		if encodeErr != nil {
			buf.Reset()
			encodeErr = gif.Encode(&buf, img, nil)
			if encodeErr != nil {
				utils.InternalError(c, "图片编码失败")
				return
			}
			outExt = ".gif"
		} else {
			outExt = ".jpg"
		}
	}

	filename := fmt.Sprintf("img_%d_%d%s", userID, time.Now().UnixNano(), outExt)
	imgPath := filepath.Join(imgDir, filename)

	if err := os.WriteFile(imgPath, buf.Bytes(), 0644); err != nil {
		utils.InternalError(c, "保存图片失败")
		return
	}

	imageURL := fmt.Sprintf("/uploads/images/%s", filename)
	utils.Success(c, gin.H{
		"url":     imageURL,
		"message": "图片上传成功",
	})
}

// hasImageMagicBytes 验证文件头魔术字节是否匹配声称的扩展名
func hasImageMagicBytes(data []byte, ext string) bool {
	switch ext {
	case ".jpg", ".jpeg":
		return data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF
	case ".png":
		return data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 &&
			data[4] == 0x0D && data[5] == 0x0A && data[6] == 0x1A && data[7] == 0x0A
	case ".gif":
		return data[0] == 'G' && data[1] == 'I' && data[2] == 'F' && data[3] == '8'
	case ".webp":
		return data[0] == 'R' && data[1] == 'I' && data[2] == 'F' && data[3] == 'F'
	case ".bmp":
		return data[0] == 'B' && data[1] == 'M'
	default:
		return false
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// PUT /api/author/profile — 更新作者资料（签名、隔离开关）
func UpdateAuthorProfile(c *gin.Context) {
	userID := c.GetUint("userID")

	var req struct {
		Signature   *string `json:"signature"`
		WallEnabled *bool   `json:"wall_enabled"` // 针对某个作品
		NovelID     *uint   `json:"novel_id"`     // 可选：指定作品
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}

	// 更新签名
	if req.Signature != nil {
		sig := strings.TrimSpace(*req.Signature)
		if len([]rune(sig)) > 128 {
			utils.BadRequest(c, "签名不能超过128个字符")
			return
		}
		models.DB.Model(&models.User{}).Where("id = ?", userID).Update("signature", bluemonday.StrictPolicy().Sanitize(sig))
	}

	// 更新作品隔离墙开关
	if req.WallEnabled != nil && req.NovelID != nil {
		models.DB.Model(&models.Novel{}).
			Where("id = ? AND author_id = ?", *req.NovelID, userID).
			Update("wall_enabled", *req.WallEnabled)
	}

	utils.SuccessMessage(c, "更新成功", nil)
}