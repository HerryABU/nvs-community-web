package blog

import (
	"strconv"
	"strings"

	"nvs-server/models"
)

// Models 返回 blog 域全部需要迁移的模型；根包 AllModels 在运行时聚合。
func Models() []interface{} {
	return []interface{}{
		&AuthorBlog{}, &BlogCategory{}, &BlogLike{}, &BlogFavorite{},
		&BlogTag{}, &BlogTagLink{}, &BlogTopic{}, &BlogTopicLink{},
	}
}

// NotifyBlogComment 新评论触达博客作者时通知（由根包 CreateComment 通过 BlogCommentNotifier 钩子调用）。
func NotifyBlogComment(actorID, blogAuthorID uint, blogTitle, link string) error {
	return models.PushNotification(actorID, blogAuthorID, "blog_comment",
		"你的博客收到新评论", "《"+blogTitle+"》", link)
}

// SpecialFeedBlogs 特别关注动态流中的博客部分（由根包 GetSpecialFeed 通过 BlogFeedProvider 钩子调用）。
func SpecialFeedBlogs(ids []uint, window int) ([]models.FeedItem, int64, error) {
	var blogTotal int64
	models.DB.Model(&AuthorBlog{}).Where("author_id IN ?", ids).Count(&blogTotal)
	var blogs []AuthorBlog
	if err := models.DB.Where("author_id IN ?", ids).
		Order("created_at DESC").Limit(window).Find(&blogs).Error; err != nil {
		return nil, 0, err
	}
	items := make([]models.FeedItem, 0, len(blogs))
	for _, b := range blogs {
		items = append(items, models.FeedItem{
			Type:      "blog",
			ID:        b.ID,
			Title:     b.Title,
			Summary:   clampSummary(b.Summary),
			UserID:    b.AuthorID,
			CreatedAt: b.CreatedAt,
			URL:       feedURL("blog", b.AuthorID, b.ID),
		})
	}
	return items, blogTotal, nil
}

// clampSummary / feedURL 与根包 follow_special.go 同源，这里复刻一份，避免根包回引域子包造成导入循环。
func clampSummary(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	const maxRunes = 80
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "…"
}

func feedURL(typ string, userID, id uint) string {
	switch typ {
	case "novel":
		return "/author/" + strconv.FormatUint(uint64(userID), 10) + "/novel/" + strconv.FormatUint(uint64(id), 10)
	case "blog":
		return "/author/" + strconv.FormatUint(uint64(userID), 10) + "/blogs/" + strconv.FormatUint(uint64(id), 10)
	case "thread":
		return "/thread/" + strconv.FormatUint(uint64(id), 10)
	}
	return "/"
}

// init 把 blog 域的模型与跨域钩子登记到根包，根包据此聚合，无需回引本子包。
func init() {
	models.RegisterDomainModels(Models)
	models.BlogCommentNotifier = NotifyBlogComment
	models.BlogFeedProvider = SpecialFeedBlogs
}
