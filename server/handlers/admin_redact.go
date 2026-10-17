package handlers

// 此文件包含日志脱敏配置的管理后台 API（公安部令第176号合规）。
// 路由（main.go 注册，均需管理员权限）：
//   GET  /api/admin/log-redact         获取当前脱敏配置
//   PUT  /api/admin/log-redact         更新脱敏配置（即时生效，无需重启；持久化到 platform_configs）
//   POST /api/admin/log-redact/preview 脱敏效果预览（不修改实际配置）

import (
	"log"
	"strconv"
	"strings"

	"nvs-server/config"
	"nvs-server/middleware"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// GetLogRedactConfig 获取当前日志脱敏配置
// GET /api/admin/log-redact
func GetLogRedactConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	rc := config.RedactMgr.GetConfig()
	utils.Success(c, gin.H{
		"enabled":        rc.Enabled,
		"mask":           rc.Mask,
		"sensitive_keys": rc.SensitiveKeys,
	})
}

// UpdateLogRedactConfig 更新日志脱敏配置：先持久化到数据库，成功后切换内存配置，
// 立即对后续所有请求日志生效（F3：新增字段实时生效，无需重启）。
// PUT /api/admin/log-redact
// 入参（均可选，缺省保留当前值）：enabled bool / mask string / sensitive_keys []string
func UpdateLogRedactConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Enabled       *bool    `json:"enabled"`
		Mask          *string  `json:"mask"`
		SensitiveKeys []string `json:"sensitive_keys"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	rc := config.RedactMgr.GetConfig()
	if req.Enabled != nil {
		rc.Enabled = *req.Enabled
	}
	if req.Mask != nil && strings.TrimSpace(*req.Mask) != "" {
		rc.Mask = strings.TrimSpace(*req.Mask)
	}
	if req.SensitiveKeys != nil {
		rc.SensitiveKeys = req.SensitiveKeys
	}
	if err := config.RedactMgr.Update(rc); err != nil {
		utils.InternalError(c, "保存脱敏配置失败")
		return
	}
	// .env 自动同步（重启后仍生效；写失败仅告警不阻断）
	if _, err := config.SyncPlatformConfigEnv(map[string]string{
		"log_redact_enabled": strconv.FormatBool(rc.Enabled),
		"log_redact_mask":    rc.Mask,
		"log_redact_fields":  strings.Join(rc.SensitiveKeys, ","),
	}); err != nil {
		log.Printf("[admin] 脱敏配置已保存但 .env 同步失败: %v", err)
	}
	utils.Success(c, gin.H{
		"message":        "日志脱敏配置已更新，即时生效",
		"enabled":        rc.Enabled,
		"mask":           rc.Mask,
		"sensitive_keys": rc.SensitiveKeys,
	})
}

// PreviewLogRedact 脱敏效果预览（F4）：按指定类型展示脱敏结果，不修改实际配置。
// POST /api/admin/log-redact/preview
// 入参：
//
//	kind   string   预览类型：body（JSON 请求体，默认）| header（请求头，每行 Key: Value）| query（Query 参数）
//	sample string   原始内容
//	enabled / mask / sensitive_keys 可选，用于预览未保存的表单配置
func PreviewLogRedact(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Kind          string   `json:"kind"`
		Sample        string   `json:"sample"`
		Enabled       *bool    `json:"enabled"`
		Mask          *string  `json:"mask"`
		SensitiveKeys []string `json:"sensitive_keys"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	rc := config.RedactMgr.GetConfig()
	if req.Enabled != nil {
		rc.Enabled = *req.Enabled
	}
	if req.Mask != nil && strings.TrimSpace(*req.Mask) != "" {
		rc.Mask = strings.TrimSpace(*req.Mask)
	}
	if req.SensitiveKeys != nil {
		rc.SensitiveKeys = req.SensitiveKeys
	}
	r := middleware.NewRedactor(rc)

	var result string
	switch req.Kind {
	case "header":
		// 每行一个 "Key: Value"；无冒号的行视为 Cookie 内容
		headers := map[string]string{}
		for _, line := range strings.Split(req.Sample, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if idx := strings.Index(line, ":"); idx > 0 {
				headers[strings.TrimSpace(line[:idx])] = strings.TrimSpace(line[idx+1:])
			} else {
				headers["Cookie"] = line
			}
		}
		out := r.RedactHeaders(headers)
		lines := make([]string, 0, len(out))
		for k, v := range out {
			lines = append(lines, k+": "+v)
		}
		result = strings.Join(lines, "\n")
	case "query":
		result = r.RedactQuery(req.Sample)
	default: // body
		result = r.RedactBody(req.Sample)
	}
	utils.Success(c, gin.H{"result": result})
}
