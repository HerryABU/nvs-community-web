package handlers

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nvs-server/config"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)


// POST /api/author/avatar — 作者上传头像
func UploadAvatar(c *gin.Context) {
	userID := c.GetUint("userID")

	file, header, err := c.Request.FormFile("avatar")
	if err != nil {
		utils.BadRequest(c, "请选择要上传的头像")
		return
	}
	defer file.Close()

	// 安全检查：限制文件大小（最大 5MB）
	if header.Size > 5*1024*1024 {
		utils.BadRequest(c, "头像文件大小不能超过 5MB")
		return
	}

	// 安全检查：只允许图片类型
	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true}
	if !allowedExts[ext] {
		utils.BadRequest(c, "只支持 JPG/PNG/GIF 格式的头像")
		return
	}

	// 读取原始数据
	rawData, err := io.ReadAll(file)
	if err != nil {
		utils.InternalError(c, "读取头像文件失败")
		return
	}

	// 验证文件头魔术字节（防止伪造扩展名的图片马）
	if !hasImageMagicBytes(rawData, ext) {
		utils.BadRequest(c, "文件内容与扩展名不匹配，可能是恶意文件")
		return
	}

	// 验证 MIME 类型（第二层防护）
	contentType := http.DetectContentType(rawData)
	if !strings.HasPrefix(contentType, "image/") {
		utils.BadRequest(c, "文件不是有效的图片格式")
		return
	}

	// 重新编码：解码 → 编码为 PNG（丢弃所有元数据、隐藏payload）
	img, format, err := image.Decode(bytes.NewReader(rawData))
	if err != nil {
		utils.BadRequest(c, "无法解码图片，文件可能已损坏")
		return
	}
	_ = format

	// 创建上传目录
	avatarDir := filepath.Join(config.UploadDir, "avatars")
	os.MkdirAll(avatarDir, 0755)

	// 先编码到内存 buffer，确定最终格式后再写文件（避免后缀不一致）
	var buf bytes.Buffer
	ext = ".png"
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
			ext = ".gif"
		} else {
			ext = ".jpg"
		}
	}

	// 根据最终编码格式生成正确的文件名和后缀
	filename := fmt.Sprintf("user_%d_%d%s", userID, time.Now().UnixNano(), ext)
	avatarPath := filepath.Join(avatarDir, filename)

	if err := os.WriteFile(avatarPath, buf.Bytes(), 0644); err != nil {
		utils.InternalError(c, "保存头像失败")
		return
	}

	// 更新用户头像 URL
	avatarURL := fmt.Sprintf("/uploads/avatars/%s", filename)
	models.DB.Model(&models.User{}).Where("id = ?", userID).Update("avatar_url", avatarURL)

	utils.Success(c, gin.H{
		"avatar_url": avatarURL,
		"message":    "头像上传成功",
	})
}

// ============ 作者签名状态 ============

// GET /api/author/signature-status — 获取作者签名状态与密钥信息
func GetSignatureStatus(c *gin.Context) {
	userID := c.GetUint("userID")

	user, err := models.GetUserByID(userID)
	if err != nil {
		utils.NotFound(c, "用户不存在")
		return
	}

	hasKey := user.SigningKey != ""

	// 统计该用户作品的签名覆盖率
	type signStats struct {
		Total    int64
		Signed   int64
	}
	var stats signStats
	models.DB.Raw(
		"SELECT COUNT(*) AS total, COALESCE(SUM(CASE WHEN content_signature != '' THEN 1 ELSE 0 END), 0) AS signed "+
			"FROM chapters WHERE novel_id IN (SELECT id FROM novels WHERE author_id = ?)",
		userID,
	).Scan(&stats)

	utils.Success(c, gin.H{
		"has_signing_key":    hasKey,
		"total_chapters":     stats.Total,
		"signed_chapters":    stats.Signed,
		"unsigned_chapters":  stats.Total - stats.Signed,
		"coverage_percent":   func() float64 {
			if stats.Total == 0 {
				return 100.0
			}
			return float64(stats.Signed) / float64(stats.Total) * 100.0
		}(),
	})
}

// ============ 外联站代理 ============