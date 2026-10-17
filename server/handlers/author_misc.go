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
	"golang.org/x/image/webp"
)

// POST /api/author/avatar — 作者上传头像
func UploadAvatar(c *gin.Context) {
	userID := currentUserID(c)

	file, header, err := c.Request.FormFile("avatar")
	if err != nil {
		// 细化错误：区分 Content-Type 缺失 与 boundary 缺失（旧前端缓存常见）
		ct := c.GetHeader("Content-Type")
		if !strings.HasPrefix(strings.ToLower(ct), "multipart/form-data") {
			utils.BadRequest(c, "请求缺少 multipart/form-data（当前 Content-Type: "+ct+"），请强制刷新浏览器（Ctrl+F5）后重试")
			return
		}
		utils.BadRequest(c, "无法解析上传内容（multipart boundary 缺失），请强制刷新浏览器（Ctrl+F5）后重试")
		return
	}
	defer file.Close()

	// 安全检查：限制文件大小（最大 5MB）
	if header.Size > 5*1024*1024 {
		utils.BadRequest(c, "头像文件大小不能超过 5MB")
		return
	}

	// 安全检查：只允许图片类型（含 webp，手机照片常见）
	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowedExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true}
	if !allowedExts[ext] {
		utils.BadRequest(c, "只支持 JPG/PNG/GIF/WebP 格式的头像")
		return
	}

	// 读取原始数据
	rawData, err := io.ReadAll(file)
	if err != nil {
		utils.InternalError(c, "读取头像文件失败")
		return
	}

	// 空/过小文件防护
	if len(rawData) < 8 {
		utils.BadRequest(c, "图片文件为空或已损坏")
		return
	}

	// 按文件头嗅探实际格式（扩展名仅作准入；防马由「解码重编码」保证）
	actual := sniffImageFormat(rawData)
	if actual == "" {
		utils.BadRequest(c, "无法识别的图片格式（支持 JPG/PNG/GIF/WebP）")
		return
	}

	// 验证 MIME 类型（第二层防护）
	contentType := http.DetectContentType(rawData)
	if !strings.HasPrefix(contentType, "image/") {
		utils.BadRequest(c, "文件不是有效的图片格式")
		return
	}

	// 重新编码：解码 → 编码为 PNG（丢弃所有元数据、隐藏payload）
	// 支持 JPG/PNG/GIF/WebP（WebP 由 x/image/webp 提供）
	var img image.Image
	if actual == "webp" {
		img, err = webp.Decode(bytes.NewReader(rawData))
	} else {
		img, _, err = image.Decode(bytes.NewReader(rawData))
	}
	if err != nil {
		utils.BadRequest(c, "无法解码图片，文件可能已损坏或格式不支持（支持 JPG/PNG/GIF/WebP）")
		return
	}

	// 创建上传目录（失败时给出清晰错误，避免"保存失败"迷雾）
	avatarDir := filepath.Join(config.UploadDir, "avatars")
	if err := os.MkdirAll(avatarDir, 0755); err != nil {
		utils.InternalError(c, "无法创建头像目录，请检查 data/uploads 目录权限: "+err.Error())
		return
	}

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
		utils.InternalError(c, "保存头像失败: "+err.Error())
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
	userID := currentUserID(c)

	user, err := models.GetUserByID(userID)
	if err != nil {
		utils.NotFound(c, "用户不存在")
		return
	}

	hasKey := user.SigningKey != ""

	// 统计该用户作品的签名覆盖率
	type signStats struct {
		Total  int64
		Signed int64
	}
	var stats signStats
	models.DB.Raw(
		"SELECT COUNT(*) AS total, COALESCE(SUM(CASE WHEN content_signature != '' THEN 1 ELSE 0 END), 0) AS signed "+
			"FROM chapters WHERE deleted_at IS NULL AND novel_id IN (SELECT id FROM novels WHERE author_id = ? AND deleted_at IS NULL)",
		userID,
	).Scan(&stats)

	utils.Success(c, gin.H{
		"has_signing_key":   hasKey,
		"total_chapters":    stats.Total,
		"signed_chapters":   stats.Signed,
		"unsigned_chapters": stats.Total - stats.Signed,
		"coverage_percent": func() float64 {
			if stats.Total == 0 {
				return 100.0
			}
			return float64(stats.Signed) / float64(stats.Total) * 100.0
		}(),
	})
}

// ============ 外联站代理 ============
