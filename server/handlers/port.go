package handlers

import (
	"fmt"

	"nvs-server/middleware"
	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// AllocatePort POST /api/port/allocate
// 为扩展HTML分配代理端口 + 用户命名项目名（隐藏端口，防打洞扫描）
func AllocatePort(c *gin.Context) {
	var req struct {
		HTMLID      uint   `json:"html_id" binding:"required"`
		ProjectName string `json:"project_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误: project_name 为必填")
		return
	}

	// 验证项目名（字母数字下划线横线，2-32字符）
	if !isValidProjectName(req.ProjectName) {
		utils.BadRequest(c, "项目名仅允许字母/数字/下划线/横线，2-32字符")
		return
	}

	userID := c.GetUint("user_id")

	// 检查同一用户下项目名唯一
	var existing models.UserHTML
	if err := models.DB.Where("user_id = ? AND project_name = ? AND port > 0 AND is_active = ?", userID, req.ProjectName, true).First(&existing).Error; err == nil {
		utils.BadRequest(c, "项目名 \""+req.ProjectName+"\" 已被使用，请换一个")
		return
	}

	allocator := utils.GetPortAllocator()
	port, err := allocator.Allocate()
	if err != nil {
		utils.InternalError(c, "端口分配失败: "+err.Error())
		return
	}

	// 注册命名代理（隐藏端口号在URL中）
	proxyURL := "http://127.0.0.1:" + formatPort(port)
	middleware.GlobalNamedProxy.Register(userID, req.ProjectName, port, proxyURL)

	// 保存端口+项目名到数据库
	models.DB.Model(&models.UserHTML{}).Where("id = ?", req.HTMLID).Updates(map[string]interface{}{
		"port":         port,
		"project_name": req.ProjectName,
	})

	proxyPath := fmt.Sprintf("/sandbox/proxy/%d/%s/", userID, req.ProjectName)

	utils.Success(c, gin.H{
		"port":          port,
		"project_name":  req.ProjectName,
		"proxy_path":    proxyPath,
		"direct_url":    proxyURL,
		"html_id":       req.HTMLID,
		"port_range":    "49152-65535",
		"alloc_strategy": "高位优先 (high-to-low) · 端口号不在URL中暴露",
	})
}

// ReleasePort POST /api/port/release
func ReleasePort(c *gin.Context) {
	var req struct {
		HTMLID      uint   `json:"html_id"`
		ProjectName string `json:"project_name"`
		Port        int    `json:"port"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}

	userID := c.GetUint("user_id")

	// 注销命名代理
	if req.ProjectName != "" {
		middleware.GlobalNamedProxy.Unregister(userID, req.ProjectName)
	}

	// 释放端口
	if req.Port > 0 {
		utils.GetPortAllocator().Release(req.Port)
	}

	// 清除数据库
	models.DB.Model(&models.UserHTML{}).Where("id = ?", req.HTMLID).Updates(map[string]interface{}{
		"port":         0,
		"project_name": "",
	})

	utils.Success(c, gin.H{"message": "端口已释放"})
}

// ListPorts GET /api/port/list
func ListPorts(c *gin.Context) {
	userID := c.GetUint("user_id")

	var htmls []models.UserHTML
	models.DB.Where("user_id = ? AND port > 0 AND is_active = ?", userID, true).Find(&htmls)

	var result []gin.H
	for _, h := range htmls {
		result = append(result, gin.H{
			"port":         h.Port,
			"project_name": h.ProjectName,
			"proxy_path":   fmt.Sprintf("/sandbox/proxy/%d/%s/", userID, h.ProjectName),
			"html_id":      h.ID,
			"html_name":    h.Name,
		})
	}

	utils.Success(c, result)
}

func isValidProjectName(name string) bool {
	if len(name) < 2 || len(name) > 32 {
		return false
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

func formatPort(port int) string {
	return fmt.Sprintf("%d", port)
}
