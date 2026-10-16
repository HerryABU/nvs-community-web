package models

import (
	"strconv"
	"strings"
	"time"
)

// ==================== 特别关注（F14） ====================
//
// 语义：特别关注 = 关注关系的加强态。特别关注列表是动态流的数据源；
// 「取消特别关注」只把 is_special 置 false，普通关注关系保留。

// FeedItem 动态流统一条目（聚合 novels / author_blogs / threads 三类内容）
type FeedItem struct {
	Type      string    `json:"type"`       // novel / blog / thread
	ID        uint      `json:"id"`         // 内容 ID
	Title     string    `json:"title"`      // 标题
	Summary   string    `json:"summary"`    // 摘要（长文本截断）
	UserID    uint      `json:"user_id"`    // 作者/发帖人 ID
	UserName  string    `json:"user_name"`  // 作者/发帖人展示名
	CreatedAt time.Time `json:"created_at"` // 发布时间（倒序依据）
	URL       string    `json:"url"`        // 前端路由路径
}

// SetSpecialFollow 设置/取消特别关注。
// special=true：确保关注行存在（不存在则创建），并把 is_special 置 true；
// special=false：把 is_special 置 false（保留普通关注行；未关注则无操作）。
// 布尔零值会被 GORM struct 写入跳过，故一律用 map 精确写列。
func SetSpecialFollow(followerID, followedID uint, special bool) error {
	if followerID == 0 || followedID == 0 {
		return nil
	}
	// 先确保关注行存在（特别关注的前提是已关注）
	var existing Follow
	err := DB.Where("follower_id = ? AND followed_id = ?", followerID, followedID).First(&existing).Error
	if err != nil {
		if !special {
			// 未关注时取消特别关注 = 无操作（不得顺手建出普通关注行）
			return nil
		}
		if err := DB.Create(&Follow{FollowerID: followerID, FollowedID: followedID}).Error; err != nil {
			return err
		}
	}
	return DB.Model(&Follow{}).
		Where("follower_id = ? AND followed_id = ?", followerID, followedID).
		Updates(map[string]interface{}{"is_special": special}).Error
}

// IsSpecialFollowing 是否特别关注了某人
func IsSpecialFollowing(followerID, followedID uint) bool {
	if followerID == 0 || followedID == 0 {
		return false
	}
	var count int64
	DB.Model(&Follow{}).
		Where("follower_id = ? AND followed_id = ? AND is_special = ?", followerID, followedID, true).
		Count(&count)
	return count > 0
}

// ListSpecialFollowing 我的特别关注列表（JOIN users，按关注时间倒序）
func ListSpecialFollowing(userID uint, page, pageSize int) ([]User, int64, error) {
	var total int64
	DB.Model(&Follow{}).Where("follower_id = ? AND is_special = ?", userID, true).Count(&total)

	page, pageSize = normalizePage(page, pageSize)
	offset := (page - 1) * pageSize
	var users []User
	err := DB.Table("users").
		Joins("INNER JOIN follows ON follows.followed_id = users.id").
		Where("follows.follower_id = ? AND follows.is_special = ?", userID, true).
		Order("follows.created_at DESC").
		Offset(offset).Limit(pageSize).
		Find(&users).Error
	return users, total, err
}

// CountSpecialFollowing 特别关注人数
func CountSpecialFollowing(userID uint) int64 {
	var count int64
	DB.Model(&Follow{}).Where("follower_id = ? AND is_special = ?", userID, true).Count(&count)
	return count
}

// specialFollowingIDs 特别关注的全部用户 ID（动态流数据源）。
// 特别关注是用户主动精选的少量对象，一次性取全可接受；
// 若未来放开规模，可改为在 feed 聚合时用子查询下推。
func specialFollowingIDs(userID uint) []uint {
	var ids []uint
	DB.Model(&Follow{}).
		Where("follower_id = ? AND is_special = ?", userID, true).
		Pluck("followed_id", &ids)
	return ids
}

// GetSpecialFeed 聚合特别关注用户的最新动态，按 created_at 倒序分页。
//
// 实现取舍：UNION 查询在 SQLite/MySQL 间的字段类型/排序兼容性坑较多
// （如时间列精度、LIMIT 语义差异），故采用「分查合并」：三类内容各查
// 前 page*pageSize 条（足够覆盖本页所需的合并窗口），在内存中按
// created_at 倒序归并后截取本页。总数 = 三类计数之和（精确值）。
func GetSpecialFeed(userID uint, page, pageSize int) ([]FeedItem, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	ids := specialFollowingIDs(userID)
	if len(ids) == 0 {
		return []FeedItem{}, 0, nil
	}

	// 合并窗口：本页之前 + 本页所需条数，保证截取后本页数据完整
	window := page * pageSize

	// 总数 = 三类计数之和（精确值，分页依赖它）
	var novelTotal, blogTotal, threadTotal int64
	DB.Model(&Novel{}).Where("author_id IN ? AND status = ?", ids, "published").Count(&novelTotal)
	DB.Model(&AuthorBlog{}).Where("author_id IN ?", ids).Count(&blogTotal)
	DB.Model(&Thread{}).Where("user_id IN ?", ids).Count(&threadTotal)
	total := novelTotal + blogTotal + threadTotal

	items := make([]FeedItem, 0, window)

	// 小说（仅已发布；软删除由 GORM 自动排除）
	var novels []Novel
	if err := DB.Where("author_id IN ? AND status = ?", ids, "published").
		Order("created_at DESC").Limit(window).Find(&novels).Error; err != nil {
		return nil, 0, err
	}
	for _, n := range novels {
		items = append(items, FeedItem{
			Type:      "novel",
			ID:        n.ID,
			Title:     n.Title,
			Summary:   clampSummary(n.Summary),
			UserID:    n.AuthorID,
			CreatedAt: n.CreatedAt,
			URL:       feedURL("novel", n.AuthorID, n.ID),
		})
	}

	// 博客
	var blogs []AuthorBlog
	if err := DB.Where("author_id IN ?", ids).
		Order("created_at DESC").Limit(window).Find(&blogs).Error; err != nil {
		return nil, 0, err
	}
	for _, b := range blogs {
		items = append(items, FeedItem{
			Type:      "blog",
			ID:        b.ID,
			Title:     b.Title,
			Summary:   clampSummary(b.Summary),
			UserID:    b.AuthorID,
			CreatedAt: b.CreatedAt,
			URL:       feedURL("blog", b.AuthorID, b.ID),
		})
	}

	// 论坛帖子
	var threads []Thread
	if err := DB.Where("user_id IN ?", ids).
		Order("created_at DESC").Limit(window).Find(&threads).Error; err != nil {
		return nil, 0, err
	}
	for _, t := range threads {
		items = append(items, FeedItem{
			Type:      "thread",
			ID:        t.ID,
			Title:     t.Title,
			Summary:   clampSummary(t.Content),
			UserID:    t.UserID,
			CreatedAt: t.CreatedAt,
			URL:       feedURL("thread", t.UserID, t.ID),
		})
	}

	// 统一按时间倒序（同秒按 ID 倒序，保证顺序稳定）
	sortFeedItems(items)

	// 截取本页
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []FeedItem{}, total, nil
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	pageItems := items[start:end]

	// 批量填充作者展示名
	fillFeedUserNames(pageItems)
	return pageItems, total, nil
}

// fillFeedUserNames 批量填充 UserName（昵称优先，无昵称用用户名）
func fillFeedUserNames(items []FeedItem) {
	if len(items) == 0 {
		return
	}
	need := map[uint]bool{}
	for _, it := range items {
		need[it.UserID] = true
	}
	ids := make([]uint, 0, len(need))
	for id := range need {
		ids = append(ids, id)
	}
	var users []User
	DB.Where("id IN ?", ids).Find(&users)
	names := map[uint]string{}
	for _, u := range users {
		name := u.Nickname
		if name == "" {
			name = u.Username
		}
		names[u.ID] = name
	}
	for i := range items {
		items[i].UserName = names[items[i].UserID]
	}
}

// sortFeedItems 按 CreatedAt 倒序；同秒按 ID 倒序保证稳定
func sortFeedItems(items []FeedItem) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0; j-- {
			a, b := items[j-1], items[j]
			if a.CreatedAt.Before(b.CreatedAt) ||
				(a.CreatedAt.Equal(b.CreatedAt) && a.ID < b.ID) {
				items[j-1], items[j] = items[j], items[j-1]
				continue
			}
			break
		}
	}
}

// clampSummary 摘要截断（按 rune，避免截断多字节字符）
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

// normalizePage 页码/页大小归一化
func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

// feedURL 动态条目前端路由路径（与 web/src/router/index.ts 对齐）：
// 小说 /author/{作者}/novel/{id}，博客 /author/{作者}/blogs/{id}，帖子 /thread/{id}
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
