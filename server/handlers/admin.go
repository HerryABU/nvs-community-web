package handlers

import (
	"sort"
	"strconv"
	"time"

	"nvs-server/ipgeo"
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

	userID := currentUserID(c)

	user, err := models.GetUserByID(userID)
	if err != nil || (user.Role != "author" && user.Role != "vip_author") {
		utils.Forbidden(c, "需要先成为认证作者")
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	c.ShouldBindJSON(&req)

	app := &models.VipApplication{UserID: userID, Reason: req.Reason}
	models.DB.Where("user_id = ? AND status = ?", userID, "pending").FirstOrCreate(app)

	utils.Success(c, app)
}

// GET /api/admin/vip-applications — 查看 VIP 申请列表
func ListVipApplications(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
	var apps []models.VipApplication
	models.DB.Preload("User").Order("created_at DESC").Find(&apps)
	utils.Success(c, apps)
}

// POST /api/admin/vip-applications/:id/approve
func ApproveVip(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
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
	if !ensureAdmin(c) {
		return
	}
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
		"SELECT date(created_at) AS date, COUNT(*) AS novels FROM novels WHERE deleted_at IS NULL AND created_at >= ? GROUP BY date ORDER BY date",
		dates[0],
	).Scan(&dailyNovels)

	models.DB.Raw(
		"SELECT date(cm.created_at) AS date, COUNT(*) AS comments FROM comments cm JOIN novels n ON n.id = cm.novel_id "+
			"WHERE n.deleted_at IS NULL AND cm.deleted_at IS NULL AND cm.created_at >= ? GROUP BY date(cm.created_at) ORDER BY date",
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
		"SELECT date(c.created_at) AS date, COUNT(*) AS count FROM chapters c JOIN novels n ON n.id = c.novel_id "+
			"WHERE n.deleted_at IS NULL AND c.deleted_at IS NULL AND c.created_at >= ? GROUP BY date(c.created_at) ORDER BY date",
		dates[0],
	).Scan(&dailyChapters)

	chapterMap := make(map[string]int64)
	for _, r := range dailyChapters {
		chapterMap[r.Date] = r.Count
	}
	trendChapters := make([]gin.H, 0, 7)

	// 每日字数增长趋势
	type trendWordRow struct {
		Date  string
		Count int64
	}
	var dailyWords []trendWordRow
	models.DB.Raw(
		"SELECT date(c.created_at) AS date, COALESCE(SUM(c.word_count),0) AS count FROM chapters c JOIN novels n ON n.id = c.novel_id "+
			"WHERE n.deleted_at IS NULL AND c.deleted_at IS NULL AND c.created_at >= ? GROUP BY date(c.created_at) ORDER BY date",
		dates[0],
	).Scan(&dailyWords)

	wordsMap := make(map[string]int64)
	for _, r := range dailyWords {
		wordsMap[r.Date] = r.Count
	}
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
		"SELECT n.title, n.total_words, n.total_chapters, u.nickname AS author_name " +
			"FROM novels n JOIN users u ON u.id = n.author_id " +
			"WHERE n.deleted_at IS NULL " +
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
		"SELECT COALESCE(category,'未分类') AS name, COUNT(*) AS count FROM novels WHERE deleted_at IS NULL GROUP BY category ORDER BY count DESC",
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
	wordsTrendCounts := make([]int64, 7)
	cumulativeWordsCounts := make([]int64, 7)
	var cumSum int64
	for i, d := range dates {
		wordsTrendCounts[i] = wordsMap[d]
		cumSum += wordsMap[d]
		cumulativeWordsCounts[i] = cumSum
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
			"dates":  dates,
			"counts": chapterTrendCounts,
		},
		"word_count_trend": gin.H{
			"dates":      dates,
			"counts":     wordsTrendCounts,
			"cumulative": cumulativeWordsCounts,
		},
		"top_novels":            topNovels,
		"category_distribution": categoryDistribution,
		// 热力图数据（前端渲染）：省级访问热力 + 星期×小时 活跃热力
		"province_heat": computeProvinceHeat(30),
		// 境外访问按国家聚合 + 境内/境外/内网占比（中国地图与国际地图共用一个数据源）
		"country_heat": computeCountryHeat(30),
		"geo_summary":  computeGeoSummary(30),
		"active_hours": computeActiveHours(30),
		// 客户端画像（设备/OS/浏览器分布，结构化 UA 解析）
		"client": computeClientBreakdown(30),
		// 内容治理与热词（概览第二幕）：违禁词命中 + 词云 + 三类热词
		"badword":  computeBadWordOverview(30),
		"hotwords": computeHotWordOverview(7, 16),
	})
}

// computeBadWordOverview 违禁词命中概览（概览页用）。
//
// 口径说明（前端文案会同步）：
//   - blocked  近 N 天确认留痕的命中条数（BadWordFlag 行数）
//   - pending  其中待审数
//   - words    命中词频 top N，喂词云
//   - library  词库总数 / 启用数
//
// 不含"瞬时拒绝"（规则直接 block 且没留 flag 的那部分）：
// 那部分没有内容快照，统计出来也无法溯源，宁可不报。
func computeBadWordOverview(days int) gin.H {
	total, pending, confirmed, words, err := models.BadWordHitStats(days)
	if err != nil {
		return gin.H{"available": false}
	}
	libTotal, libEnabled := models.BadWordLibraryStats()
	return gin.H{
		"available":   true,
		"blocked":     total,
		"pending":     pending,
		"confirmed":   confirmed,
		"words":       words,
		"library":     libTotal,
		"lib_enabled": libEnabled,
		"days":        days,
	}
}

// computeHotWordOverview 三类热词各取 top N（概览第二幕：内容/搜索/标签）。
//
// 三类口径不同，分开取而不是混在一起：
//
//	内容 —— 评论/回帖分词（读者在聊什么）
//	搜索 —— 搜索词（读者想找什么，需求最直接）
//	标签 —— 作品标签（作者在写什么）
func computeHotWordOverview(days, limit int) gin.H {
	out := gin.H{"days": days, "limit": limit}
	if list, err := models.AggregateHotWords(days, limit); err == nil {
		out["content"] = list
	}
	if list, err := models.AggregateSearchWords(days, limit); err == nil {
		out["search"] = list
	}
	if list, err := models.AggregateTagHotWords(days, limit); err == nil {
		out["tag"] = list
	}
	return out
}

// computeCountryHeat 站点访问的「国家/地区热力」。
//
// 为什么需要：省级热度只覆盖中国，境外访问此前被 `continue` 直接跳过，
// 于是「国际」这张图没有数据源。这里按国家聚合（ip2region 境外精度只到国家，
// 够做国家排行），中国单列（其明细由 province_heat 负责）。
//
// 返回 [{code:'US', name:'United States', value:N, cn:false}]，按访问量降序。
func computeCountryHeat(days int) []gin.H {
	if !ipgeo.Available() {
		return []gin.H{}
	}
	type ipRow struct {
		IP  string
		Cnt int64
	}
	var rows []ipRow
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")
	if err := models.DB.Model(&models.WebAccessLog{}).
		Where("created_at >= ?", since).
		Select("ip, COUNT(*) as cnt").
		Group("ip").Order("cnt desc").Limit(5000).
		Scan(&rows).Error; err != nil {
		return []gin.H{}
	}

	type agg struct {
		name string
		cn   bool
	}
	counts := map[string]int64{}
	meta := map[string]agg{}
	for _, r := range rows {
		reg := ipgeo.Lookup(r.IP)
		if reg.IsReserved() {
			continue
		}
		code := reg.CountryCD
		if code == "" || code == "0" {
			code = reg.Country // 没有国家码时退化为国名做 key
		}
		if code == "" {
			continue
		}
		counts[code] += r.Cnt
		if _, ok := meta[code]; !ok {
			meta[code] = agg{name: reg.Country, cn: reg.Country == "中国"}
		}
	}

	out := make([]gin.H, 0, len(counts))
	for code, v := range counts {
		m := meta[code]
		out = append(out, gin.H{
			"code":  code,
			"name":  m.name,
			"value": v,
			"cn":    m.cn,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i]["value"].(int64) > out[j]["value"].(int64)
	})
	return out
}

// computeGeoSummary 境内 / 境外 / 内网（保留地址）的访问占比。
//
// 有了这三个数，站长一眼能看出「我的读者主要在哪」——
// 而不必先看图。内网单独列出是刻意的：本机/内网调试流量不该混进地域统计。
func computeGeoSummary(days int) gin.H {
	if !ipgeo.Available() {
		return gin.H{"available": false}
	}
	type ipRow struct {
		IP  string
		Cnt int64
	}
	var rows []ipRow
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")
	if err := models.DB.Model(&models.WebAccessLog{}).
		Where("created_at >= ?", since).
		Select("ip, COUNT(*) as cnt").
		Group("ip").Order("cnt desc").Limit(5000).
		Scan(&rows).Error; err != nil {
		return gin.H{"available": false}
	}
	var cn, overseas, reserved int64
	for _, r := range rows {
		reg := ipgeo.Lookup(r.IP)
		switch {
		case reg.IsReserved():
			reserved += r.Cnt
		case reg.Country == "中国":
			cn += r.Cnt
		default:
			overseas += r.Cnt
		}
	}
	total := cn + overseas + reserved
	pct := func(v int64) float64 {
		if total == 0 {
			return 0
		}
		return float64(int64(float64(v)/float64(total)*1000+0.5)) / 10
	}
	return gin.H{
		"available": true,
		"cn":        cn,
		"overseas":  overseas,
		"reserved":  reserved,
		"total":     total,
		"cn_pct":    pct(cn),
		"os_pct":    pct(overseas),
		"days":      days,
	}
}

// computeProvinceHeat 站点访问的「省级热力」（复用 ip2region 离线解析，与 GetWebAccessGeo 同口径）。
// 先按 IP 聚合再查表，解析成本与 IP 数成正比而非日志条数；内网/保留地址与境外不计入省份排名。
// 返回 [{name: 省全称(如"广东省"), value: 访问次数}]，直接喂给前端 ChinaGeoMap.vue 的 visualMap 着色。
func computeProvinceHeat(days int) []gin.H {
	if !ipgeo.Available() {
		return []gin.H{}
	}
	type ipRow struct {
		IP  string
		Cnt int64
	}
	var rows []ipRow
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")
	if err := models.DB.Model(&models.WebAccessLog{}).
		Where("created_at >= ?", since).
		Select("ip, COUNT(*) as cnt").
		Group("ip").Order("cnt desc").Limit(5000).
		Scan(&rows).Error; err != nil {
		return []gin.H{}
	}
	provinces := map[string]int64{}
	for _, r := range rows {
		reg := ipgeo.Lookup(r.IP)
		if reg.IsReserved() || reg.Country == "" {
			continue
		}
		if reg.Country == "中国" {
			if prov := ipgeo.NormalizeProvince(reg.Province); prov != "" {
				provinces[prov] += r.Cnt
			}
		}
	}
	out := make([]gin.H, 0, len(provinces))
	for k, v := range provinces {
		out = append(out, gin.H{"name": k, "value": v})
	}
	return out
}

// computeActiveHours 近 days 天「星期×小时」用户互动活跃热力（0=周日..6=周六，0..23 时）。
// 数据来源：评论创建时间（用户互动的强代理）。返回 7×24 矩阵，前端 ECharts heatmap 直接消费。
func computeActiveHours(days int) [][]int64 {
	matrix := make([][]int64, 7)
	for i := range matrix {
		matrix[i] = make([]int64, 24)
	}
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")
	type ahRow struct {
		Wd  int
		Hr  int
		Cnt int64
	}
	var rows []ahRow
	models.DB.Raw(
		"SELECT CAST(strftime('%w', created_at) AS INTEGER) AS wd, "+
			"CAST(strftime('%H', created_at) AS INTEGER) AS hr, COUNT(*) AS cnt "+
			"FROM comments WHERE deleted_at IS NULL AND created_at >= ? GROUP BY wd, hr",
		since,
	).Scan(&rows)
	for _, r := range rows {
		if r.Wd >= 0 && r.Wd < 7 && r.Hr >= 0 && r.Hr < 24 {
			matrix[r.Wd][r.Hr] = r.Cnt
		}
	}
	return matrix
}

// computeClientBreakdown 近 days 天「客户端画像」分布（结构化 UA 解析结果）。
// 返回 {os:[{name,count}], browser:[...], device:[...]}，供前端「客户端分布」卡片渲染。
func computeClientBreakdown(days int) gin.H {
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")
	var osDist, browserDist, deviceDist []map[string]any
	models.DB.Model(&models.WebAccessLog{}).
		Where("created_at >= ? AND os_name <> '' AND os_name <> 'unknown'", since).
		Select("os_name as name, COUNT(*) as cnt").Group("os_name").Order("cnt desc").Limit(8).
		Scan(&osDist)
	models.DB.Model(&models.WebAccessLog{}).
		Where("created_at >= ? AND browser_name <> '' AND browser_name <> 'unknown'", since).
		Select("browser_name as name, COUNT(*) as cnt").Group("browser_name").Order("cnt desc").Limit(8).
		Scan(&browserDist)
	models.DB.Model(&models.WebAccessLog{}).
		Where("created_at >= ? AND device_type <> '' AND device_type <> 'unknown'", since).
		Select("device_type as name, COUNT(*) as cnt").Group("device_type").Order("cnt desc").
		Scan(&deviceDist)
	return gin.H{
		"os":      osDist,
		"browser": browserDist,
		"device":  deviceDist,
	}
}

// ============ 财务 ============

// POST /api/author/withdraw — 申请提现
func RequestWithdraw(c *gin.Context) {
	userID := currentUserID(c)
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
	userID := currentUserID(c)
	var total float64
	models.DB.Model(&models.EarningsRecord{}).Where("user_id = ?", userID).Select("COALESCE(SUM(amount),0)").Scan(&total)
	utils.Success(c, gin.H{"total_earnings": total})
}

// GET /api/admin/finance — 财务总览
func GetFinanceOverview(c *gin.Context) {
	if !ensureAdmin(c) {
		return
	}
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
	userID := currentUserID(c)
	user, _ := models.GetUserByID(userID)
	if user == nil || user.Role != "admin" {
		utils.Forbidden(c, "需要管理员权限")
		c.Abort()
		return false
	}
	return true
}

// isAdminUser 非阻断式判断当前用户是否为管理员
func isAdminUser(c *gin.Context) bool {
	userID := currentUserID(c)
	if userID == 0 {
		return false
	}
	user, _ := models.GetUserByID(userID)
	return user != nil && user.Role == "admin"
}

// isPoliceOrAdminUser 非阻断式判断当前用户是否具备「警方取管台 / 管理员」权限。
// 用于前台取证豁免：警方可越过房间号/密码门禁查看封锁、加密、隐藏论坛，
// 并在评论区看到红字标注的已删除内容。与 isAdminUser 一致走 DB 角色判定，
// 即使上下文仅写入 userID 也能正确识别。
func isPoliceOrAdminUser(c *gin.Context) bool {
	userID := currentUserID(c)
	if userID == 0 {
		return false
	}
	user, _ := models.GetUserByID(userID)
	return user != nil && models.IsAdminOrPolice(user.Role)
}
