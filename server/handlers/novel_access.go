package handlers

import (
	"math"

	"nvs-server/models"
)

// NovelView 作品详情返回体：内嵌 Novel 全部字段（保持前端 res.data.data.xxx 兼容），
// 并额外携带访客可见性策略，前端据此提示「需登录 / 试读上限」。
type NovelView struct {
	models.Novel
	RequiresLogin bool `json:"requires_login"` // 匿名必须登录才能查看
	IsTrial       bool `json:"is_trial"`        // 匿名仅可试读前 TrialLimit 章
	TrialLimit    int  `json:"trial_limit"`     // 试读章节上限
}

// NovelAccess 解析某作品对当前访问者的可见性策略。
type NovelAccess struct {
	RequiresLogin bool // 匿名必须登录才能查看（无论阅读还是爬取均被拦截）
	IsTrial       bool // 匿名仅可试读前 TrialLimit 章
	TrialLimit    int  // 试读章节上限
}

// ResolveNovelAccess 计算访问策略（核心逻辑，3.1/3.2/3.3 统一在此收敛）：
//
//	已登录用户：全部可见（含完整爬取）。
//	匿名用户：
//	  · 全局 novel_visitor_mode=login_required            → 必须登录；
//	  · 作者隔离开关 novel.wall_enabled=true              → 必须登录（3.2 作者自决）；
//	  · 全局 novel_visitor_mode=trial（且作者未单独上锁） → 仅试读前 min(5, 30%) 章；
//	  · 全局 open（且作者未单独上锁）                     → 完全开放，可正常爬取。
//
// 结论（对应 3.3）：仅当「管理员放开(open) 且 作者放开(wall_enabled=false)」时才允许匿名爬取；
// 任一环节要求登录，匿名爬虫即被 401 拦截，无法抓取全文。
func ResolveNovelAccess(novel *models.Novel, isLogin bool, _ string) NovelAccess {
	if isLogin {
		return NovelAccess{RequiresLogin: false, IsTrial: false}
	}
	mode := models.GetNovelVisitorMode()
	walled := novel != nil && novel.WallEnabled
	if mode == "login_required" || walled {
		return NovelAccess{RequiresLogin: true, IsTrial: false}
	}
	if mode == "trial" {
		return NovelAccess{RequiresLogin: false, IsTrial: true, TrialLimit: ComputeTrialLimit(0)}
	}
	return NovelAccess{RequiresLogin: false, IsTrial: false}
}

// ComputeTrialLimit 计算试读章节上限：min(5, ceil(total*0.3))；total<=0 时取保守值 5。
func ComputeTrialLimit(total int) int {
	if total <= 0 {
		return 5
	}
	limit := int(math.Ceil(float64(total) * 0.3))
	if limit < 1 {
		limit = 1
	}
	if limit > 5 {
		limit = 5
	}
	return limit
}
