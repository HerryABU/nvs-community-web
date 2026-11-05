package ops

// ==================== 运营位：自定义菜单 / 轮播 / 签到 / 访客设置（gap-audit #20 #23）====================
// 前台接口（不需要登录）：导航菜单、轮播、签到状态
// 管理接口（ensureAdmin）：全部 CRUD + 统计
//
// 口径说明：签到奖励必须幂等（同一天重复请求不再发），Redis 不在依赖里，
// 所以用「唯一索引 + 先落记录后发币」保证并发安全（见 siteops.DoCheckIn）。

import (
	"nvs-server/handlers/admincommon"
	"nvs-server/handlers/hctx"
	"nvs-server/utils"
	"strconv"

	"github.com/gin-gonic/gin"
	siteops "nvs-server/models/siteops"
	social "nvs-server/models/social"
)

// ─────────────── 导航菜单 ───────────────

// PublicNavMenus GET /api/nav-menus
func PublicNavMenus(c *gin.Context) {
	isLogin := hctx.CurrentUserID(c) > 0
	isAdmin := false
	if isLogin {
		role := hctx.CurrentUserRole(c)
		isAdmin = role == social.RoleAdmin || role == social.RolePolice
	}
	list, err := siteops.PublicNavMenus(isLogin, isAdmin)
	if err != nil {
		utils.InternalError(c, "读取菜单失败")
		return
	}
	utils.Success(c, gin.H{"items": list})
}

// AdminNavMenus GET /api/admin/nav-menus
func AdminNavMenus(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	list, err := siteops.AllNavMenus()
	if err != nil {
		utils.InternalError(c, "读取菜单失败")
		return
	}
	utils.Success(c, gin.H{"items": list})
}

// SaveNavMenu POST /api/admin/nav-menus
func SaveNavMenu(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	var in siteops.NavMenu
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	item, err := siteops.SaveNavMenu(in)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, item)
}

// DeleteNavMenu DELETE /api/admin/nav-menus/:id
func DeleteNavMenu(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := siteops.DeleteNavMenu(uint(id)); err != nil {
		utils.InternalError(c, "删除失败")
		return
	}
	utils.SuccessMessage(c, "已删除", nil)
}

// ─────────────── 轮播 ───────────────

// PublicCarousels GET /api/carousels
func PublicCarousels(c *gin.Context) {
	list, err := siteops.ActiveCarousels()
	if err != nil {
		utils.InternalError(c, "读取轮播失败")
		return
	}
	utils.Success(c, gin.H{"items": list})
}

// AdminCarousels GET /api/admin/carousels
func AdminCarousels(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	list, err := siteops.AdminCarousels()
	if err != nil {
		utils.InternalError(c, "读取轮播失败")
		return
	}
	utils.Success(c, gin.H{"items": list})
}

// SaveCarousel POST /api/admin/carousels
func SaveCarousel(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	var in siteops.Carousel
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	item, err := siteops.SaveCarousel(in)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.Success(c, item)
}

// DeleteCarousel DELETE /api/admin/carousels/:id
func DeleteCarousel(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := siteops.DeleteCarousel(uint(id)); err != nil {
		utils.InternalError(c, "删除失败")
		return
	}
	utils.SuccessMessage(c, "已删除", nil)
}

// ─────────────── 签到 ───────────────

// CheckInStatus GET /api/checkin/status
func CheckInStatus(c *gin.Context) {
	utils.Success(c, siteops.CheckInStatus(hctx.CurrentUserID(c)))
}

// DoCheckIn POST /api/checkin
func DoCheckIn(c *gin.Context) {
	uid := hctx.CurrentUserID(c)
	if uid == 0 {
		utils.Unauthorized(c, "请先登录")
		return
	}
	res, err := siteops.DoCheckIn(uid)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if res.Already {
		utils.SuccessMessage(c, "今天已经签到过了", res)
		return
	}
	utils.SuccessMessage(c, "签到成功", res)
}

// AdminCheckIn GET /api/admin/checkin —— 配置 + 近 7 天统计
func AdminCheckIn(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	utils.Success(c, gin.H{
		"config": siteops.LoadCheckInConfig(),
		"stats":  siteops.CheckInStats(),
	})
}

// SaveCheckIn PUT /api/admin/checkin
func SaveCheckIn(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	var in siteops.CheckInConfig
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := siteops.SaveCheckInConfig(in); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.SuccessMessage(c, "已保存", siteops.LoadCheckInConfig())
}

// ─────────────── 访客设置 ───────────────

// PublicVisitSetting GET /api/visit-setting —— 前台用于决定「要不要拦」
func PublicVisitSetting(c *gin.Context) {
	s := siteops.LoadVisitSetting()
	utils.Success(c, gin.H{
		"allow_guest_browse": s.AllowGuestBrowse,
		"allow_guest_search": s.AllowGuestSearch,
		"guest_max_chapters": s.GuestMaxChapters,
		"show_visit_count":   s.ShowVisitCount,
		"notice_text":        s.NoticeText,
	})
}

// AdminVisitSetting GET /api/admin/visit-setting
func AdminVisitSetting(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	utils.Success(c, gin.H{
		"config": siteops.LoadVisitSetting(),
		"stats":  siteops.VisitStats(14),
	})
}

// SaveVisitSetting PUT /api/admin/visit-setting
func SaveVisitSetting(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	var in siteops.VisitSetting
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := siteops.SaveVisitSetting(in); err != nil {
		utils.InternalError(c, "保存失败")
		return
	}
	utils.SuccessMessage(c, "已保存", siteops.LoadVisitSetting())
}

// ─────────────── 直播入口（gap-audit #23）───────────────

// PublicLive GET /api/live —— 前台直播状态（未开启时 enabled=false，前端不显示入口）
func PublicLive(c *gin.Context) {
	s := siteops.LoadLiveSetting()
	if !s.Enabled {
		utils.Success(c, gin.H{"enabled": false})
		return
	}
	// 需要登录才看内容时，未登录只回「有直播 + 要不要登录」
	login := hctx.CurrentUserID(c) > 0
	pay := gin.H{
		"enabled": true, "title": s.Title, "status": s.Status,
		"notice": s.Notice, "starts_at": s.StartsAt,
		"chat_group_id": s.ChatGroupID, "require_login": s.RequireLogin,
	}
	if s.RequireLogin && !login {
		pay["need_login"] = true
		utils.Success(c, pay)
		return
	}
	pay["embed_url"] = s.EmbedURL
	pay["video_url"] = s.VideoURL
	pay["poster_url"] = s.PosterURL
	utils.Success(c, pay)
}

// AdminLive GET /api/admin/live
func AdminLive(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	utils.Success(c, gin.H{
		"config": siteops.LoadLiveSetting(),
		"note":   "本站不提供推流/转码/HLS：这里只做入口与嵌入。外部直播间填 embed_url（必须 https），录播可填 video_url 直链。",
	})
}

// SaveLive PUT /api/admin/live
func SaveLive(c *gin.Context) {
	if !admincommon.EnsureAdmin(c) {
		return
	}
	var in siteops.LiveSetting
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := siteops.SaveLiveSetting(in); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	utils.SuccessMessage(c, "已保存", siteops.LoadLiveSetting())
}
