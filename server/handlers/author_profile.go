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

	// 隐藏敏感信息但保留 author_id
	user.PasswordHash = ""
	user.Email = ""

	novels, totalNovels, _ := models.GetNovelsByAuthor(uint(authorID), 1, 50)

	totalWords := 0
	totalChapters := 0
	for _, n := range novels {
		totalWords += n.TotalWords
		totalChapters += n.TotalChapters
	}

	// 收集作者所有作品ID，用于统计
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
		"author_id":       user.ID,
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