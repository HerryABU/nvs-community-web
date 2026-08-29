package models

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Forum 论坛版块
type Forum struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	Name        string `gorm:"size:128;not null" json:"name"`
	Description string `gorm:"type:text" json:"description"`
	Type        string `gorm:"size:16;default:general;index" json:"type"` // general/reader/reader_author/author/sensitive/custom
	Zone        string `gorm:"size:64;default:''" json:"zone"`            // 敏感区绑定的隔离墙分区名
	RefID       string `gorm:"size:64;default:''" json:"ref_id"`          // novel_id 或 category
	ParentID    *uint  `gorm:"default:null;index" json:"parent_id"`       // 父论坛ID，null为顶级论坛
	SortOrder   int    `gorm:"default:0" json:"sort_order"`
	ThreadCount int    `gorm:"default:0" json:"thread_count"`
	// 用户自建论坛
	CreatorID    uint           `gorm:"default:0;index" json:"creator_id"`                   // 创建者用户ID，0=系统/管理员
	IsPublic     bool           `gorm:"default:false" json:"is_public"`                      // 是否在大厅自建区公开显示
	PasswordHash string         `gorm:"size:128;default:''" json:"-"`                        // 进入密码哈希（空=无密码）
	RoomCode     string         `gorm:"size:16;index" json:"room_code"`                      // 房间号（数字+字母）——不可见论坛的进入凭证；系统默认论坛为空；自建论坛唯一性由代码层保证
	ReviewStatus string         `gorm:"size:16;default:approved;index" json:"review_status"` // approved/pending/rejected（自建论坛默认 pending 待管理员审查）
	ReviewNote   string         `gorm:"size:512;default:''" json:"review_note"`              // 管理员审查意见
	CreatedAt    time.Time      `json:"created_at"`
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
	ForumID    uint       `gorm:"not null;index:idx_forum_mutes_forum_user,priority:1" json:"forum_id"`
	UserID     uint       `gorm:"not null;index:idx_forum_mutes_forum_user,priority:2" json:"user_id"`
	MutedUntil *time.Time `json:"muted_until"`
	Reason     string     `gorm:"size:256;default:''" json:"reason"`
	AdminBan   bool       `gorm:"default:false" json:"admin_ban"` // 管理员封禁（论坛创建者无权解除）
	CreatedAt  time.Time  `json:"created_at"`

	// 运行时补充
	Username string `gorm:"-" json:"username,omitempty"`
	BannedBy string `gorm:"-" json:"banned_by,omitempty"` // 封禁执行者描述（管理员/创建者）
}

func (ForumMute) TableName() string { return "forum_mutes" }

// ==================== 自建论坛配额（全局默认 + 按人覆盖） ====================

// UserForumQuota 管理员对单个用户「自建论坛数量」的额度覆盖。
// MaxForums 语义：-1 = 跟随全局默认；0 = 禁止创建；> 0 = 该用户的专属上限。
// 典型用法：对滥用建版的人单独压到 1 或 0（0 即等同削减其建版权利），
// 对产出优质社区的人单独放宽到 10。
type UserForumQuota struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;uniqueIndex" json:"user_id"`
	MaxForums int       `gorm:"default:-1" json:"max_forums"`
	Note      string    `gorm:"size:256;default:''" json:"note"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Username string `gorm:"-" json:"username,omitempty"`
}

func (UserForumQuota) TableName() string { return "user_forum_quotas" }

// ForumQuotaConfig 全局自建论坛配额（存 platform_configs.forum_quota）
type ForumQuotaConfig struct {
	MaxPerUser int `json:"max_per_user"` // 每人最多可创建的自建论坛数；0 = 不限制
}

const forumQuotaConfigKey = "forum_quota"

// DefaultForumQuotaConfig 默认：每人 3 个自建论坛
func DefaultForumQuotaConfig() ForumQuotaConfig {
	return ForumQuotaConfig{MaxPerUser: 3}
}

// LoadForumQuotaConfig 读取全局自建论坛配额
func LoadForumQuotaConfig() ForumQuotaConfig {
	cfg := DefaultForumQuotaConfig()
	val := GetPlatformConfig(forumQuotaConfigKey)
	if val == "" {
		return cfg
	}
	if err := jsonUnmarshal([]byte(val), &cfg); err != nil {
		return DefaultForumQuotaConfig()
	}
	if cfg.MaxPerUser < 0 {
		cfg.MaxPerUser = 0
	}
	if cfg.MaxPerUser > 1000 {
		cfg.MaxPerUser = 1000
	}
	return cfg
}

// SaveForumQuotaConfig 保存全局自建论坛配额
func SaveForumQuotaConfig(cfg ForumQuotaConfig) error {
	if cfg.MaxPerUser < 0 {
		cfg.MaxPerUser = 0
	}
	if cfg.MaxPerUser > 1000 {
		cfg.MaxPerUser = 1000
	}
	b, err := jsonMarshal(cfg)
	if err != nil {
		return err
	}
	return SetPlatformConfig(forumQuotaConfigKey, string(b))
}

// GetUserForumQuota 读取某用户的额度覆盖（无记录返回 nil = 跟随全局）
func GetUserForumQuota(userID uint) *UserForumQuota {
	if userID == 0 {
		return nil
	}
	var q UserForumQuota
	if err := DB.Where("user_id = ?", userID).First(&q).Error; err != nil {
		return nil
	}
	return &q
}

// SaveUserForumQuota 写入/更新某用户的额度覆盖。
//
// ⚠️ 必须用 map 显式 upsert：MaxForums 的「0 = 禁止创建」是零值，
// 若走 struct Create，GORM 会把零值字段当作「未设置」而套用列默认值 -1，
// 导致「剥夺建版权利」静默失效（表现为仍然能建版）。这里按列显式写死。
func SaveUserForumQuota(q *UserForumQuota) error {
	if q == nil || q.UserID == 0 {
		return fmt.Errorf("用户 ID 无效")
	}
	if q.MaxForums < -1 {
		q.MaxForums = -1
	}
	if q.MaxForums > 1000 {
		q.MaxForums = 1000
	}
	now := time.Now()
	cols := map[string]interface{}{
		"user_id":    q.UserID,
		"max_forums": q.MaxForums,
		"note":       q.Note,
		"updated_at": now,
	}
	if existing := GetUserForumQuota(q.UserID); existing != nil {
		q.ID = existing.ID
		return DB.Model(&UserForumQuota{}).Where("user_id = ?", q.UserID).Updates(cols).Error
	}
	cols["created_at"] = now
	return DB.Model(&UserForumQuota{}).Create(cols).Error
}

// ListUserForumQuotas 列出所有被单独设置过建版额度的用户（附用户名）
func ListUserForumQuotas() ([]UserForumQuota, error) {
	var list []UserForumQuota
	if err := DB.Order("updated_at DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return list, nil
	}
	ids := make([]uint, 0, len(list))
	for _, q := range list {
		ids = append(ids, q.UserID)
	}
	var users []User
	DB.Where("id IN ?", ids).Find(&users)
	names := map[uint]string{}
	for _, u := range users {
		names[u.ID] = u.Username
	}
	for i := range list {
		list[i].Username = names[list[i].UserID]
	}
	return list, nil
}

// CountUserCreatedForums 统计某用户已创建的自建论坛数（不含软删除）
func CountUserCreatedForums(userID uint) int64 {
	var n int64
	DB.Model(&Forum{}).Where("creator_id = ? AND type = ?", userID, "custom").Count(&n)
	return n
}

// ForumQuotaStatus 自建论坛额度状态（供用户侧与管理端展示）
type ForumQuotaStatus struct {
	Used      int64 `json:"used"`      // 已创建数量
	Limit     int   `json:"limit"`     // 实际生效上限（-1 = 不限制）
	Global    int   `json:"global"`    // 全局默认（0 = 不限制）
	Override  bool  `json:"override"`  // 是否被管理员单独设置过
	Remaining int   `json:"remaining"` // 还可创建几个（-1 = 不限制）
}

// ForumQuotaFor 计算某用户的自建论坛额度状态
func ForumQuotaFor(userID uint) ForumQuotaStatus {
	global := LoadForumQuotaConfig()
	// 全局语义：0 = 不限制 → 内部统一用 -1 表示「不限制」
	limit := global.MaxPerUser
	if limit == 0 {
		limit = -1
	}
	st := ForumQuotaStatus{Global: global.MaxPerUser, Limit: limit}
	if userID != 0 {
		st.Used = CountUserCreatedForums(userID)
	}
	// 按人覆盖语义：-1 = 跟随全局；0 = 禁止创建；>0 = 专属上限
	if q := GetUserForumQuota(userID); q != nil && q.MaxForums >= 0 {
		st.Override = true
		st.Limit = q.MaxForums
	}
	if st.Limit < 0 {
		st.Remaining = -1
	} else {
		rem := int64(st.Limit) - st.Used
		if rem < 0 {
			rem = 0
		}
		st.Remaining = int(rem)
	}
	return st
}

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
	ID        uint           `gorm:"primaryKey" json:"id"`
	ForumID   uint           `gorm:"not null;index:idx_threads_forum_created,priority:1" json:"forum_id"`
	UserID    uint           `gorm:"not null;index" json:"user_id"`
	Title     string         `gorm:"size:256;not null" json:"title"`
	Content   string         `gorm:"type:text;not null" json:"content"`
	IsPinned  bool           `gorm:"default:false" json:"is_pinned"`
	IsLocked  bool           `gorm:"default:false" json:"is_locked"`
	ViewCount int            `gorm:"default:0" json:"view_count"`
	PostCount int            `gorm:"default:0" json:"post_count"`
	CreatedAt time.Time      `gorm:"index:idx_threads_forum_created,priority:2" json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"` // 合规留存：软删除标记
	User      *User          `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Forum     *Forum         `gorm:"foreignKey:ForumID" json:"forum,omitempty"`
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
// 删除语义（方案A）：posts 表不设 deleted_at 软删除标记，删除统一走
// softdel_posts 留证表（先 ArchiveSoftDelete 归档快照，再物理删除原始记录），
// 与 novels/chapters/comments 等「软删除 + softdel_* 留证」双轨逻辑保持一致，
// 避免 deleted_at 与 softdel_posts 两套删除语义并存造成混乱。
type Post struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ThreadID  uint      `gorm:"not null;index:idx_posts_thread_created,priority:1" json:"thread_id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	CreatedAt time.Time `gorm:"index:idx_posts_thread_created,priority:2" json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
		// 站内通知：帖子作者收到「有人回复」
		var t Thread
		if DB.Select("user_id", "title").First(&t, p.ThreadID).Error == nil {
			title := t.Title
			if len(title) > 40 {
				title = title[:40] + "…"
			}
			_ = PushNotification(p.UserID, t.UserID, "thread_reply",
				"你的帖子有新回复",
				"《"+title+"》",
				"/thread/"+strconv.FormatUint(uint64(p.ThreadID), 10))
		}
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
	// 存量自建论坛补齐房间号（RoomCode 曾因唯一索引导致创建失败/为空）
	BackfillForumRoomCodes()
}

// BackfillForumRoomCodes 为所有缺少房间号的论坛补生成唯一房间号。
// 系统默认论坛保持空房间号（公开可见，无需凭证）；自建论坛必须有房间号。
func BackfillForumRoomCodes() {
	var empty []Forum
	DB.Where("type = ? AND (room_code = '' OR room_code IS NULL)", "custom").
		Find(&empty)
	for _, f := range empty {
		code := GenerateUniqueRoomCode()
		DB.Model(&Forum{}).Where("id = ?", f.ID).Update("room_code", code)
	}
}

// CreateForum 创建论坛
func CreateForum(forum *Forum) error {
	return DB.Create(forum).Error
}

// 房间号字符集：数字+大写字母，剔除易混淆字符（0/O、1/I/L）
const roomCodeChars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// GenerateUniqueRoomCode 生成唯一房间号（6 位，数字+字母）
func GenerateUniqueRoomCode() string {
	for i := 0; i < 10; i++ {
		b := make([]byte, 6)
		for j := range b {
			b[j] = roomCodeChars[int(time.Now().UnixNano()+int64(j+i)*7919)%len(roomCodeChars)]
		}
		code := string(b)
		var count int64
		DB.Model(&Forum{}).Where("room_code = ?", code).Count(&count)
		if count == 0 {
			return code
		}
	}
	// 兜底：时间戳后 6 位
	return fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
}

// GetForumByRoomCode 按房间号查找论坛
func GetForumByRoomCode(code string) (*Forum, error) {
	var forum Forum
	err := DB.Where("room_code = ?", strings.ToUpper(strings.TrimSpace(code))).First(&forum).Error
	if err != nil {
		return nil, err
	}
	return &forum, nil
}

// RoomCodeExists 房间号是否已被占用（不含软删除的论坛）
func RoomCodeExists(code string) bool {
	var count int64
	DB.Model(&Forum{}).Where("room_code = ?", strings.ToUpper(strings.TrimSpace(code))).Count(&count)
	return count > 0
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
