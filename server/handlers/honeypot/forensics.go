package honeypothandler

// ==================== 蜜罐：取证包 / 指纹 / 规则反哺 ====================

import (
	"os"
	"path/filepath"
	"strconv"

	"nvs-server/honeypot"
	"nvs-server/middleware"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// Fingerprints GET /api/admin/honeypot/sessions/:sid/fingerprints
func Fingerprints(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	sid, _ := strconv.ParseUint(c.Param("sid"), 10, 64)
	list, err := honeypot.ListFingerprints(uint(sid))
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"list": list})
}

// ExportZip GET /api/admin/honeypot/export.zip?session_id= —— 取证包（含 SHA256 清单）
func ExportZip(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	sid, _ := strconv.ParseUint(c.Query("session_id"), 10, 64)
	bundle, err := honeypot.BuildForensicsZip(uint(sid), middleware.UserID(c))
	if err != nil {
		utils.InternalError(c, "生成取证包失败")
		return
	}
	c.Header("Content-Disposition", "attachment; filename="+filepath.Base(bundle.Path))
	c.Header("X-Bundle-SHA256", bundle.SHA256)
	c.File(bundle.Path)
	// 下载后删除临时包（留 manifest 与审计事件即可）
	go func() { _ = os.Remove(bundle.Path) }()
}

// Rules GET /api/admin/honeypot/rules —— 提炼出的候选规则
func Rules(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	utils.Success(c, gin.H{"list": honeypot.SuggestRules(20)})
}

// ApplyRules POST /api/admin/honeypot/rules { keywords: [] }
func ApplyRules(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var in struct {
		Keywords []string `json:"keywords"`
	}
	if err := c.ShouldBindJSON(&in); err != nil || len(in.Keywords) == 0 {
		utils.BadRequest(c, "请选择要反哺的规则")
		return
	}
	added, err := honeypot.ApplyRules(in.Keywords)
	if err != nil {
		utils.InternalError(c, "写入规则失败")
		return
	}
	utils.SuccessMessage(c, "已写入爬虫阻断规则并即时生效", gin.H{"added": added})
}
