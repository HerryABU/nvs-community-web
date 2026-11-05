package admingov

import (
	"nvs-server/handlers/admincommon"
	"strconv"
	"strings"

	"nvs-server/handlers/hctx"
	"nvs-server/police"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// AdminListEvidenceRequests GET /api/admin/police/evidence-requests
// 站长查看警方提交的取证导出申请（按状态筛，待审在前）。
func AdminListEvidenceRequests(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")
	list, total, err := police.ListEvidenceRequests(page, size, status)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"items": list, "total": total, "page": page})
}

// AdminReviewEvidenceRequest POST /api/admin/police/evidence-requests/:id/review
// 站长审批：approve=true 批准 / approve=false 驳回。
func AdminReviewEvidenceRequest(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		utils.BadRequest(c, "参数错误")
		return
	}
	var req struct {
		Approve bool   `json:"approve"`
		Note    string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if !req.Approve && strings.TrimSpace(req.Note) == "" {
		utils.BadRequest(c, "驳回须填写理由")
		return
	}
	adminID := hctx.CurrentUserID(c)
	if err := police.ReviewEvidenceRequest(uint(id), adminID, req.Approve, req.Note); err != nil {
		utils.InternalError(c, err.Error())
		return
	}
	utils.Success(c, gin.H{"message": "已处理"})
}
