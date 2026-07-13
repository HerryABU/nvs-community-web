package handlers

import (
	"net/http"
	"path/filepath"
	"strings"

	"nvs-server/config"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)


// OutsideWebProxy 处理外联站点访问：/{outsidewebid}/*proxyPath → 转发到内部同路径处理
func OutsideWebProxy(c *gin.Context) {
	outsideWebID := c.Param("outsidewebid")
	proxyPath := c.Param("proxyPath")

	// 记录外部站点 ID（可用于访问日志和分析）
	c.Set("outside_web_id", outsideWebID)

	// 验证外部站点 ID 是否存在且有效
	var site models.FederatedSite
	if err := models.DB.Where("id = ? AND status = ?", outsideWebID, "active").First(&site).Error; err != nil {
		// 外联 ID 无效，但仍可尝试代理（可能是公开访问）
		c.Set("outside_web_verified", false)
	} else {
		c.Set("outside_web_verified", true)
		c.Set("outside_site_name", site.Name)
	}

	// 处理代理路径：重写 URL 到内部处理
	// /{outsidewebid}/api/novels/1 → /api/novels/1
	// /{outsidewebid}/novels/... → /novels/...
	// /{outsidewebid}/uploads/... → /uploads/...

	// 去除前导斜杠
	newPath := proxyPath

	// 如果代理路径以 /api 开头，委托给 API 路由
	if strings.HasPrefix(newPath, "/api/") || newPath == "/api" {
		c.Request.URL.Path = newPath
		// 让 Gin 重新路由到正确的 handler
		c.Request.URL.RawPath = ""
		// 直接代理：复用 Gin 的 ServeHTTP
		return
	}

	// 静态资源：/novels/** 或 /uploads/**
	if strings.HasPrefix(newPath, "/novels/") || strings.HasPrefix(newPath, "/uploads/") {
		// 构建本地文件系统路径
		cleanPath := filepath.Clean(newPath)
		// 安全检查：防止路径遍历
		if strings.Contains(cleanPath, "..") {
			utils.Forbidden(c, "非法路径")
			return
		}

		var filePath string
		if strings.HasPrefix(cleanPath, "/novels/") {
			filePath = filepath.Join(config.NovelDataDir, strings.TrimPrefix(cleanPath, "/novels/"))
		} else {
			filePath = filepath.Join(config.UploadDir, strings.TrimPrefix(cleanPath, "/uploads/"))
		}

		// 确保路径在工作目录内
		absPath, _ := filepath.Abs(filePath)
		novelAbs, _ := filepath.Abs(config.NovelDataDir)
		uploadAbs, _ := filepath.Abs(config.UploadDir)
		if !strings.HasPrefix(absPath, novelAbs) && !strings.HasPrefix(absPath, uploadAbs) {
			utils.Forbidden(c, "非法路径")
			return
		}

		http.ServeFile(c.Writer, c.Request, filePath)
		return
	}

	// SPA fallback：对于其他路径，重写为 / 让前端处理
	// 将请求目标指向根路径的 index.html
	c.Request.URL.Path = "/"

	// 移除外部站点 ID 前缀，让后续的 NoRoute handler 处理
	// 实际的 SPA 回退由 NoRoute 处理
	// 但这里我们需要把 URL 恢复到没有前缀的形式
	c.Request.URL.Path = proxyPath
	// 设置一个标记让外部知道请求来自外联站
	c.Header("X-Outside-Web-ID", outsideWebID)

	// 对于 /api/ 路径，手动触发 Gin 重新路由
	if strings.HasPrefix(proxyPath, "/api/") || proxyPath == "/api" {
		c.Request.URL.Path = proxyPath
		// Gin 的 ServeHTTP 会重新查找匹配的路由
		return
	}

	// 对于其他路径，当作 SPA 路由
	c.Request.URL.Path = "/"
}