package handlers

import (
	"fmt"
	"strconv"
	"time"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)

// GetMyNovels 获取我的作品列表
func GetMyNovels(c *gin.Context) {
	userID := c.GetUint("userID")
	novels, _, err := models.GetNovelsByAuthor(userID, 1, 1000)
	if err != nil {
		utils.InternalError(c, "获取作品列表失败")
		return
	}
	utils.Success(c, novels)
}

// GetAuthorProfile 获取作者公开主页信息（包括作品列表和统计数据）
func GetAuthorProfile(c *gin.Context) {
	authorID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	user, err := models.GetUserByID(uint(authorID))
	if err != nil {
		utils.NotFound(c, "作者不存在")
		return
	}

	// 隐藏敏感信息
	user.PasswordHash = ""
	user.Email = ""

	novels, totalNovels, _ := models.GetNovelsByAuthor(uint(authorID), 1, 50)

	totalWords := 0
	totalChapters := 0
	for _, n := range novels {
		totalWords += n.TotalWords
		totalChapters += n.TotalChapters
	}

	// 收集作者所有作品的ID，用于统计
	var novelIDs []uint
	for _, n := range novels {
		novelIDs = append(novelIDs, n.ID)
	}

	var totalComments int64
	if len(novelIDs) > 0 {
		models.DB.Model(&models.Comment{}).Where("novel_id IN ?", novelIDs).Count(&totalComments)
	}

	// 最近7天章节增长趋势
	now := time.Now()
	dates := make([]string, 7)
	for i := 0; i < 7; i++ {
		dates[6-i] = now.AddDate(0, 0, -i).Format("2006-01-02")
	}

	type dayCount struct {
		Date  string
		Count int64
	}
	var dailyChapters []dayCount
	if len(novelIDs) > 0 {
		models.DB.Raw(
			"SELECT date(chapters.created_at) AS date, COUNT(*) AS count "+
				"FROM chapters JOIN novels ON novels.id = chapters.novel_id "+
				"WHERE novels.author_id = ? AND chapters.created_at >= ? "+
				"GROUP BY date(chapters.created_at) ORDER BY date",
			authorID, dates[0],
		).Scan(&dailyChapters)
	}
	chapterMap := make(map[string]int64)
	for _, r := range dailyChapters {
		chapterMap[r.Date] = r.Count
	}
	chapterTrendCounts := make([]int64, 7)
	for i, d := range dates {
		chapterTrendCounts[i] = chapterMap[d]
	}

	// 最近7天评论趋势
	var dailyComments []dayCount
	if len(novelIDs) > 0 {
		models.DB.Raw(
			"SELECT date(comments.created_at) AS date, COUNT(*) AS count "+
				"FROM comments JOIN novels ON novels.id = comments.novel_id "+
				"WHERE novels.author_id = ? AND comments.created_at >= ? "+
				"GROUP BY date(comments.created_at) ORDER BY date",
			authorID, dates[0],
		).Scan(&dailyComments)
	}
	commentMap := make(map[string]int64)
	for _, r := range dailyComments {
		commentMap[r.Date] = r.Count
	}
	commentTrendCounts := make([]int64, 7)
	for i, d := range dates {
		commentTrendCounts[i] = commentMap[d]
	}

	// 计算平均评分
	var avgRating float64
	if len(novelIDs) > 0 {
		models.DB.Raw(
			"SELECT COALESCE(AVG((type_completion + narrative_quality + thought_depth + community_reputation + update_stability) / 5.0), 0) "+
				"FROM ratings JOIN novels ON novels.id = ratings.novel_id "+
				"WHERE novels.author_id = ?",
			authorID,
		).Scan(&avgRating)
	}

	utils.Success(c, gin.H{
		"author":          user,
		"novels":          novels,
		"total_novels":    totalNovels,
		"total_words":     totalWords,
		"total_chapters":  totalChapters,
		"total_comments":  totalComments,
		"avg_rating":      avgRating,
		"chapter_trend": gin.H{
			"dates": dates,
			"counts": chapterTrendCounts,
		},
		"comment_trend": gin.H{
			"dates": dates,
			"counts": commentTrendCounts,
		},
	})
}

// GetAuthorForum 获取作者的专属讨论区（自动创建）
func GetAuthorForum(c *gin.Context) {
	authorID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	user, err := models.GetUserByID(uint(authorID))
	if err != nil {
		utils.NotFound(c, "作者不存在")
		return
	}

	authorName := user.Nickname
	if authorName == "" {
		authorName = user.Username
	}

	refID := fmt.Sprintf("author_%d", authorID)
	forum, err := models.GetOrCreateForum(
		authorName+" 的讨论区",
		"author_sub",
		refID,
		"作者 "+authorName+" 的专属讨论区，读者可以在此与作者交流。",
	)
	if err != nil {
		utils.InternalError(c, "获取/创建论坛失败")
		return
	}

	threads, total, _ := models.GetThreadsByForum(forum.ID, 1, 20)

	utils.Success(c, gin.H{
		"forum":   forum,
		"threads": threads,
		"total":   total,
	})
}

// GetNovelStats 获取作品统计
func GetNovelStats(c *gin.Context) {
	userID := c.GetUint("userID")
	novelID, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	novel, err := models.GetNovelByID(uint(novelID))
	if err != nil || novel.AuthorID != userID {
		utils.NotFound(c, "作品不存在")
		return
	}

	stats := map[string]interface{}{
		"total_words":    novel.TotalWords,
		"total_chapters": novel.TotalChapters,
		"status":         novel.Status,
		"created_at":     novel.CreatedAt,
		"updated_at":     novel.UpdatedAt,
	}

	utils.Success(c, stats)
}

// ============ 作者数据大屏 ============

// GET /api/author/dashboard — 作者端数据大屏
func GetAuthorDashboard(c *gin.Context) {
	userID := c.GetUint("userID")

	// --- 获取该作者所有作品ID ---
	var novelIDs []uint
	models.DB.Model(&models.Novel{}).Where("author_id = ?", userID).Pluck("id", &novelIDs)

	// --- overview ---
	var totalNovels, totalWords, totalChapters, totalComments int64
	models.DB.Model(&models.Novel{}).Where("author_id = ?", userID).Count(&totalNovels)
	models.DB.Model(&models.Novel{}).Where("author_id = ?", userID).
		Select("COALESCE(SUM(total_words),0)").Scan(&totalWords)
	models.DB.Model(&models.Novel{}).Where("author_id = ?", userID).
		Select("COALESCE(SUM(total_chapters),0)").Scan(&totalChapters)
	if len(novelIDs) > 0 {
		models.DB.Model(&models.Comment{}).Where("novel_id IN ?", novelIDs).Count(&totalComments)
	}

	// --- 最近7天日期列表 ---
	now := time.Now()
	dates := make([]string, 7)
	for i := 0; i < 7; i++ {
		dates[6-i] = now.AddDate(0, 0, -i).Format("2006-01-02")
	}

	// --- trend_chapters: 该作者最近7天每天新增章节 ---
	type dayCount struct {
		Date  string
		Count int64
	}
	var dailyChapters []dayCount
	if len(novelIDs) > 0 {
		models.DB.Raw(
			"SELECT date(chapters.created_at) AS date, COUNT(*) AS count "+
				"FROM chapters JOIN novels ON novels.id = chapters.novel_id "+
				"WHERE novels.author_id = ? AND chapters.created_at >= ? "+
				"GROUP BY date(chapters.created_at) ORDER BY date",
			userID, dates[0],
		).Scan(&dailyChapters)
	}

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

	// --- trend_comments: 该作者最近7天每天新增评论 ---
	var dailyComments []dayCount
	if len(novelIDs) > 0 {
		models.DB.Raw(
			"SELECT date(comments.created_at) AS date, COUNT(*) AS count "+
				"FROM comments JOIN novels ON novels.id = comments.novel_id "+
				"WHERE novels.author_id = ? AND comments.created_at >= ? "+
				"GROUP BY date(comments.created_at) ORDER BY date",
			userID, dates[0],
		).Scan(&dailyComments)
	}

	commentMap := make(map[string]int64)
	for _, r := range dailyComments {
		commentMap[r.Date] = r.Count
	}
	trendComments := make([]gin.H, 0, 7)
	for _, d := range dates {
		trendComments = append(trendComments, gin.H{
			"date":  d,
			"count": commentMap[d],
		})
	}

	// --- novel_stats: 该作者每部作品的字数+章节 ---
	type novelStatRow struct {
		Title         string
		TotalWords    int
		TotalChapters int
		Status        string
	}
	var novelStatsRaw []novelStatRow
	models.DB.Raw(
		"SELECT title, total_words, total_chapters, status FROM novels WHERE author_id = ? ORDER BY updated_at DESC",
		userID,
	).Scan(&novelStatsRaw)

	novelStats := make([]gin.H, 0, len(novelStatsRaw))
	var completedCount int64
	for _, r := range novelStatsRaw {
		novelStats = append(novelStats, gin.H{
			"title":          r.Title,
			"total_words":    r.TotalWords,
			"total_chapters": r.TotalChapters,
			"status":         r.Status,
		})
		if r.Status == "published" || r.Status == "completed" {
			completedCount++
		}
	}

	// --- completion_rate: 完本率 ---
	var completionRate float64
	if totalNovels > 0 {
		completionRate = float64(completedCount) / float64(totalNovels)
	}

	// --- avg_rating: 平均评分 ---
	var avgRating float64
	if len(novelIDs) > 0 {
		models.DB.Raw(
			"SELECT COALESCE(AVG((type_completion + narrative_quality + thought_depth + community_reputation + update_stability) / 5.0), 0) "+
				"FROM ratings JOIN novels ON novels.id = ratings.novel_id "+
				"WHERE novels.author_id = ?",
			userID,
		).Scan(&avgRating)
	}

	// 构建前端兼容的趋势格式: chapter_trend {dates, counts}, comment_trend {dates, counts}
	chapterTrendCounts := make([]int64, 7)
	for i, d := range dates {
		chapterTrendCounts[i] = chapterMap[d]
	}
	commentTrendCounts := make([]int64, 7)
	for i, d := range dates {
		commentTrendCounts[i] = commentMap[d]
	}

	utils.Success(c, gin.H{
		"stats": gin.H{
			"novels":         totalNovels,
			"total_words":    totalWords,
			"total_chapters": totalChapters,
			"total_comments": totalComments,
		},
		"trend_chapters": trendChapters,
		"trend_comments": trendComments,
		"chapter_trend": gin.H{
			"dates": dates,
			"counts": chapterTrendCounts,
		},
		"comment_trend": gin.H{
			"dates": dates,
			"counts": commentTrendCounts,
		},
		"novel_stats":     novelStats,
		"completion_rate": completionRate,
		"avg_rating":      avgRating,
	})
}
