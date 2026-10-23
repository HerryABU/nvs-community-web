package models

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"nvs-server/utils"
)

// ==================== 用户自备 API Key（§5.5：BYOK） ====================
// 用户自带 OpenAI 兼容上游密钥：AES-256-GCM 加密落库、只写不读。
// 命中自备密钥的调用不扣平台代币（费用由用户与上游结算）。

// MaxUserAiKeys 每用户最多自备密钥数（防滥用）
const MaxUserAiKeys = 5

// UserAiKey 用户自备上游密钥
type UserAiKey struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"index;not null" json:"user_id"`
	Label      string    `gorm:"size:64;not null" json:"label"`       // 备注名（如「我的 DeepSeek」）
	BaseURL    string    `gorm:"size:256;not null" json:"base_url"`   // 上游地址（不含 /chat/completions）
	KeyEnc     string    `gorm:"type:text" json:"-"`                 // 密钥密文，不回传前端
	ModelID    string    `gorm:"size:128;default:''" json:"model_id"` // 默认上游模型标识（可空，调用时覆盖）
	Enabled    bool      `gorm:"not null;default:true" json:"enabled"`
	IsDefault  bool      `gorm:"not null;default:false" json:"is_default"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (UserAiKey) TableName() string { return "user_ai_keys" }

// DecryptApiKey 解密用户密钥
func (k *UserAiKey) DecryptApiKey() (string, error) {
	if k.KeyEnc == "" {
		return "", errors.New("密钥为空")
	}
	return utils.DecryptField(k.KeyEnc)
}

// ListUserAiKeys 我的自备密钥（不回传密钥本体）
func ListUserAiKeys(userID uint) ([]UserAiKey, error) {
	var list []UserAiKey
	err := DB.Where("user_id = ?", userID).Order("is_default DESC, id ASC").Find(&list).Error
	return list, err
}

// GetUserAiKey 按 ID 读取（含归属校验，越权返回 nil, nil）
func GetUserAiKey(id, userID uint) (*UserAiKey, error) {
	var k UserAiKey
	if err := DB.Where("id = ? AND user_id = ?", id, userID).First(&k).Error; err != nil {
		return nil, err
	}
	return &k, nil
}

// CreateUserAiKey 新增自备密钥（首个自动设为默认）
func CreateUserAiKey(userID uint, label, baseURL, apiKey, modelID string) (*UserAiKey, error) {
	label = strings.TrimSpace(label)
	baseURL = strings.TrimSpace(baseURL)
	apiKey = strings.TrimSpace(apiKey)
	modelID = strings.TrimSpace(modelID)
	if label == "" {
		label = "我的密钥"
	}
	if baseURL == "" || apiKey == "" {
		return nil, errors.New("上游地址与密钥均为必填")
	}
	var cnt int64
	if err := DB.Model(&UserAiKey{}).Where("user_id = ?", userID).Count(&cnt).Error; err != nil {
		return nil, err
	}
	if int(cnt) >= MaxUserAiKeys {
		return nil, errors.New("自备密钥数量已达上限（最多 5 个）")
	}
	enc, err := utils.EncryptField(apiKey)
	if err != nil {
		return nil, err
	}
	isDefault := cnt == 0
	now := time.Now()
	k := &UserAiKey{
		UserID: userID, Label: label, BaseURL: baseURL, KeyEnc: enc,
		ModelID: modelID, Enabled: true, IsDefault: isDefault,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := DB.Create(k).Error; err != nil {
		return nil, err
	}
	return k, nil
}

// DeleteUserAiKey 删除（归属校验；删默认后提升剩余第一个为默认）
func DeleteUserAiKey(id, userID uint) error {
	k, err := GetUserAiKey(id, userID)
	if err != nil || k == nil {
		return errors.New("密钥不存在")
	}
	if err := DB.Where("id = ? AND user_id = ?", id, userID).Delete(&UserAiKey{}).Error; err != nil {
		return err
	}
	if k.IsDefault {
		var rest []UserAiKey
		DB.Where("user_id = ?", userID).Order("id ASC").Limit(1).Find(&rest)
		if len(rest) > 0 {
			_ = DB.Model(&UserAiKey{}).Where("id = ?", rest[0].ID).
				Update("is_default", true).Error
		}
	}
	return nil
}

// SetDefaultUserAiKey 设默认（同事务内先清零再置位，防并发双默认）
func SetDefaultUserAiKey(id, userID uint) error {
	k, err := GetUserAiKey(id, userID)
	if err != nil || k == nil {
		return errors.New("密钥不存在")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&UserAiKey{}).Where("user_id = ?", userID).
			Update("is_default", false).Error; err != nil {
			return err
		}
		return tx.Model(&UserAiKey{}).Where("id = ?", id).
			Update("is_default", true).Error
	})
}

// ResolveUserAiKey 解析本次调用使用的自备密钥：指定 ID 优先，否则默认密钥。
// 用户未配置/不可用返回 nil, nil。
func ResolveUserAiKey(userID, keyID uint) (*UserAiKey, error) {
	q := DB.Where("user_id = ? AND enabled = ?", userID, true)
	if keyID > 0 {
		q = q.Where("id = ?", keyID)
	} else {
		q = q.Where("is_default = ?", true)
	}
	var k UserAiKey
	if err := q.First(&k).Error; err != nil {
		return nil, nil // 未配置 → 回落平台池
	}
	return &k, nil
}

// TouchUserAiKey 记录最近使用时间
func TouchUserAiKey(id uint) {
	now := time.Now()
	_ = DB.Model(&UserAiKey{}).Where("id = ?", id).
		Update("last_used_at", &now).Error
}
