package handlers

import (
	"fmt"
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
	allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}
	if !allowedExts[ext] {
		utils.BadRequest(c, "只支持 JPG/PNG/GIF/WebP 格式的头像")
		return
	}

	// 读取文件数据
	data, err := io.ReadAll(file)
	if err != nil {
		utils.InternalError(c, "读取头像文件失败")
		return
	}

	// 验证是否是有效图片（防止非图片文件伪装）
	contentType := http.DetectContentType(data)
	if !strings.HasPrefix(contentType, "image/") {
		utils.BadRequest(c, "文件不是有效的图片格式")
		return
	}

	// 创建上传目录
	avatarDir := filepath.Join(config.UploadDir, "avatars")
	os.MkdirAll(avatarDir, 0755)

	// 生成文件名：user_{id}_{timestamp}.{ext}
	filename := fmt.Sprintf("user_%d_%d%s", userID, time.Now().Unix(), ext)
	avatarPath := filepath.Join(avatarDir, filename)

	// 写入文件
	if err := os.WriteFile(avatarPath, data, 0644); err != nil {
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
