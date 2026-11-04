package police

import (
	"fmt"
	"net/http"
	"os"
	"strconv"

	"nvs-server/handlers/hctx"
	"nvs-server/models"
	"nvs-server/police"
	"nvs-server/supervise"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// PoliceCreateEvidenceRequest POST /api/police/evidence-requests
// 警方发起取证导出申请（须经站长批准后才可导出）。
func PoliceCreateEvidenceRequest(c *gin.Context) {
	if !ensurePoliceOrAdmin(c) {
		return
	}
	var req struct {
		TargetUserID uint   `json:"target_user_id"`
		Reason       string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	uid := hctx.CurrentUserID(c)
	if req.TargetUserID == 0 {
		utils.BadRequest(c, "请提供目标用户 ID")
		return
	}
	targetName := ""
	if u, e := models.GetUserByID(req.TargetUserID); e == nil && u != nil {
		targetName = u.Username
	}
	er, err := police.CreateEvidenceRequest(uid, req.TargetUserID, targetName, req.Reason)
	if err != nil {
		utils.InternalError(c, err.Error())
		return
	}
	utils.Success(c, gin.H{"message": "已提交，等待站长审批", "request": er})
}

// PoliceListMyEvidenceRequests GET /api/police/evidence-requests
// 警方查看自己的申请与审批状态。
func PoliceListMyEvidenceRequests(c *gin.Context) {
	if !ensurePoliceOrAdmin(c) {
		return
	}
	uid := hctx.CurrentUserID(c)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	list, total, err := police.ListMyEvidenceRequests(uid, page, size)
	if err != nil {
		utils.InternalError(c, "查询失败")
		return
	}
	utils.Success(c, gin.H{"items": list, "total": total, "page": page})
}

// PoliceExportUserForensics GET /api/police/export-user?user_id=X
// 导出某入档用户的取证包。admin 直接可导出；police 必须已获站长对该目标的批准。
func PoliceExportUserForensics(c *gin.Context) {
	if !ensurePoliceOrAdmin(c) {
		return
	}
	uid := hctx.CurrentUserID(c)
	role := hctx.CurrentUserRole(c)
	target, _ := strconv.ParseUint(c.Query("user_id"), 10, 64)
	if target == 0 {
		utils.BadRequest(c, "请提供 user_id")
		return
	}
	if models.IsPolice(role) && !police.HasApprovedRequest(uid, uint(target)) {
		utils.Forbidden(c, "须先向站长提交取证申请并经批准")
		return
	}
	bundle, err := supervise.BuildSuperviseForensicsZip(uint(target), uid)
	if err != nil {
		utils.InternalError(c, "导出失败："+err.Error())
		return
	}
	defer os.Remove(bundle.Path)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=supervise_%d.zip", target))
	c.Header("Content-Type", "application/zip")
	data, rerr := os.ReadFile(bundle.Path)
	if rerr != nil {
		utils.InternalError(c, "读取导出文件失败")
		return
	}
	c.Data(http.StatusOK, "application/zip", data)
}
