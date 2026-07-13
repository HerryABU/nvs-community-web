package handlers

import (
	"strconv"
	"time"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// ============ VIP 系统 ============

// POST /api/author/apply-vip — 申请成为 VIP 作者
func ApplyVip(c *gin.Context) {
	// 检查 VIP 功能是否被站长关闭
	if !models.IsVipEnabled() {
		utils.Forbidden(c, "VIP 付费功能暂未开放")
		return
	}

	userID := c.GetUint("userID")

	user, err := models.GetUserByID(userID)
	if err != nil || (user.Role != "author" && user.Role != "vip_author") {
		utils.Forbidden(c, "需要先成为认证作者")
		return
	}

	var req struct{ Reason string `json:"reason"` }
	c.ShouldBindJSON(&req)

	app := &models.VipApplication{UserID: userID, Reason: req.Reason}
	models.DB.Where("user_id = ? AND status = ?", userID, "pending").FirstOrCreate(app)

	utils.Success(c, app)
}

// GET /api/admin/vip-applications — 查看 VIP 申请列表
func ListVipApplications(c *gin.Context) {
	if !ensureAdmin(c) { return }
	var apps []models.VipApplication
	models.DB.Preload("User").Order("created_at DESC").Find(&apps)
	utils.Success(c, apps)
}

// POST /api/admin/vip-applications/:id/approve
func ApproveVip(c *gin.Context) {
	if !ensureAdmin(c) { return }
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	now := time.Now()
	if err := models.DB.Model(&models.VipApplication{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status": "approved", "reviewed_at": now,
	}).Error; err != nil {
		utils.InternalError(c, "操作失败")
		return
	}

	var app models.VipApplication
	models.DB.First(&app, id)
	models.DB.Model(&models.User{}).Where("id = ?", app.UserID).Update("role", "vip_author")

	utils.Success(c, gin.H{"message": "已批准"})
}

// ============ 站长面板 ============

// GET /api/admin/stats — 平台统计
func GetAdminStats(c *gin.Context) {
	if !ensureAdmin(c) { return }
	var userCount, novelCount, commentCount, forumCount int64
	models.DB.Model(&models.User{}).Count(&userCount)
	models.DB.Model(&models.Novel{}).Count(&novelCount)
	models.DB.Model(&models.Comment{}).Count(&commentCount)
	models.DB.Model(&models.Forum{}).Count(&forumCount)

	utils.Success(c, gin.H{
		"users": userCount, "novels": novelCount, "comments": commentCount, "forums": forumCount,
	})
}

// ============ 数据大屏 ============

// GET /api/admin/dashboard — 管理端数据大屏
func GetDashboardStats(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}

	// --- overview ---
	var userCount, novelCount, commentCount, forumCount int64
	models.DB.Model(&models.User{}).Count(&userCount)
	models.DB.Model(&models.Novel{}).Count(&novelCount)
	models.DB.Model(&models.Comment{}).Count(&commentCount)
	models.DB.Model(&models.Forum{}).Count(&forumCount)

	// --- 最近7天日期列表 ---
	now := time.Now()
	dates := make([]string, 7)
	for i := 0; i < 7; i++ {
		dates[6-i] = now.AddDate(0, 0, -i).Format("2006-01-02")
	}

	// --- trend_visits: 每天新增用户/作品/评论 ---
	type trendVisitRow struct {
		Date  string
		Users int64
	}
	type trendNovelRow struct {
		Date   string
		Novels int64
	}
	type trendCommentRow struct {
		Date     string
		Comments int64
	}

	var dailyUsers []trendVisitRow
	var dailyNovels []trendNovelRow
	var dailyComments []trendCommentRow

	models.DB.Raw(
		"SELECT date(created_at) AS date, COUNT(*) AS users FROM users WHERE created_at >= ? GROUP BY date ORDER BY date",
		dates[0],
	).Scan(&dailyUsers)

	models.DB.Raw(
		"SELECT date(created_at) AS date, COUNT(*) AS novels FROM novels WHERE created_at >= ? GROUP BY date ORDER BY date",
		dates[0],
	).Scan(&dailyNovels)

	models.DB.Raw(
		"SELECT date(created_at) AS date, COUNT(*) AS comments FROM comments WHERE created_at >= ? GROUP BY date ORDER BY date",
		dates[0],
	).Scan(&dailyComments)

	// 按日期合并
	userMap := make(map[string]int64)
	novelMap := make(map[string]int64)
	commentMap := make(map[string]int64)
	for _, r := range dailyUsers {
		userMap[r.Date] = r.Users
	}
	for _, r := range dailyNovels {
		novelMap[r.Date] = r.Novels
	}
	for _, r := range dailyComments {
		commentMap[r.Date] = r.Comments
	}

	trendVisits := make([]gin.H, 0, 7)
	for _, d := range dates {
		trendVisits = append(trendVisits, gin.H{
			"date":     d,
			"users":    userMap[d],
			"novels":   novelMap[d],
			"comments": commentMap[d],
		})
	}

	// --- trend_chapters: 最近7天每天新增章节数 ---
	type trendChapterRow struct {
		Date  string
		Count int64
	}
	var dailyChapters []trendChapterRow
	models.DB.Raw(
		"SELECT date(created_at) AS date, COUNT(*) AS count FROM chapters WHERE created_at >= ? GROUP BY date ORDER BY date",
		dates[0],
	).Scan(&dailyChapters)

	chapterMap := make(map[string]int64)
	for _, r := range dailyChapters {
		chapterMap[r.Date] = r.Count
	}
	trendChapters := make([]gin.H, 0, 7)
	for _, d := range dates {
		trendChapters = append(trendChapters, gin.H{
			"date":  d,
			"count": chapterMap[d],
		})
	}

	// --- top_novels: 字数最多的前6部作品 ---
	type topNovelRow struct {
		Title         string
		TotalWords    int
		TotalChapters int
		AuthorName    string
	}
	var topNovelsRaw []topNovelRow
	models.DB.Raw(
		"SELECT n.title, n.total_words, n.total_chapters, u.nickname AS author_name "+
			"FROM novels n JOIN users u ON u.id = n.author_id "+
			"ORDER BY n.total_words DESC LIMIT 6",
	).Scan(&topNovelsRaw)

	topNovels := make([]gin.H, 0, len(topNovelsRaw))
	for _, r := range topNovelsRaw {
		topNovels = append(topNovels, gin.H{
			"title":          r.Title,
			"total_words":    r.TotalWords,
			"total_chapters": r.TotalChapters,
			"author_name":    r.AuthorName,
		})
	}

	// --- category_distribution ---
	type catRow struct {
		Name  string
		Count int64
	}
	var catDist []catRow
	models.DB.Raw(
		"SELECT COALESCE(category,'未分类') AS name, COUNT(*) AS count FROM novels GROUP BY category ORDER BY count DESC",
	).Scan(&catDist)

	categoryDistribution := make([]gin.H, 0, len(catDist))
	for _, r := range catDist {
		categoryDistribution = append(categoryDistribution, gin.H{
			"name":  r.Name,
			"count": r.Count,
		})
	}

	// 构建前端兼容的趋势格式: user_trend {dates, new_users}, chapter_trend {dates, counts}
	userTrendDates := dates
	userTrendNewUsers := make([]int64, 7)
	for i, d := range dates {
		userTrendNewUsers[i] = userMap[d]
	}
	chapterTrendCounts := make([]int64, 7)
	for i, d := range dates {
		chapterTrendCounts[i] = chapterMap[d]
	}

	utils.Success(c, gin.H{
		"stats": gin.H{
			"users":    userCount,
			"novels":   novelCount,
			"comments": commentCount,
			"forums":   forumCount,
		},
		"trend_visits":   trendVisits,
		"trend_chapters": trendChapters,
		"user_trend": gin.H{
			"dates":     userTrendDates,
			"new_users": userTrendNewUsers,
		},
		"chapter_trend": gin.H{
			"dates": dates,
			"counts": chapterTrendCounts,
		},
		"top_novels":            topNovels,
		"category_distribution": categoryDistribution,
	})
}

// ============ 财务 ============

// POST /api/author/withdraw — 申请提现
func RequestWithdraw(c *gin.Context) {
	userID := c.GetUint("userID")
	var req struct {
		Amount  float64 `json:"amount" binding:"required"`
		Method  string  `json:"method"`
		Account string  `json:"account"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Amount <= 0 {
		utils.BadRequest(c, "参数错误")
		return
	}

	wr := &models.WithdrawalRequest{UserID: userID, Amount: req.Amount, Method: req.Method, Account: req.Account}
	models.DB.Create(wr)
	utils.Success(c, wr)
}

// GET /api/author/earnings — 我的收益
func GetEarnings(c *gin.Context) {
	userID := c.GetUint("userID")
	var total float64
	models.DB.Model(&models.EarningsRecord{}).Where("user_id = ?", userID).Select("COALESCE(SUM(amount),0)").Scan(&total)
	utils.Success(c, gin.H{"total_earnings": total})
}

// GET /api/admin/finance — 财务总览
func GetFinanceOverview(c *gin.Context) {
	if !ensureAdmin(c) { return }
	var totalTips float64
	models.DB.Model(&models.EarningsRecord{}).Select("COALESCE(SUM(amount),0)").Scan(&totalTips)

	var userCount int64
	models.DB.Model(&models.User{}).Count(&userCount)

	utils.Success(c, gin.H{
		"total_earnings": totalTips,
		"platform_fee":   totalTips * 0.10,
		"user_count":     userCount,
	})
}

// ============ 辅助 ============

func ensureAdmin(c *gin.Context) bool {
	userID := c.GetUint("userID")
	user, _ := models.GetUserByID(userID)
	if user == nil || user.Role != "admin" {
		utils.Forbidden(c, "需要管理员权限")
		c.Abort()
		return false
	}
	return true
}