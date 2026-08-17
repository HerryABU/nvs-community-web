package models

import "time"

// XssRule 管理员可配置的 XSS 防御正则规则
// 规则会在 SanitizeUserContent（评论/帖子/章节/博客等所有用户内容）时动态应用，
// 默认动作 escape（转义匹配片段中的 HTML 标签，保留完整文本）；
// 也可选 replace（替换为 Replacement）或 delete（删除匹配内容）。
type XssRule struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:128;not null" json:"name"`          // 规则名称
	Pattern     string    `gorm:"type:text;not null" json:"pattern"`      // 正则表达式（Go RE2 语法）
	Action      string    `gorm:"size:16;default:escape" json:"action"`   // escape/replace/delete
	Replacement string    `gorm:"size:256;default:''" json:"replacement"` // 仅 replace 动作时使用
	Enabled     bool      `gorm:"default:true" json:"enabled"`            // 是否启用
	Description string    `gorm:"type:text" json:"description"`           // 规则说明
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (XssRule) TableName() string { return "xss_rules" }

// ListXssRules 列出所有规则（含禁用）
func ListXssRules() ([]XssRule, error) {
	var rules []XssRule
	err := DB.Order("id asc").Find(&rules).Error
	return rules, err
}

// ListEnabledXssRules 列出所有启用的规则
func ListEnabledXssRules() ([]XssRule, error) {
	var rules []XssRule
	err := DB.Where("enabled = ?", true).Order("id asc").Find(&rules).Error
	return rules, err
}

// GetXssRule 获取单个规则
func GetXssRule(id uint) (*XssRule, error) {
	var rule XssRule
	if err := DB.First(&rule, id).Error; err != nil {
		return nil, err
	}
	return &rule, nil
}

// CreateXssRule 创建规则
func CreateXssRule(rule *XssRule) error {
	return DB.Create(rule).Error
}

// UpdateXssRule 更新规则
func UpdateXssRule(rule *XssRule) error {
	return DB.Save(rule).Error
}

// DeleteXssRule 删除规则
func DeleteXssRule(id uint) error {
	return DB.Delete(&XssRule{}, id).Error
}
