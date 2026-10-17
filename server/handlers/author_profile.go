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
	userID := currentUserID(c)
	novels, _, err := models.GetNovelsByAuthor(userID, 1, 1000)
	if err != nil {
		utils.InternalError(c, "获取作品列表失败")
		return
	}
	utils.Success(c, novels)
}

// ResolveUserKey 解析用户标识（数字主键 id 或 username），返回用户。
// 规则：纯数字 → 按 id 查；其余 → 按 username 查（大小写不敏感）。
func ResolveUserKey(key string) (*models.User, error) {
	if key == "" {
		return nil, fmt.Errorf("用户标识为空")
	}
	if id, err := strconv.ParseUint(key, 10, 64); err == nil {
		return models.GetUserByID(uint(id))
	}
	var u models.User
	if err := models.DB.Where("LOWER(username) = LOWER(?)", key).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// ResolveUserByKey GET /api/users/resolve?key=xxx — 轻量解析用户标识（id 或用户名）
// 供前端把 URL 参数（可能是 username）解析为数字主键，避免拉取完整主页。
func ResolveUserByKey(c *gin.Context) {
	key := c.Query("key")
	u, err := ResolveUserKey(key)
	if err != nil || u == nil {
		utils.NotFound(c, "用户不存在")
		return
	}
	utils.Success(c, gin.H{"id": u.ID, "username": u.Username, "nickname": u.Nickname, "role": u.Role})
}

// GetAuthorProfile 获取作者公开主页信息（包括作品列表和统计数据）
// :id 参数兼容：数字主键 id 或 用户名（username）
func GetAuthorProfile(c *gin.Context) {
	user, err := ResolveUserKey(c.Param("id"))
	if err != nil || user == nil {
		utils.NotFound(c, "作者不存在")
		return
	}
	authorID := user.ID

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
				"WHERE novels.author_id = ? AND novels.deleted_at IS NULL AND chapters.deleted_at IS NULL AND chapters.created_at >= ? "+
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
				"WHERE novels.author_id = ? AND novels.deleted_at IS NULL AND comments.deleted_at IS NULL AND comments.created_at >= ? "+
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
				"WHERE novels.author_id = ? AND novels.deleted_at IS NULL",
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
// :id 兼容数字主键 id 与用户名（username）
func GetAuthorForum(c *gin.Context) {
	user, err := ResolveUserKey(c.Param("id"))
	if err != nil || user == nil {
		utils.NotFound(c, "作者不存在")
		return
	}
	authorID := user.ID

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

	// 黑名单 hide 模式（F13）：不看被自己屏蔽（hide）的作者（含其讨论区内他人代发内容不滤，仅滤作者本人所发）
	threads = FilterHiddenItems(currentUserID(c), threads, func(t models.Thread) uint { return t.UserID })

	utils.Success(c, gin.H{
		"forum":   forum,
		"threads": threads,
		"total":   total,
	})
}

// GetNovelStats 获取作品统计
func GetNovelStats(c *gin.Context) {
	userID := currentUserID(c)
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

// GetAuthorBannedContent 获取作者被封禁（176号令）的小说/章节及封禁原因。
// 采用 Unscoped 查询软删除记录，作者可看到「被封禁的内容 + 原因 + 时间」，满足合规告知要求。
// 注意：仅当 ban_reason 非空才视为「管理员封禁」——作者自己删除的记录（软删除但无封禁原因）
// 不应展示 176 号令封禁横幅，避免误报。
func GetAuthorBannedContent(c *gin.Context) {
	userID := currentUserID(c)

	type bannedNovel struct {
		ID        uint       `json:"id"`
		Title     string     `json:"title"`
		BanReason string     `json:"ban_reason"`
		BannedAt  *time.Time `json:"banned_at"`
	}
	novels := make([]bannedNovel, 0)
	models.DB.Unscoped().Model(&models.Novel{}).
		Select("id, title, ban_reason, deleted_at AS banned_at").
		Where("author_id = ? AND deleted_at IS NOT NULL AND ban_reason IS NOT NULL AND ban_reason <> ''", userID).
		Order("deleted_at DESC").
		Scan(&novels)

	type bannedChapter struct {
		ID            uint       `json:"id"`
		NovelID       uint       `json:"novel_id"`
		NovelTitle    string     `json:"novel_title"`
		ChapterNumber int        `json:"chapter_number"`
		Title         string     `json:"title"`
		BanReason     string     `json:"ban_reason"`
		BannedAt      *time.Time `json:"banned_at"`
	}
	chapters := make([]bannedChapter, 0)
	models.DB.Unscoped().Model(&models.Chapter{}).
		Select("chapters.id, chapters.novel_id, chapters.chapter_number, chapters.title, chapters.ban_reason, chapters.deleted_at AS banned_at, novels.title AS novel_title").
		Joins("JOIN novels ON novels.id = chapters.novel_id").
		Where("novels.author_id = ? AND chapters.deleted_at IS NOT NULL AND chapters.ban_reason IS NOT NULL AND chapters.ban_reason <> ''", userID).
		Order("chapters.deleted_at DESC").
		Scan(&chapters)

	utils.Success(c, gin.H{"novels": novels, "chapters": chapters})
}