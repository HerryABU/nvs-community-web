package models

import (
	"time"

	"gorm.io/gorm"
)

// Forum 论坛版块
type Forum struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:128;not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	Type        string    `gorm:"size:16;default:general;index" json:"type"` // general/reader/reader_author/author/sensitive/custom
	Zone        string    `gorm:"size:64;default:''" json:"zone"`            // 敏感区绑定的隔离墙分区名
	RefID       string    `gorm:"size:64;default:''" json:"ref_id"`          // novel_id 或 category
	ParentID    *uint     `gorm:"default:null;index" json:"parent_id"`       // 父论坛ID，null为顶级论坛
	SortOrder   int       `gorm:"default:0" json:"sort_order"`
	ThreadCount int       `gorm:"default:0" json:"thread_count"`
	// 用户自建论坛
	CreatorID    uint   `gorm:"default:0;index" json:"creator_id"`                        // 创建者用户ID，0=系统/管理员
	IsPublic     bool   `gorm:"default:false" json:"is_public"`                           // 是否在大厅自建区公开显示
	PasswordHash string `gorm:"size:128;default:''" json:"-"`                             // 进入密码哈希（空=无密码）
	ReviewStatus string `gorm:"size:16;default:approved;index" json:"review_status"`      // approved/pending/rejected（自建论坛默认 pending 待管理员审查）
	ReviewNote   string `gorm:"size:512;default:''" json:"review_note"`                   // 管理员审查意见
	CreatedAt    time.Time `json:"created_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"` // 合规留存：论坛软删除

	HasPassword bool `gorm:"-" json:"has_password"` // 是否有进入密码（运行时计算，不落库）
}

func (Forum) TableName() string { return "forums" }

// AfterFind 查询后计算派生字段
func (f *Forum) AfterFind(tx *gorm.DB) error {
	f.HasPassword = f.PasswordHash != ""
	return nil
}

// ForumMute 论坛级禁言（自建论坛创建者可对违规用户在本论坛禁言）
type ForumMute struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	ForumID    uint       `gorm:"not null;index" json:"forum_id"`
	UserID     uint       `gorm:"not null;index" json:"user_id"`
	MutedUntil *time.Time `json:"muted_until"`
	Reason     string     `gorm:"size:256;default:''" json:"reason"`
	CreatedAt  time.Time  `json:"created_at"`
}

func (ForumMute) TableName() string { return "forum_mutes" }

func GetOrCreateForum(name, ftype, refID, desc string) (*Forum, error) {
	var forum Forum
	err := DB.Where("type = ? AND ref_id = ?", ftype, refID).First(&forum).Error
	if err == nil {
		return &forum, nil
	}
	forum = Forum{Name: name, Type: ftype, RefID: refID, Description: desc}
	if err := DB.Create(&forum).Error; err != nil {
		return nil, err
	}
	return &forum, nil
}

func GetForumsByType(ftype string) ([]Forum, error) {
	var forums []Forum
	err := DB.Where("type = ? AND parent_id IS NULL", ftype).Order("sort_order ASC, id ASC").Find(&forums).Error
	return forums, err
}

// GetForumsByTypes 支持同时查询多个 type（传入切片），仅返回顶级论坛
func GetForumsByTypes(types []string) ([]Forum, error) {
	var forums []Forum
	err := DB.Where("type IN ? AND parent_id IS NULL", types).Order("sort_order ASC, id ASC").Find(&forums).Error
	return forums, err
}

// GetSubForums 获取某个父论坛下的所有子论坛
func GetSubForums(parentID uint) ([]Forum, error) {
	var forums []Forum
	err := DB.Where("parent_id = ?", parentID).Order("sort_order ASC, id ASC").Find(&forums).Error
	return forums, err
}

// ListAllForums 返回所有论坛，可选 type 过滤（逗号分隔）和 parent_id 过滤
func ListAllForums(ftype string) ([]Forum, error) {
	var forums []Forum
	query := DB.Model(&Forum{}).Order("sort_order ASC, id ASC")
	if ftype != "" {
		query = query.Where("type = ?", ftype)
	}
	err := query.Find(&forums).Error
	return forums, err
}

func GetForumByID(id uint) (*Forum, error) {
	var forum Forum
	err := DB.First(&forum, id).Error
	return &forum, err
}

// Thread 帖子
type Thread struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ForumID   uint      `gorm:"not null;index" json:"forum_id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Title     string    `gorm:"size:256;not null" json:"title"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	IsPinned  bool      `gorm:"default:false" json:"is_pinned"`
	IsLocked  bool      `gorm:"default:false" json:"is_locked"`
	ViewCount int       `gorm:"default:0" json:"view_count"`
	PostCount int       `gorm:"default:0" json:"post_count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"` // 合规留存：软删除标记
	User      *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Forum     *Forum    `gorm:"foreignKey:ForumID" json:"forum,omitempty"`
}

func (Thread) TableName() string { return "threads" }

func CreateThread(t *Thread) error {
	err := DB.Create(t).Error
	if err == nil {
		DB.Model(&Forum{}).Where("id = ?", t.ForumID).UpdateColumn("thread_count", gorm.Expr("thread_count + 1"))
	}
	return err
}

func GetThreadsByForum(forumID uint, page, pageSize int) ([]Thread, int64, error) {
	var threads []Thread
	var total int64
	query := DB.Model(&Thread{}).Where("forum_id = ?", forumID)
	query.Count(&total)
	err := query.Preload("User").Order("is_pinned DESC, updated_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&threads).Error
	return threads, total, err
}

func GetThreadByID(id uint) (*Thread, error) {
	var t Thread
	err := DB.Preload("User").Preload("Forum").First(&t, id).Error
	return &t, err
}

func IncrementThreadView(id uint) {
	DB.Model(&Thread{}).Where("id = ?", id).UpdateColumn("view_count", gorm.Expr("view_count + 1"))
}

// Post 回复
type Post struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ThreadID  uint      `gorm:"not null;index" json:"thread_id"`
	UserID    uint      `gorm:"not null" json:"user_id"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"` // 合规留存：软删除标记
	User      *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (Post) TableName() string { return "posts" }

func CreatePost(p *Post) error {
	err := DB.Create(p).Error
	if err == nil {
		DB.Model(&Thread{}).Where("id = ?", p.ThreadID).Updates(map[string]interface{}{
			"post_count": gorm.Expr("post_count + 1"),
			"updated_at": time.Now(),
		})
	}
	return err
}

func GetPostsByThread(threadID uint, page, pageSize int) ([]Post, int64, error) {
	var posts []Post
	var total int64
	query := DB.Model(&Post{}).Where("thread_id = ?", threadID)
	query.Count(&total)
	err := query.Preload("User").Order("created_at ASC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&posts).Error
	return posts, total, err
}

// InitDefaultForums 初始化默认大论坛
func InitDefaultForums() {
	type forumDef struct {
		name, ftype, desc string
	}
	defaults := []forumDef{
		{"读者交流广场", "reader", "读者之间交流读书心得、推荐好书"},
		{"读者·作者互动", "reader_author", "读者向作者提问、反馈、催更"},
		{"作者工坊", "author", "作者之间交流创作技巧、互评作品"},
		{"类型文学理论研讨", "general", "探讨各类型文学的理论基础、创作方法和美学特征"},
		{"社区议事厅", "general", "平台规则讨论、社区治理建议、公共事务投票"},
		{"跨界灵感碰撞", "general", "跨类型的创意交流、写作技巧分享、灵感启发"},
		{"创作急诊室", "general", "写作难题求助、情节推演、设定检验"},
		{"资源共享库", "general", "写作工具、参考资料、行业资讯分享"},
		{"同人创作区", "sensitive", "同人作品交流与讨论（需确认年龄和阅读风险）"},
		{"政治文学区", "sensitive", "政治题材文学创作与讨论（需确认法律风险）"},
	}
	for i, d := range defaults {
		var count int64
		DB.Model(&Forum{}).Where("type = ? AND name = ?", d.ftype, d.name).Count(&count)
		if count == 0 {
			zone := ""
		if d.ftype == "sensitive" {
			zone = d.name
		}
		DB.Create(&Forum{Name: d.name, Description: d.desc, Type: d.ftype, Zone: zone, SortOrder: i})
		}
	}
}

// CreateForum 创建论坛
func CreateForum(forum *Forum) error {
	return DB.Create(forum).Error
}

// UpdateForum 更新论坛
func UpdateForum(forum *Forum) error {
	return DB.Save(forum).Error
}

// DeleteForum 删除论坛
func DeleteForum(id uint) error {
	return DB.Delete(&Forum{}, id).Error
}

// IsForumMuted 用户是否在某论坛被禁言（未过期）
func IsForumMuted(forumID, userID uint) bool {
	var count int64
	DB.Model(&ForumMute{}).
		Where("forum_id = ? AND user_id = ? AND muted_until > ?", forumID, userID, time.Now()).
		Count(&count)
	return count > 0
}

// GetForumMute 获取用户在论坛的禁言记录（未过期）
func GetForumMute(forumID, userID uint) (*ForumMute, error) {
	var m ForumMute
	err := DB.Where("forum_id = ? AND user_id = ? AND muted_until > ?", forumID, userID, time.Now()).
		Order("muted_until DESC").First(&m).Error
	if err != nil {
		return nil, err
	}
	return &m, nil
}
