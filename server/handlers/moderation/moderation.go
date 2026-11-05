package moderation

import (
	"errors"
	"nvs-server/handlers/admincommon"
	"nvs-server/handlers/admingov"
	"nvs-server/handlers/hctx"
	"nvs-server/moderation"
	"nvs-server/utils/infra"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// GetModerationConfig GET /api/admin/moderation/config
func GetModerationConfig(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	cfg, err := moderation.GetConfig()
	if err != nil {
		infra.InternalError(c, "读取配置失败")
		return
	}
	infra.Success(c, cfg)
}

// SaveModerationConfig PUT /api/admin/moderation/config
func SaveModerationConfig(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	var cfg moderation.ModerationConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	if cfg.BatchIntervalHours <= 0 {
		cfg.BatchIntervalHours = 6
	}
	if cfg.RecheckReportCount <= 0 {
		cfg.RecheckReportCount = 5
	}
	if cfg.MaliciousThreshold <= 0 {
		cfg.MaliciousThreshold = 5
	}
	if cfg.StopReviewAfter <= 0 {
		cfg.StopReviewAfter = 20
	}
	if err := moderation.SaveConfig(&cfg); err != nil {
		infra.InternalError(c, "保存失败")
		return
	}
	infra.Success(c, cfg)
}

// ListModerationRecords GET /api/admin/moderation/records
func ListModerationRecords(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	f := moderation.RecordFilter{
		Scope:        c.Query("scope"),
		Verdict:      c.Query("verdict"),
		Trigger:      c.Query("trigger"),
		ReviewStatus: c.Query("review_status"),
		Todo:         c.Query("todo") == "1",
		Page:         page,
		PageSize:     50,
	}
	list, total, err := moderation.ListRecords(f)
	if err != nil {
		infra.InternalError(c, "查询失败")
		return
	}
	infra.Success(c, gin.H{"items": list, "total": total, "page": f.Page})
}

// moderationSoftDelType 审核作用域 → 软删留证类型。
// 两者命名一致，但必须显式白名单：软删支持的类型有限，未知类型要拒绝而不是猜。
func moderationSoftDelType(scope string) (string, bool) {
	switch scope {
	case moderation.ScopeNovel, moderation.ScopeChapter, moderation.ScopeBlog,
		moderation.ScopeForum, moderation.ScopeThread, moderation.ScopePost, moderation.ScopeComment:
		return scope, true
	}
	return "", false
}

// DecideModerationRecord POST /api/admin/moderation/records/:id/decide
// body: { action: "pass"|"delete"|"ban", reason? }
//
// AI 只给结论，最终处置由人决定：
//   - pass   放行（内容不动，只记处置结果）
//   - delete 删除并 softdel 留证
//   - ban    封禁并 softdel 留证（写 176 封禁原因，作者端可查）
func DecideModerationRecord(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Action string `json:"action"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		infra.BadRequest(c, "参数错误")
		return
	}
	rec, err := moderation.GetRecord(uint(id))
	if err != nil {
		infra.NotFound(c, "审核记录不存在")
		return
	}
	reason := strings.TrimSpace(req.Reason)
	switch req.Action {
	case moderation.ActionPass:
		// 放行：内容不动
	case moderation.ActionDelete, moderation.ActionBan:
		ctype, ok := moderationSoftDelType(rec.Scope)
		if !ok {
			infra.BadRequest(c, "该内容类型不支持自动处置，请到对应页面处理")
			return
		}
		if reason == "" {
			reason = "AI 内容审核判定违规（人工确认处置）"
		}
		if err := admingov.SoftDeleteContent(c, ctype, rec.TargetID, reason, "moderation", req.Action); err != nil {
			if errors.Is(err, admingov.ErrContentNotFound) {
				infra.NotFound(c, "目标内容不存在（可能已被删除或封禁）")
				return
			}
			infra.InternalError(c, "处置失败："+err.Error())
			return
		}
	default:
		infra.BadRequest(c, "action 只能是 pass / delete / ban")
		return
	}
	if err := moderation.MarkReviewed(rec.ID, req.Action, reason, hctx.CurrentUserID(c)); err != nil {
		infra.InternalError(c, "保存处置结果失败")
		return
	}
	infra.Success(c, gin.H{"message": "已处置", "action": req.Action})
}

// RegisterModerationHooks 把 moderation 的「违规自动处置」接到软删留证。
// main 启动时调用一次（AutoMigrate 与 moderation.Init 之后）。
func RegisterModerationHooks() {
	moderation.SetOnFailHook(func(scope string, targetID uint, reason string) {
		ctype, ok := moderationSoftDelType(scope)
		if !ok {
			return
		}
		if strings.TrimSpace(reason) == "" {
			reason = "AI 审核判定违规（自动下架）"
		}
		_ = admingov.SoftDeleteContentSystem(ctype, targetID, reason, "moderation", "delete")
	})
}

// RunModerationBatch POST /api/admin/moderation/batch
func RunModerationBatch(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	n, err := moderation.RunBatchOnce()
	if err != nil {
		infra.InternalError(c, "批量审核失败")
		return
	}
	infra.Success(c, gin.H{"reviewed": n})
}

// GetModerationStats GET /api/admin/moderation/stats
func GetModerationStats(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	s, err := moderation.GetStats()
	if err != nil {
		infra.InternalError(c, "查询失败")
		return
	}
	infra.Success(c, s)
}

// ListMaliciousReporters GET /api/admin/moderation/malicious
func ListMaliciousReporters(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	list, err := moderation.ListMaliciousReporters()
	if err != nil {
		infra.InternalError(c, "查询失败")
		return
	}
	infra.Success(c, list)
}

// UnbanReporter POST /api/admin/moderation/malicious/:id/unban
func UnbanReporter(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := moderation.UnbanReporter(uint(id)); err != nil {
		infra.InternalError(c, "操作失败")
		return
	}
	infra.Success(c, gin.H{"message": "已恢复举报功能"})
}
