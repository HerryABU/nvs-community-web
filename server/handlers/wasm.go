package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"nvs-server/config"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ==================== 沙盒预览 + WASM静态服务 ====================

// ServeSandboxPreview GET /sandbox/frame/:userID/*filepath
// 在安全沙盒中返回用户自定义内容文件（CSS/JS/WASM等）
func ServeSandboxPreview(c *gin.Context) {
	userIDStr := c.Param("userID")
	filePath := c.Param("filepath")

	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		c.String(http.StatusBadRequest, "无效的用户ID")
		return
	}

	// 路径映射：frame → UserFrameDir, html → UserHTMLDir
	routes := []struct {
		prefix string
		dir    string
	}{
		{"frame", config.UserFrameDir},
		{"html", config.UserHTMLDir},
	}

	for _, route := range routes {
		if strings.HasPrefix(filePath, route.prefix) {
			relPath := strings.TrimPrefix(filePath, route.prefix)
			relPath = strings.TrimPrefix(relPath, "/")

			fullPath := filepath.Join(route.dir, fmt.Sprintf("%d", userID), relPath)

			// 安全检查
			absBase, _ := filepath.Abs(filepath.Join(route.dir, fmt.Sprintf("%d", userID)))
			absPath, _ := filepath.Abs(fullPath)
			if !strings.HasPrefix(absPath, absBase) {
				c.String(http.StatusForbidden, "路径越权")
				return
			}

			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				c.String(http.StatusNotFound, "文件不存在")
				return
			}

			// 安全响应头
			setSandboxHeaders(c)

			// WASM MIME
			if strings.HasSuffix(relPath, ".wasm") {
				c.Header("Content-Type", "application/wasm")
			}

			c.File(fullPath)
			return
		}
	}

	c.String(http.StatusBadRequest, "无效的沙盒路径")
}

// GetSandboxInfo GET /api/sandbox/info
func GetSandboxInfo(c *gin.Context) {
	utils.Success(c, gin.H{
		"sandbox_policy":     "allow-scripts allow-same-origin",
		"wasm_supported":     true,
		"zip_max_size":       "20MB",
		"zip_max_uncompress": "50MB",
		"zip_max_ratio":      "100:1",
		"allowed_extensions": []string{
			".html", ".htm", ".css", ".js", ".wasm",
			".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp",
			".json", ".xml", ".woff2", ".data",
		},
	})
}

// GetFramePreview GET /api/userframes/:id/preview
// 返回模板的沙盒预览页面（嵌入平台API）
func GetFramePreview(c *gin.Context) {
	frameID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var frame models.UserFrame
	if err := models.DB.Where("id = ? AND is_active = ?", frameID, true).First(&frame).Error; err != nil {
		utils.NotFound(c, "模板不存在或已禁用")
		return
	}

	content, err := os.ReadFile(frame.FilePath)
	if err != nil {
		utils.InternalError(c, "读取模板内容失败")
		return
	}

	// 根据 sandbox_level 决定 sandbox 策略
	sandboxPolicy := "allow-scripts allow-same-origin"
	if frame.HasControls {
		sandboxPolicy += " allow-forms allow-popups"
	}

	wrapped := wrapSandboxPage(string(content), frame.Name, sandboxPolicy, frame.UsesNovelAPI, "")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(wrapped))
}

// GetHTMLPreview GET /api/userhtmls/:id/preview
// 返回扩展HTML的沙盒预览（从解压目录加载入口文件）
func GetHTMLPreview(c *gin.Context) {
	htmlID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var htmlItem models.UserHTML
	if err := models.DB.Where("id = ? AND is_active = ?", htmlID, true).First(&htmlItem).Error; err != nil {
		utils.NotFound(c, "扩展不存在或已禁用")
		return
	}

	// 读取入口文件
	entryPath := filepath.Join(htmlItem.ExtractDir, htmlItem.EntryFile)
	content, err := os.ReadFile(entryPath)
	if err != nil {
		utils.InternalError(c, "读取入口文件失败")
		return
	}

	sandboxPolicy := "allow-scripts allow-same-origin"
	if htmlItem.AllowWasm {
		sandboxPolicy += " allow-forms"
	}

	// 虚拟路径base，隐藏真实存储路径
	basePath := fmt.Sprintf("/app/%d/", htmlID)
	wrapped := wrapSandboxPage(string(content), htmlItem.Name, sandboxPolicy, false, basePath)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(wrapped))
}

// ==================== 辅助 ====================

// ServeAppHTML GET /app/:htmlId
// 虚拟沙盒入口——隐藏真实存储路径
func ServeAppHTML(c *gin.Context) {
	htmlID, _ := strconv.ParseUint(c.Param("htmlId"), 10, 64)
	var htmlItem models.UserHTML
	if err := models.DB.Where("id = ? AND is_active = ?", htmlID, true).First(&htmlItem).Error; err != nil {
		c.String(http.StatusNotFound, "应用不存在")
		return
	}
	entryPath := filepath.Join(htmlItem.ExtractDir, htmlItem.EntryFile)
	content, err := os.ReadFile(entryPath)
	if err != nil {
		c.String(http.StatusInternalServerError, "读取失败")
		return
	}
	// base指向虚拟路径，沙盒内无法探测 /api/userhtmls/
	virtualBase := fmt.Sprintf("/app/%d/", htmlID)
	wrapped := wrapAppPage(string(content), htmlItem.Name, virtualBase)
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(wrapped))
}

// ServeAppResource GET /app/:htmlId/*filepath
// 虚拟沙盒静态资源——隐藏真实文件系统路径
func ServeAppResource(c *gin.Context) {
	htmlID, _ := strconv.ParseUint(c.Param("htmlId"), 10, 64)
	resourcePath := c.Param("filepath")
	resourcePath = strings.TrimPrefix(resourcePath, "/")

	var htmlItem models.UserHTML
	if err := models.DB.Where("id = ? AND is_active = ?", htmlID, true).First(&htmlItem).Error; err != nil {
		c.String(http.StatusNotFound, "资源不存在")
		return
	}

	fullPath := filepath.Join(htmlItem.ExtractDir, resourcePath)
	absBase, _ := filepath.Abs(htmlItem.ExtractDir)
	absTarget, _ := filepath.Abs(fullPath)
	if !strings.HasPrefix(absTarget, absBase) {
		c.String(http.StatusForbidden, "路径越权")
		return
	}

	setSandboxHeaders(c)
	if strings.HasSuffix(resourcePath, ".wasm") {
		c.Header("Content-Type", "application/wasm")
	}
	c.File(fullPath)
}

// wrapAppPage 全页沙盒包装（无NVS标识栏，纯净运行环境）
func wrapAppPage(content, name, base string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>%s</title>
<base href="%s">
<meta http-equiv="Content-Security-Policy" content="default-src 'self' 'unsafe-inline' 'unsafe-eval' 'wasm-unsafe-eval'; script-src 'self' 'unsafe-inline' 'unsafe-eval' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; connect-src 'self' http: https: ws: wss: data: blob:; frame-ancestors 'self';">
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;background:#fff;color:#333;line-height:1.6;overflow:auto}
</style>
</head>
<body>
%s
</body>
</html>`, name, base, content)
}

func baseTag(path string) string {
	if path == "" {
		return ""
	}
	return fmt.Sprintf("<base href=\"%s\">\n", path)
}

func setSandboxHeaders(c *gin.Context) {
	c.Header("Content-Security-Policy",
		"default-src 'self' 'unsafe-inline'; "+
			"script-src 'self' 'unsafe-eval' 'wasm-unsafe-eval'; "+
			"style-src 'self' 'unsafe-inline'; "+
			"img-src 'self' data: blob:; "+
			"connect-src 'self' http: https: ws: wss: data: blob:; "+
			"frame-ancestors 'self'; "+
			"base-uri 'self'")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Frame-Options", "SAMEORIGIN")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
}

func wrapSandboxPage(content, title, sandboxPolicy string, hasAPI bool, basePath string) string {
	safeTitle := title
	if safeTitle == "" {
		safeTitle = "沙盒预览"
	}

	pinnedName := "📚 小说模板预览"
	if !hasAPI {
		pinnedName = "🔌 扩展应用"
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
%s<title>%s - 沙盒预览</title>
<meta http-equiv="Content-Security-Policy" content="default-src 'self' 'unsafe-inline' 'unsafe-eval' 'wasm-unsafe-eval'; script-src 'self' 'unsafe-inline' 'unsafe-eval' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; connect-src 'self' http: https: ws: wss:; frame-ancestors 'self';">
<style>
* { box-sizing: border-box; margin: 0; padding: 0; }
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #fff; color: #333; line-height: 1.6; }
[data-theme="dark"] body { background: #1a1a2e; color: #ddd; }
.sandbox-badge { position: fixed; bottom: 8px; right: 8px; background: rgba(0,0,0,0.7); color: #fff; padding: 2px 8px; border-radius: 4px; font-size: 11px; z-index: 99999; pointer-events: none; }
.sandbox-info { position: fixed; top: 0; left: 0; right: 0; padding: 4px 12px; background: #f0f0f0; border-bottom: 1px solid #ddd; font-size: 11px; color: #666; z-index: 99998; }
.sandbox-info span { margin-right: 12px; }
</style>
</head>
<body>
<div class="sandbox-info"><span>%s</span><span>Policy: %s</span></div>
<div style="padding-top: 26px;">
%s
</div>
<div class="sandbox-badge">🔒 沙盒 · NVS</div>
</body>
</html>`, baseTag(basePath), safeTitle, pinnedName, sandboxPolicy, content)
}
