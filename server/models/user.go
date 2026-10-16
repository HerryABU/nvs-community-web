package models

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// DB 全局数据库实例
var DB *gorm.DB

type User struct {
	ID               uint   `gorm:"primaryKey" json:"id"`
	Username         string `gorm:"size:64;uniqueIndex;not null" json:"username"`
	Email            string `gorm:"size:128;uniqueIndex;not null" json:"email"`
	PasswordHash     string `gorm:"size:256;not null" json:"-"`
	Nickname         string `gorm:"size:64;default:''" json:"nickname"`
	AvatarURL        string `gorm:"size:512;default:''" json:"avatar_url"`
	Signature        string `gorm:"size:256;default:''" json:"signature"`
	Bio              string `gorm:"type:text" json:"bio"`
	Role             string `gorm:"size:32;default:reader" json:"role"`
	SigningKey       string `gorm:"size:64;default:''" json:"-"` // base64(AES-256 key), only set for authors
	// 手机号登录（F10）：普通索引 + 应用层唯一校验（空串唯一索引在多行下冲突，故不用 DB uniqueIndex）
	Phone string `gorm:"size:20;index;default:''" json:"phone,omitempty"`
	// 实名制（F10，公安部令第176号合规）：姓名 AES-256-GCM 加密存储；身份证仅存哈希+脱敏展示
	RealName     string     `gorm:"size:256;default:''" json:"-"`       // 密文（EncryptField 产出）
	IDCardHash   string     `gorm:"size:64;index;default:''" json:"-"`  // sha256(身份证号) 用于查重/防重复实名
	IDCardMasked string     `gorm:"size:32;default:''" json:"id_card_masked,omitempty"`
	RealNameAt   *time.Time `json:"-"`
	// 未成年保护（F21.1）：出生日期 + 未成年人标记（可由生日推算或国家实名 API 返回）
	BirthDate      *time.Time `json:"birth_date,omitempty"`
	MinorProtected bool       `gorm:"default:false" json:"minor_protected"`
	// 微信扫码登录（F21.2）：开放平台 OpenID/UnionID 绑定
	WeChatOpenID  string `gorm:"size:64;index;default:''" json:"-"`
	WeChatUnionID string `gorm:"size:64;default:''" json:"-"`
	// 私信设置（F20）：是否要求互相关注才能发消息（默认开启）
	RequireFollowToMessage bool `gorm:"default:true" json:"require_follow_to_message"`
	RealNameVerified       bool `gorm:"default:false" json:"real_name_verified"`
	// 合规留存：注册IP / 最后登录IP / 最后登录时间（176号令用户信息追溯）
	RegistrationIP string     `gorm:"size:45;default:''" json:"-"`
	LastLoginIP    string     `gorm:"size:45;default:''" json:"-"`
	LastLoginAt    *time.Time `json:"-"`
	LoginFailCount int        `gorm:"default:0" json:"-"`
	LockedUntil    *time.Time `json:"-"`
	MutedUntil     *time.Time `json:"muted_until"`                            // 全局禁言截止时间（null/过去=未禁言）
	MuteReason     string     `gorm:"size:256;default:''" json:"mute_reason"` // 禁言原因
	// 算法推荐偏好（176号令 第七条第九项：算法推荐管理 —— 用户可关闭个性化推荐）
	PersonalizedRecommend bool           `gorm:"default:false" json:"personalized_recommend"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
	DeletedAt             gorm.DeletedAt `gorm:"index" json:"-"` // 合规留存：封禁/删除用户采用软删除（可追溯）
	// 警方取管台：由管理员提权而来（不可自注册），记录提权来源与时间为审计留痕
	PoliceGrantedBy string     `gorm:"size:64;default:''" json:"police_granted_by"`
	PoliceGrantedAt *time.Time `json:"police_granted_at"`
}

func (User) TableName() string {
	return "users"
}

// IsMuted 是否处于全局禁言状态
func (u *User) IsMuted() bool {
	return u.MutedUntil != nil && u.MutedUntil.After(time.Now())
}

func CreateUser(user *User) error {
	return DB.Create(user).Error
}

func GetUserByEmail(email string) (*User, error) {
	var user User
	err := DB.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByPhone 按手机号查询（F10；应用层保证唯一性）
func GetUserByPhone(phone string) (*User, error) {
	var user User
	err := DB.Where("phone = ?", phone).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// PhoneTaken 手机号是否已被占用（排除自身）
func PhoneTaken(phone string, excludeID uint) (bool, error) {
	var n int64
	q := DB.Model(&User{}).Where("phone = ?", phone)
	if excludeID > 0 {
		q = q.Where("id != ?", excludeID)
	}
	if err := q.Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// GetUserByWeChatOpenID 按微信 OpenID 查询（F21.2）
func GetUserByWeChatOpenID(openID string) (*User, error) {
	var user User
	err := DB.Where("we_chat_open_id = ?", openID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func GetUserByID(id uint) (*User, error) {
	var user User
	err := DB.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func UpdateUser(user *User) error {
	return DB.Save(user).Error
}

// SanitizeUserEmails 对用户列表进行邮箱脱敏处理
func SanitizeUserEmails(users []User) {
	for i := range users {
		users[i].Email = sanitizeEmail(users[i].Email)
	}
}

// sanitizeEmail 邮箱脱敏：只显示前3位和域名
func sanitizeEmail(email string) string {
	if email == "" {
		return ""
	}
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return email
	}
	local := parts[0]
	domain := parts[1]
	if len(local) <= 3 {
		return local + "***@" + domain
	}
	return local[:3] + "***@" + domain
}
