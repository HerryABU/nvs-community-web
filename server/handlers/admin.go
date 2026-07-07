package handlers

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
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

// ============ 举报/仲裁 ============

// POST /api/reports — 提交举报
func CreateReport(c *gin.Context) {
	userID := c.GetUint("userID")
	var req struct {
		TargetType string `json:"target_type" binding:"required"`
		TargetID   uint   `json:"target_id" binding:"required"`
		Reason     string `json:"reason" binding:"required"`
		Detail     string `json:"detail"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}

	r := &models.Report{
		ReporterID: userID, TargetType: req.TargetType, TargetID: req.TargetID,
		Reason: req.Reason, Detail: html.EscapeString(req.Detail),
	}
	models.DB.Create(r)
	utils.Success(c, r)
}

// GET /api/admin/reports — 举报列表
func ListReports(c *gin.Context) {
	if !ensureAdmin(c) { return }
	var reports []models.Report
	models.DB.Preload("Reporter").Order("created_at DESC").Find(&reports)
	utils.Success(c, reports)
}

// POST /api/admin/reports/:id/handle — 处理举报
func HandleReport(c *gin.Context) {
	if !ensureAdmin(c) { return }
	userID := c.GetUint("userID")
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var req struct {
		Status  string `json:"status" binding:"required"`
		Verdict string `json:"verdict"`
	}
	c.ShouldBindJSON(&req)

	models.DB.Model(&models.Report{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status": req.Status, "handler_id": userID, "verdict": req.Verdict,
	})
	utils.Success(c, gin.H{"message": "已处理"})
}

// ============ 站长面板 ============

// GET /api/admin/stats — 平台统计
func GetAdminStats(c *gin.Context) {
	if !ensureAdmin(c) { return }
	var userCount, novelCount, commentCount int64
	models.DB.Model(&models.User{}).Count(&userCount)
	models.DB.Model(&models.Novel{}).Count(&novelCount)
	models.DB.Model(&models.Comment{}).Count(&commentCount)

	utils.Success(c, gin.H{
		"users": userCount, "novels": novelCount, "comments": commentCount,
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

	utils.Success(c, gin.H{
		"overview": gin.H{
			"users":    userCount,
			"novels":   novelCount,
			"comments": commentCount,
			"forums":   forumCount,
		},
		"trend_visits":         trendVisits,
		"trend_chapters":       trendChapters,
		"top_novels":           topNovels,
		"category_distribution": categoryDistribution,
	})
}

// GET /api/admin/users — 用户列表
func ListUsers(c *gin.Context) {
	if !ensureAdmin(c) { return }
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	var users []models.User
	var total int64
	models.DB.Model(&models.User{}).Count(&total)
	models.DB.Offset((page - 1) * 20).Limit(20).Order("id DESC").Find(&users)
	utils.Success(c, gin.H{"list": users, "total": total})
}

// PUT /api/admin/users/:id — 修改用户
func UpdateUser(c *gin.Context) {
	if !ensureAdmin(c) { return }
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Role *string `json:"role"`
	}
	c.ShouldBindJSON(&req)

	updates := map[string]interface{}{}
	if req.Role != nil {
		updates["role"] = *req.Role
	}
	if len(updates) > 0 {
		models.DB.Model(&models.User{}).Where("id = ?", id).Updates(updates)
	}
	utils.Success(c, gin.H{"message": "更新成功"})
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

// ============ 站长配置 ============

// GET /api/admin/config — 获取所有平台配置
func GetPlatformConfigs(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	cfgs := models.GetAllPlatformConfigs()
	utils.Success(c, cfgs)
}

// PUT /api/admin/config — 更新平台配置（站名、VIP开关等）
func UpdatePlatformConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}

	allowed := map[string]bool{
		"site_name":   true,
		"vip_enabled": true,
		"categories":  true,
	}

	for k, v := range req {
		if !allowed[k] {
			continue
		}
		if err := models.SetPlatformConfig(k, v); err != nil {
			utils.InternalError(c, "保存配置失败: "+k)
			return
		}
	}

	utils.Success(c, gin.H{"message": "配置已更新"})
}

// ============ 隔离墙配置 ============

// GET /api/admin/wall-config — 获取隔离墙配置
func GetWallConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	cfg := models.GetWallConfig()
	utils.Success(c, cfg)
}

// PUT /api/admin/wall-config — 更新隔离墙配置
func UpdateWallConfig(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req models.WallConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "参数错误")
		return
	}
	if err := models.SetWallConfig(req); err != nil {
		utils.InternalError(c, "保存配置失败")
		return
	}
	utils.Success(c, gin.H{"message": "隔离墙配置已更新"})
}

// ============ 远程站点互通 ============

// GET /api/admin/sites — 远程站点列表
func ListFederatedSites(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var sites []models.FederatedSite
	models.DB.Order("id ASC").Find(&sites)
	utils.Success(c, sites)
}

// POST /api/admin/sites — 添加远程站点
func CreateFederatedSite(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var req struct {
		Name        string `json:"name" binding:"required"`
		URL         string `json:"url" binding:"required"`
		APIURL      string `json:"api_url" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "请填写完整信息")
		return
	}

	site := &models.FederatedSite{
		Name:        req.Name,
		URL:         req.URL,
		APIURL:      req.APIURL,
		Description: req.Description,
		Status:      "active",
	}

	if err := models.DB.Create(site).Error; err != nil {
		utils.InternalError(c, "添加失败")
		return
	}

	utils.Success(c, site)
}

// PUT /api/admin/sites/:id — 更新远程站点
func UpdateFederatedSite(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var site models.FederatedSite
	if err := models.DB.First(&site, id).Error; err != nil {
		utils.NotFound(c, "站点不存在")
		return
	}

	var req struct {
		Name        *string `json:"name"`
		URL         *string `json:"url"`
		APIURL      *string `json:"api_url"`
		Description *string `json:"description"`
		Status      *string `json:"status"`
	}
	c.ShouldBindJSON(&req)

	updates := map[string]interface{}{}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.URL != nil {
		updates["url"] = *req.URL
	}
	if req.APIURL != nil {
		updates["api_url"] = *req.APIURL
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	if len(updates) > 0 {
		models.DB.Model(&site).Updates(updates)
	}

	utils.Success(c, gin.H{"message": "更新成功"})
}

// DELETE /api/admin/sites/:id — 删除远程站点
func DeleteFederatedSite(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	// 同步删除关联的缓存作品
	models.DB.Where("site_id = ?", id).Delete(&models.FederatedNovel{})
	models.DB.Delete(&models.FederatedSite{}, id)

	utils.Success(c, gin.H{"message": "已删除"})
}

// POST /api/admin/sites/:id/sync — 手动触发同步
func SyncFederatedSite(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var site models.FederatedSite
	if err := models.DB.First(&site, id).Error; err != nil {
		utils.NotFound(c, "站点不存在")
		return
	}

	// 调用远程站点 API 获取作品列表
	resp, err := http.Get(site.APIURL + "/novels?page_size=50")
	if err != nil {
		utils.InternalError(c, "连接远程站点失败: "+err.Error())
		return
	}
	defer resp.Body.Close()

	var result struct {
		Code int `json:"code"`
		Data struct {
			List []struct {
				ID       uint   `json:"id"`
				Title    string `json:"title"`
				Category string `json:"category"`
				Summary  string `json:"summary"`
				CoverURL string `json:"cover_url"`
				Author   struct {
					Nickname string `json:"nickname"`
				} `json:"author"`
			} `json:"list"`
			Total int64 `json:"total"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		utils.InternalError(c, "解析远程数据失败: "+err.Error())
		return
	}

	synced := 0
	for _, item := range result.Data.List {
		var existing models.FederatedNovel
		err := models.DB.Where("site_id = ? AND remote_id = ?", id, item.ID).First(&existing).Error
		if err == nil {
			continue // 已存在
		}

		authorName := ""
		if item.Author.Nickname != "" {
			authorName = item.Author.Nickname
		}

		fn := &models.FederatedNovel{
			SiteID:    uint(id),
			RemoteID:  item.ID,
			Title:     item.Title,
			Category:  item.Category,
			Author:    authorName,
			Summary:   item.Summary,
			CoverURL:  item.CoverURL,
			SourceURL: fmt.Sprintf("%s/novel/%d", site.URL, item.ID),
			CachedAt:  time.Now(),
		}
		models.DB.Create(fn)
		synced++
	}

	// 更新同步时间和数量
	now := time.Now()
	count := int64(0)
	models.DB.Model(&models.FederatedNovel{}).Where("site_id = ?", id).Count(&count)
	models.DB.Model(&site).Updates(map[string]interface{}{
		"last_sync_at": now,
		"novel_count":  count,
	})

	utils.Success(c, gin.H{
		"message":   "同步完成",
		"synced":    synced,
		"total":     count,
	})
}

// GET /api/federated/novels — 查看所有远程站点的作品（公开接口）
func ListFederatedNovels(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	siteID := c.Query("site_id")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var novels []models.FederatedNovel
	var total int64

	query := models.DB.Model(&models.FederatedNovel{}).Preload("Site")
	if siteID != "" {
		query = query.Where("site_id = ?", siteID)
	}

	query.Count(&total)
	query.Order("cached_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&novels)

	utils.Success(c, gin.H{
		"list":      novels,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GET /api/federated/sites — 公开的远程站点列表
func ListPublicSites(c *gin.Context) {
	var sites []models.FederatedSite
	models.DB.Where("status = ?", "active").Order("id ASC").Find(&sites)
	utils.Success(c, sites)
}

// ============ 公共 API ============

// GET /api/site-info — 获取站点公开信息（站名等，无需登录）
func GetSiteInfo(c *gin.Context) {
	utils.Success(c, gin.H{
		"site_name": models.GetSiteName(),
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