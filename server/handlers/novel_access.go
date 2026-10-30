package handlers

import (
	"math"

	"nvs-server/models"
)

// NovelView 作品详情返回体：内嵌 Novel 全部字段（保持前端 res.data.data.xxx 兼容），
// 并额外携带访客可见性策略，前端据此提示「需登录 / 试读上限」。
type NovelView struct {
	models.Novel
	RequiresLogin bool   `json:"requires_login"`   // 匿名必须登录才能查看
	IsTrial       bool   `json:"is_trial"`         // 匿名仅可试读前 TrialLimit 章
	TrialLimit    int    `json:"trial_limit"`      // 试读章节上限
	Byline        string `json:"byline,omitempty"` // 多人协作署名：主笔：X · 协作：A、B
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
//	  · 访客设置 allow_guest_browse=false                → 必须登录（站长总闸）；
//	  · 全局 novel_visitor_mode=login_required            → 必须登录；
//	  · 作者隔离开关 novel.wall_enabled=true              → 必须登录（3.2 作者自决）；
//	  · 全局 novel_visitor_mode=trial（且作者未单独上锁） → 仅试读前 N 章；
//	  · 全局 open（且作者未单独上锁）                     → 完全开放，可正常爬取。
//
// ⚠️ 历史上这里只读 novel_visitor_mode，后台「访客设置」里的
// allow_guest_browse / guest_max_chapters 是**空转**的（只有管理端读写、无人校验），
// 站长以为关掉了游客浏览其实没关。现在两者取「更严」的一侧，让开关真正生效。
//
// 结论（对应 3.3）：仅当「管理员放开(open 且允许访客浏览) 且 作者放开(wall_enabled=false)」时
// 才允许匿名爬取；任一环节要求登录，匿名爬虫即被 401 拦截，无法抓取全文。
func ResolveNovelAccess(novel *models.Novel, isLogin bool, _ string) NovelAccess {
	if isLogin {
		return NovelAccess{RequiresLogin: false, IsTrial: false}
	}
	visit := models.LoadVisitSetting()
	if !visit.AllowGuestBrowse {
		return NovelAccess{RequiresLogin: true, IsTrial: false}
	}
	mode := models.GetNovelVisitorMode()
	walled := novel != nil && novel.WallEnabled
	if mode == "login_required" || walled {
		return NovelAccess{RequiresLogin: true, IsTrial: false}
	}
	if mode == "trial" {
		// 站长可自定义试读章数（0 或负数 → 退回按比例计算的保守值）
		limit := ComputeTrialLimit(0)
		if visit.GuestMaxChapters > 0 {
			limit = visit.GuestMaxChapters
		}
		return NovelAccess{RequiresLogin: false, IsTrial: true, TrialLimit: limit}
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
