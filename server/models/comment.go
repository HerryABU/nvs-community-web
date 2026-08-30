package models

import (
	"strconv"
	"time"

	"gorm.io/gorm"
)

type Comment struct {
	ID            uint   `gorm:"primaryKey" json:"id"`
	UserID        uint   `gorm:"not null;index" json:"user_id"`
	NovelID       uint   `gorm:"default:0;index:idx_comments_novel_created,priority:1" json:"novel_id"`
	BlogID        uint   `gorm:"default:0;index" json:"blog_id"`
	ChapterNumber int    `gorm:"default:0" json:"chapter_number"`
	Content       string `gorm:"type:text;not null" json:"content"`
	QuoteText     string `gorm:"size:1024;default:''" json:"quote_text"`
	QuoteOffset   int    `gorm:"default:0" json:"quote_offset"`
	ParentID      *uint  `gorm:"index:idx_comments_parent" json:"parent_id"`
	// 楼层模型（公众号评论区 + Discuz 楼中楼）：
	//   RootID  = 所属「楼」的顶层评论 id；顶层评论自身为 NULL
	//   FloorNo = 楼层号（仅顶层评论有值，从 1 递增；楼内回复为 0）
	// 语义：回复主内容（作品/博客/帖子）→ 开新楼（ParentID=NULL）；
	//       回复楼内任意一条 → 挂靠该楼（RootID 继承），展示为「X 回复 Y」。
	RootID      *uint          `gorm:"index:idx_comments_root" json:"root_id"`
	FloorNo     int            `gorm:"default:0;index:idx_comments_floor" json:"floor_no"`
	IsMarkdown  bool           `gorm:"default:true" json:"is_markdown"`
	Username    string         `gorm:"-" json:"username,omitempty"`
	ReplyToName string         `gorm:"-" json:"reply_to_name,omitempty"`               // 被回复者昵称（运行时填充，展示「X 回复 Y」）
	DeletedBy   string         `gorm:"size:16;default:''" json:"deleted_by,omitempty"` // user=用户自删 / admin=管理员封禁删除 / 空=未删除或旧数据
	CreatedAt   time.Time      `gorm:"index:idx_comments_novel_created,priority:2" json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"` // 合规留存：软删除标记

	// 关联
	User   *User    `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Novel  *Novel   `gorm:"foreignKey:NovelID" json:"novel,omitempty"`
	Parent *Comment `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
}

func (Comment) TableName() string {
	return "comments"
}

// ==================== 楼层模型 ====================

// resolveFloor 在落库前推导 RootID 与 FloorNo。
//   - 无父评论（回复主内容）→ 开新楼：RootID=NULL，FloorNo=当前最大楼层+1
//   - 有父评论（回复楼内某人）→ 挂靠该楼：RootID=父评论的 RootID（父为顶层时取其自身 id），FloorNo=0
//
// 注意：必须在 Create 之前调用，否则 RootID 无法回填。
func (c *Comment) resolveFloor() {
	if c.ParentID == nil || *c.ParentID == 0 {
		var maxFloor int
		DB.Model(&Comment{}).
			Where("novel_id = ? AND blog_id = ? AND chapter_number = ? AND parent_id IS NULL",
				c.NovelID, c.BlogID, c.ChapterNumber).
			Select("COALESCE(MAX(floor_no), 0)").Scan(&maxFloor)
		c.RootID = nil
		c.FloorNo = maxFloor + 1
		return
	}
	var parent Comment
	if err := DB.Select("id", "root_id").First(&parent, *c.ParentID).Error; err != nil {
		// 父评论已删除/不存在：退化成新楼，避免产生孤儿回复
		c.ParentID = nil
		c.resolveFloor()
		return
	}
	c.RootID = &parent.ID
	if parent.RootID != nil {
		c.RootID = parent.RootID
	}
	c.FloorNo = 0
}

// CommentsPage 分页结果（楼层模型）：
// List 为本页楼层 + 这些楼层下的全部回复（平铺，靠 root_id 由前端分组成「楼」），
// Total 为该目标下评论总数（含楼内回复），FloorTotal 为楼层数。
type CommentsPage struct {
	List       []Comment
	Total      int64
	FloorTotal int64
	HasMore    bool
}

// fillReplyToName 为楼内回复填上被回复者昵称（展示「X 回复 Y」）
func fillReplyToName(list []Comment) {
	need := map[uint]bool{}
	for _, c := range list {
		if c.ParentID != nil && *c.ParentID != 0 {
			need[*c.ParentID] = true
		}
	}
	if len(need) == 0 {
		return
	}
	ids := make([]uint, 0, len(need))
	for id := range need {
		ids = append(ids, id)
	}
	var parents []Comment
	DB.Where("id IN ?", ids).Preload("User").Find(&parents)
	nameOf := map[uint]string{}
	for _, p := range parents {
		n := ""
		if p.User != nil {
			n = p.User.Nickname
			if n == "" {
				n = p.User.Username
			}
		}
		nameOf[p.ID] = n
	}
	for i := range list {
		if list[i].ParentID != nil {
			list[i].ReplyToName = nameOf[*list[i].ParentID]
		}
	}
}

// loadCommentsPage 通用分页：先取本页楼层，再取这些楼层下的全部回复
func loadCommentsPage(scope func(*gorm.DB) *gorm.DB, page, pageSize int) (CommentsPage, error) {
	var out CommentsPage
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	floorQuery := DB.Model(&Comment{}).Scopes(scope).Where("parent_id IS NULL")
	floorQuery.Count(&out.FloorTotal)
	// 评论总数（含楼内回复）
	DB.Model(&Comment{}).Scopes(scope).Count(&out.Total)

	var floors []Comment
	if err := DB.Scopes(scope).Where("parent_id IS NULL").
		Preload("User").
		Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&floors).Error; err != nil {
		return out, err
	}
	if len(floors) == 0 {
		return out, nil
	}

	rootIDs := make([]uint, 0, len(floors))
	for _, f := range floors {
		rootIDs = append(rootIDs, f.ID)
	}
	var replies []Comment
	if err := DB.Where("root_id IN ?", rootIDs).
		Preload("User").
		Order("created_at ASC").
		Find(&replies).Error; err != nil {
		return out, err
	}

	out.List = make([]Comment, 0, len(floors)+len(replies))
	out.List = append(out.List, floors...)
	out.List = append(out.List, replies...)
	fillReplyToName(out.List)
	out.HasMore = int64(page*pageSize) < out.FloorTotal
	return out, nil
}

// BackfillCommentFloors 存量数据回填（幂等）：
// 老评论没有 root_id / floor_no，按同一目标（作品+章节 或 博客）的时间正序补楼层号，
// 并把回复挂到其顶层评论下。仅在 floor_no = 0 且 parent_id IS NULL 时处理。
// db 由调用方传入（AutoMigrateAll 期间全局 DB 可能尚未就绪，测试场景尤其如此）。
func BackfillCommentFloors(db *gorm.DB) {
	// 1) 回填回复的 root_id（父为顶层→取父 id；父也是回复→取父的 root_id）
	_ = db.Exec(`UPDATE comments SET root_id = (
		SELECT p.id FROM (SELECT id, root_id FROM comments) p WHERE p.id = comments.parent_id
	) WHERE parent_id IS NOT NULL AND parent_id <> 0 AND (root_id IS NULL OR root_id = 0)`).Error
	// 父评论本身也是回复时，root_id 需要再上溯一层（二级回复场景）
	_ = db.Exec(`UPDATE comments SET root_id = (
		SELECT p.root_id FROM (SELECT id, root_id FROM comments) p WHERE p.id = comments.root_id
	) WHERE parent_id IS NOT NULL AND parent_id <> 0 AND root_id IS NOT NULL AND root_id IN (
		SELECT id FROM (SELECT id FROM comments WHERE parent_id IS NOT NULL AND parent_id <> 0) x
	)`).Error

	// 2) 为顶层评论补楼层号
	type group struct {
		NovelID       uint
		BlogID        uint
		ChapterNumber int
	}
	var groups []group
	_ = db.Model(&Comment{}).
		Where("parent_id IS NULL AND (floor_no IS NULL OR floor_no = 0)").
		Select("novel_id", "blog_id", "chapter_number").
		Group("novel_id, blog_id, chapter_number").
		Scan(&groups).Error

	for _, g := range groups {
		var tops []Comment
		db.Where("novel_id = ? AND blog_id = ? AND chapter_number = ? AND parent_id IS NULL",
			g.NovelID, g.BlogID, g.ChapterNumber).
			Order("created_at ASC, id ASC").Find(&tops)
		var maxExisting int
		db.Model(&Comment{}).
			Where("novel_id = ? AND blog_id = ? AND chapter_number = ? AND parent_id IS NULL AND floor_no > 0",
				g.NovelID, g.BlogID, g.ChapterNumber).
			Select("COALESCE(MAX(floor_no), 0)").Scan(&maxExisting)
		n := maxExisting
		for _, t := range tops {
			if t.FloorNo > 0 {
				continue
			}
			n++
			db.Model(&Comment{}).Where("id = ?", t.ID).Update("floor_no", n)
		}
	}
}

// ==================== 读写 ====================

func CreateComment(comment *Comment) error {
	// 楼层归属必须在落库前推导（RootID 依赖父评论的 RootID）
	comment.resolveFloor()
	if err := DB.Create(comment).Error; err != nil {
		return err
	}
	// 站内通知（best-effort，失败不影响评论本身）
	content := comment.Content
	if len(content) > 60 {
		content = content[:60] + "…"
	}
	// 1) 评论被回复 → 通知被回复评论的作者
	if comment.ParentID != nil && *comment.ParentID != 0 {
		var parent Comment
		if DB.Select("user_id").First(&parent, *comment.ParentID).Error == nil {
			_ = PushNotification(comment.UserID, parent.UserID, "comment_reply",
				"你的评论收到回复", content, linkForComment(comment))
		}
	}
	// 2) 作品收到新评论 → 通知作品作者
	if comment.NovelID != 0 {
		var n Novel
		if DB.Select("author_id", "title").First(&n, comment.NovelID).Error == nil {
			_ = PushNotification(comment.UserID, n.AuthorID, "novel_comment",
				"你的作品收到新评论", "《"+n.Title+"》", "/novel/"+strconv.FormatUint(uint64(comment.NovelID), 10))
		}
	}
	// 3) 博客收到新评论 → 通知博客作者
	if comment.BlogID != 0 {
		var b AuthorBlog
		if DB.Select("author_id", "title").First(&b, comment.BlogID).Error == nil {
			_ = PushNotification(comment.UserID, b.AuthorID, "blog_comment",
				"你的博客收到新评论", "《"+b.Title+"》", "/author/"+strconv.FormatUint(uint64(b.AuthorID), 10)+"/blogs/"+strconv.FormatUint(uint64(comment.BlogID), 10))
		}
	}
	return nil
}

// linkForComment 评论跳转链接：章评带章节号，作品级评论不带
func linkForComment(c *Comment) string {
	base := "/novel/" + strconv.FormatUint(uint64(c.NovelID), 10)
	if c.NovelID == 0 {
		return ""
	}
	if c.ChapterNumber > 0 {
		base += "/read/" + strconv.Itoa(c.ChapterNumber)
	}
	return base + "#comment-" + strconv.FormatUint(uint64(c.ID), 10)
}

func novelCommentScope(novelID uint, chapterNumber int) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		q := db.Where("novel_id = ?", novelID)
		if chapterNumber > 0 {
			q = q.Where("chapter_number = ?", chapterNumber)
		} else {
			// 未指定章节号 → 只取作品级评论（chapter_number = 0）
			q = q.Where("chapter_number = 0")
		}
		return q
	}
}

// GetCommentsByNovel 作品/章节评论（楼层分页）
func GetCommentsByNovel(novelID uint, chapterNumber int, page, pageSize int) (CommentsPage, error) {
	return loadCommentsPage(novelCommentScope(novelID, chapterNumber), page, pageSize)
}

func GetCommentByID(id uint) (*Comment, error) {
	var comment Comment
	err := DB.Preload("User").First(&comment, id).Error
	if err != nil {
		return nil, err
	}
	return &comment, nil
}

// GetAllCommentsByNovel 获取作品的「全部」评论（书评 + 所有章评，按时间倒序）。
// 供内容抢救台等场景一次性拉取作品下所有评论，便于审核/排查。
func GetAllCommentsByNovel(novelID uint, limit int) ([]Comment, error) {
	if limit <= 0 {
		limit = 500
	}
	var comments []Comment
	err := DB.Where("novel_id = ?", novelID).
		Preload("User").
		Order("created_at DESC").
		Limit(limit).
		Find(&comments).Error
	return comments, err
}

// DeleteComment 删除评论。若删的是楼层主评，其楼内回复一并软删除（避免孤儿回复悬浮）。
func DeleteComment(id uint) error {
	var children []Comment
	DB.Where("root_id = ?", id).Find(&children)
	if len(children) > 0 {
		ids := make([]uint, 0, len(children))
		for _, c := range children {
			ids = append(ids, c.ID)
		}
		if err := DB.Where("id IN ?", ids).Delete(&Comment{}).Error; err != nil {
			return err
		}
	}
	return DB.Delete(&Comment{}, id).Error
}

// GetCommentsByBlog 博客评论（楼层分页）
func GetCommentsByBlog(blogID uint, page, pageSize int) (CommentsPage, error) {
	return loadCommentsPage(func(db *gorm.DB) *gorm.DB {
		return db.Where("blog_id = ?", blogID)
	}, page, pageSize)
}
