package handlers

import (
	"time"

	"nvs-server/models"
	"nvs-server/utils"

	"github.com/gin-gonic/gin"
)


// GET /api/author/dashboard — 作者端数据大屏
func GetAuthorDashboard(c *gin.Context) {
	userID := c.GetUint("userID")

	var novelIDs []uint
	models.DB.Model(&models.Novel{}).Where("author_id = ?", userID).Pluck("id", &novelIDs)

	var totalNovels, totalWords, totalChapters, totalComments int64
	models.DB.Model(&models.Novel{}).Where("author_id = ?", userID).Count(&totalNovels)
	models.DB.Model(&models.Novel{}).Where("author_id = ?", userID).Select("COALESCE(SUM(total_words),0)").Scan(&totalWords)
	models.DB.Model(&models.Novel{}).Where("author_id = ?", userID).Select("COALESCE(SUM(total_chapters),0)").Scan(&totalChapters)
	if len(novelIDs) > 0 {
		models.DB.Model(&models.Comment{}).Where("novel_id IN ?", novelIDs).Count(&totalComments)
	}

	now := time.Now()
	dates := make([]string, 7)
	for i := 0; i < 7; i++ {
		dates[6-i] = now.AddDate(0, 0, -i).Format("2006-01-02")
	}

	type dayCount struct{ Date string; Count int64 }
	var dailyChapters []dayCount
	if len(novelIDs) > 0 {
		models.DB.Raw("SELECT date(chapters.created_at) AS date, COUNT(*) AS count FROM chapters JOIN novels ON novels.id = chapters.novel_id WHERE novels.author_id = ? AND chapters.created_at >= ? GROUP BY date(chapters.created_at) ORDER BY date", userID, dates[0]).Scan(&dailyChapters)
	}
	chapterMap := make(map[string]int64)
	for _, r := range dailyChapters { chapterMap[r.Date] = r.Count }
	trendChapters := make([]gin.H, 0, 7)
	for _, d := range dates { trendChapters = append(trendChapters, gin.H{"date": d, "count": chapterMap[d]}) }

	var dailyComments []dayCount
	if len(novelIDs) > 0 {
		models.DB.Raw("SELECT date(comments.created_at) AS date, COUNT(*) AS count FROM comments JOIN novels ON novels.id = comments.novel_id WHERE novels.author_id = ? AND comments.created_at >= ? GROUP BY date(comments.created_at) ORDER BY date", userID, dates[0]).Scan(&dailyComments)
	}
	commentMap := make(map[string]int64)
	for _, r := range dailyComments { commentMap[r.Date] = r.Count }
	trendComments := make([]gin.H, 0, 7)
	for _, d := range dates { trendComments = append(trendComments, gin.H{"date": d, "count": commentMap[d]}) }

	type novelStatRow struct{ Title string; TotalWords int; TotalChapters int; Status string }
	var novelStatsRaw []novelStatRow
	models.DB.Raw("SELECT title, total_words, total_chapters, status FROM novels WHERE author_id = ? ORDER BY updated_at DESC", userID).Scan(&novelStatsRaw)
	novelStats := make([]gin.H, 0, len(novelStatsRaw))
	for _, r := range novelStatsRaw {
		novelStats = append(novelStats, gin.H{"title": r.Title, "total_words": r.TotalWords, "total_chapters": r.TotalChapters, "status": r.Status})
	}

	// 总收藏数 + 总阅读量（替代完本率）
	var bookshelfCount, totalViews int64
	if len(novelIDs) > 0 {
		models.DB.Model(&models.BookShelf{}).Where("novel_id IN ?", novelIDs).Count(&bookshelfCount)
		models.DB.Model(&models.Novel{}).Where("id IN ?", novelIDs).Select("COALESCE(SUM(view_count),0)").Scan(&totalViews)
	}

	var avgRating float64
	if len(novelIDs) > 0 {
		models.DB.Raw("SELECT COALESCE(AVG((type_completion + narrative_quality + thought_depth + community_reputation + update_stability) / 5.0), 0) FROM ratings JOIN novels ON novels.id = ratings.novel_id WHERE novels.author_id = ?", userID).Scan(&avgRating)
	}

	chapterTrendCounts := make([]int64, 7)
	for i, d := range dates { chapterTrendCounts[i] = chapterMap[d] }
	commentTrendCounts := make([]int64, 7)
	for i, d := range dates { commentTrendCounts[i] = commentMap[d] }

	utils.Success(c, gin.H{
		"stats": gin.H{"novels": totalNovels, "total_words": totalWords, "total_chapters": totalChapters, "total_comments": totalComments},
		"trend_chapters": trendChapters, "trend_comments": trendComments,
		"bookshelf_count": bookshelfCount, "total_views": totalViews,
		"chapter_trend":   gin.H{"dates": dates, "counts": chapterTrendCounts},
		"comment_trend":   gin.H{"dates": dates, "counts": commentTrendCounts},
		"novel_stats":     novelStats, "avg_rating": avgRating,
	})
}
