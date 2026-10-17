package models

import (
	"encoding/json"
	"regexp"
	"strconv"
	"time"

	"gorm.io/gorm"

	"nvs-server/utils"
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
	RootID  *uint `gorm:"index:idx_comments_root" json:"root_id"`
	FloorNo int   `gorm:"default:0;index:idx_comments_floor" json:"floor_no"`
	// 显式回复目标用户（评论网分析用）：回复楼内某条时 = 被回复者 user_id；回复主内容（开新楼）时为 0
	ReplyToUserID uint `gorm:"default:0;index" json:"reply_to_user_id"`
	// 楼加密（入口限制，非内容加密）：仅顶层评论（楼）可设访问密码。
	// 语义与「加密论坛」同构——未解锁者（非楼作者、非警方/管理员）读取时
	// content 脱敏为占位、楼内回复不返回；凭 X-Floor-Password 头解锁。
	// 注意：内容本身不做加密存储， PasswordHash 仅用于入口校验。
	HasPassword  bool           `gorm:"default:false;index" json:"has_password"`
	PasswordHash string         `gorm:"size:128;default:''" json:"-"` // bcrypt，不序列化外泄
	Locked       bool           `gorm:"-" json:"locked"`              // 运行时：当前请求未解锁（脱敏占位标记）
	IsMarkdown   bool           `gorm:"default:true" json:"is_markdown"`
	Username     string         `gorm:"-" json:"username,omitempty"`
	ReplyToName  string         `gorm:"-" json:"reply_to_name,omitempty"`               // 被回复者昵称（运行时填充，展示「X 回复 Y」）
	DeletedBy    string         `gorm:"size:16;default:''" json:"deleted_by,omitempty"` // user=用户自删 / admin=管理员封禁删除 / 空=未删除或旧数据
	IsDeleted    bool           `gorm:"-" json:"is_deleted"`                            // 运行时计算：是否已软删除（警方/管理员取证视图用）
	// 微秒级时间戳（Unix 微秒）：用于评论网分析的时间估计与精确排序；旧数据回填为 0
	CreatedMicro int64 `gorm:"default:0;index" json:"created_micro"`
	// @ 提及：存储 JSON 数组 [{"name":"foo","user_id":5}]；运行时由 AfterFind/MentionsRefs 解析到 Mentions
	MentionsRaw string       `gorm:"type:text;default:''" json:"-"`
	Mentions    []MentionRef `gorm:"-" json:"mentions,omitempty"` // 解析后的提及列表（展示/跳转用）
	CreatedAt   time.Time    `gorm:"index:idx_comments_novel_created,priority:2" json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"` // 合规留存：软删除标记

	// 关联
	User   *User    `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Novel  *Novel   `gorm:"foreignKey:NovelID" json:"novel,omitempty"`
	Parent *Comment `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
}

func (Comment) TableName() string {
	return "comments"
}

// AfterFind 查询后计算 is_deleted（警方/管理员取证视图据此在前端以红字标注已删除内容）。
// 常规查询被 GORM 软删除作用域过滤，deleted_at 为空 → is_deleted=false；
// 仅 include_deleted（Unscoped）场景才会取回已删除行，此时 deleted_at 有值 → is_deleted=true。
func (c *Comment) AfterFind(tx *gorm.DB) error {
	c.IsDeleted = c.DeletedAt.Valid
	if c.MentionsRaw != "" {
		var ms []MentionRef
		if json.Unmarshal([]byte(c.MentionsRaw), &ms) == nil {
			c.Mentions = ms
		}
	}
	return nil
}

// MentionRef @ 提及解析结果：被提及的展示名与其用户 id（解析失败/用户不存在则不含该条）。
type MentionRef struct {
	Name   string `json:"name"`
	UserID uint   `json:"user_id"`
}

// mentionRe 匹配 @用户名：要求 @ 前非单词字符（避免邮箱 a@b 误判），用户名 2-20 位
// 允许字母/数字/下划线/连字符/中文。Go RE2 不支持后顾/前瞻，尾随字符校验在代码中处理。
var mentionRe = regexp.MustCompile(`(?:^|[^A-Za-z0-9_@])@([A-Za-z0-9_\-一-龥]{2,20})`)

// isWordByte 判断字节是否为 ASCII 单词字符（用于尾随校验，防止 @name 吞噬后续词）
func isWordByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

// ParseMentions 从评论正文中提取 @提及，解析为存在的用户（按 username / nickname 匹配）。
// 去重：同一用户名、同一用户只记一次；用户不存在则跳过（不报错）。
func ParseMentions(content string) []MentionRef {
	if content == "" {
		return nil
	}
	locs := mentionRe.FindAllStringSubmatchIndex(content, -1)
	if len(locs) == 0 {
		return nil
	}
	seenName := map[string]bool{}
	seenUID := map[uint]bool{}
	out := make([]MentionRef, 0, len(locs))
	for _, loc := range locs {
		// 正则仅有一个捕获组（@后的用户名）。FindAllStringSubmatchIndex 对单组匹配返回
		// 4 个整数：[fullStart, fullEnd, nameStart, nameEnd]，故用户名取 loc[2], loc[3]。
		nameStart, nameEnd := loc[2], loc[3]
		name := content[nameStart:nameEnd]
		// 尾随字符为单词字符（如命中 20 字上限后仍有字母）：视为不完整，跳过
		if nameEnd < len(content) && isWordByte(content[nameEnd]) {
			continue
		}
		if seenName[name] {
			continue
		}
		seenName[name] = true
		var u User
		if err := DB.Where("username = ? OR nickname = ?", name, name).First(&u).Error; err != nil {
			continue // 用户不存在：忽略该 @
		}
		if seenUID[u.ID] {
			continue
		}
		seenUID[u.ID] = true
		out = append(out, MentionRef{Name: name, UserID: u.ID})
	}
	return out
}

// MentionsToRaw 将提及列表序列化为 JSON 存储字符串；空列表返回空串。
func MentionsToRaw(ms []MentionRef) string {
	if len(ms) == 0 {
		return ""
	}
	b, err := json.Marshal(ms)
	if err != nil {
		return ""
	}
	return string(b)
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
	if err := DB.Select("id", "root_id", "user_id").First(&parent, *c.ParentID).Error; err != nil {
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
	// 显式记录回复目标用户（评论网分析用，避免仅靠 parent_id 反查）
	c.ReplyToUserID = parent.UserID
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

// loadCommentsPage 通用分页：先取本页楼层，再取这些楼层下的全部回复。
// includeDeleted=true 时（警方/管理员取证视图）用 Unscoped 绕过软删除作用域，
// 一并取回已删除楼层/回复，由前端以红字标注。
func loadCommentsPage(scope func(*gorm.DB) *gorm.DB, page, pageSize int, includeDeleted bool) (CommentsPage, error) {
	var out CommentsPage
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	base := DB
	if includeDeleted {
		base = DB.Unscoped()
	}

	floorQuery := base.Model(&Comment{}).Scopes(scope).Where("parent_id IS NULL")
	floorQuery.Count(&out.FloorTotal)
	// 评论总数（含楼内回复）
	base.Model(&Comment{}).Scopes(scope).Count(&out.Total)

	var floors []Comment
	if err := base.Scopes(scope).Where("parent_id IS NULL").
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
	if err := base.Where("root_id IN ?", rootIDs).
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

// ==================== 楼加密（入口限制） ====================

// FloorAccessUnlocked 判断某层楼对当前查看者是否已解锁。
// 解锁条件（任一）：楼未设密码 / 查看者是楼作者本人 / 查看者是警方或管理员 / 密码正确。
// passwords 为「楼层 id → 访问密码」映射（列表场景一次带多楼；单楼场景传单元素 map）。
// 这是入口限制而非内容加密：内容明文存储，仅控制是否返回。
func FloorAccessUnlocked(floor *Comment, viewerID uint, isStaff bool, passwords map[uint]string) bool {
	if floor == nil || !floor.HasPassword {
		return true
	}
	if viewerID != 0 && floor.UserID == viewerID {
		return true
	}
	if isStaff {
		return true
	}
	pwd := passwords[floor.ID]
	if pwd == "" {
		return false
	}
	return utils.CheckPassword(pwd, floor.PasswordHash)
}

// RedactLockedComments 对平铺评论列表做楼加密脱敏（models 层统一收口）。
// 未解锁的顶层楼 → content 置空 + Locked=true（前端渲染 🔒 占位）；
// 该楼名下的全部楼内回复一并从列表剔除（防止楼内内容绕读出）。
// 返回新切片，不修改入参（调用方可能基于同一列表做多次判定）。
func RedactLockedComments(list []Comment, viewerID uint, isStaff bool, passwords map[uint]string) []Comment {
	// 整体拷贝：脱敏标记只落在副本上，避免污染调用方切片
	work := make([]Comment, len(list))
	copy(work, list)

	lockedRoots := map[uint]bool{}
	for i := range work {
		c := &work[i]
		if c.ParentID != nil && *c.ParentID != 0 {
			continue // 楼内回复稍后按 lockedRoots 处理
		}
		if !FloorAccessUnlocked(c, viewerID, isStaff, passwords) {
			c.Locked = true
			c.Content = ""
			c.QuoteText = ""
			lockedRoots[c.ID] = true
		}
	}
	if len(lockedRoots) == 0 {
		return work
	}
	out := make([]Comment, 0, len(work))
	for _, c := range work {
		if c.ParentID != nil && *c.ParentID != 0 {
			root := c.RootID
			if root == nil {
				root = c.ParentID
			}
			if root != nil && lockedRoots[*root] {
				continue // 加密楼未解锁：楼内回复不返回
			}
		}
		out = append(out, c)
	}
	return out
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
				"你的评论收到回复", content, LinkForComment(comment))
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

// LinkForComment 评论跳转链接：章评带章节号，作品级评论不带
func LinkForComment(c *Comment) string {
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

// GetCommentsByNovel 作品/章节评论（楼层分页）。includeDeleted=true 时一并返回已删除评论（警方取证视图）。
func GetCommentsByNovel(novelID uint, chapterNumber int, page, pageSize int, includeDeleted bool) (CommentsPage, error) {
	return loadCommentsPage(novelCommentScope(novelID, chapterNumber), page, pageSize, includeDeleted)
}

// GetCommentByID 按 id 取单条评论（含 User 预加载）。
func GetCommentByID(id uint) (*Comment, error) {
	var comment Comment
	err := DB.Preload("User").First(&comment, id).Error
	if err != nil {
		return nil, err
	}
	return &comment, nil
}

// SetFloorPassword 设置/修改楼的访问密码（入口限制，非内容加密）。
// passwordHash 为空且 hasPassword=false 表示解除加密。
// 仅允许顶层评论（parent_id IS NULL）设密——楼内回复不单独设密码。
// 用 map Updates 按列精确写，避免 GORM 结构体零值被默认值吞掉。
func SetFloorPassword(floorID uint, passwordHash string, hasPassword bool) error {
	return DB.Model(&Comment{}).Where("id = ? AND parent_id IS NULL", floorID).
		Updates(map[string]any{"password_hash": passwordHash, "has_password": hasPassword}).Error
}

// GetFloorWithReplies 取单楼（顶层评论）+ 全部楼内回复（时间正序）。
// 供「楼详情」端点使用：凭密码解锁后可整楼读取（前端 🔒 占位解锁后调用）。
func GetFloorWithReplies(floorID uint) (*Comment, []Comment, error) {
	var floor Comment
	if err := DB.Preload("User").First(&floor, floorID).Error; err != nil {
		return nil, nil, err
	}
	var replies []Comment
	if err := DB.Preload("User").
		Where("root_id = ?", floorID).
		Order("created_at ASC").
		Find(&replies).Error; err != nil {
		return nil, nil, err
	}
	fillReplyToName(replies)
	return &floor, replies, nil
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

// GetCommentsByBlog 博客评论（楼层分页）。includeDeleted=true 时一并返回已删除评论（警方取证视图）。
func GetCommentsByBlog(blogID uint, page, pageSize int, includeDeleted bool) (CommentsPage, error) {
	return loadCommentsPage(func(db *gorm.DB) *gorm.DB {
		return db.Where("blog_id = ?", blogID)
	}, page, pageSize, includeDeleted)
}
