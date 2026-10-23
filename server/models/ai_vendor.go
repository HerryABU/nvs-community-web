package models

import (
	"fmt"
	"strings"
	"time"
)

// ==================== AI 模型厂家字典（管理端可维护） ====================
// 作用：统一「厂商」取值，前台模型广场按厂商筛选、后台模型表单用下拉而非手填。

// AiVendor 厂商
type AiVendor struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:32;not null;uniqueIndex" json:"name"`
	SortOrder int       `gorm:"not null;default:0" json:"sort_order"`
	Enabled   bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (AiVendor) TableName() string { return "ai_vendors" }

// ListAiVendors 厂商列表（onlyEnabled=true 时仅启用的）
func ListAiVendors(onlyEnabled bool) ([]AiVendor, error) {
	var list []AiVendor
	q := DB.Order("sort_order ASC, id ASC")
	if onlyEnabled {
		q = q.Where("enabled = ?", true)
	}
	err := q.Find(&list).Error
	return list, err
}

// CreateAiVendor 新增厂商（名称去重）
func CreateAiVendor(name string, sortOrder int) (*AiVendor, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("厂商名称不能为空")
	}
	if len([]rune(name)) > 32 {
		name = string([]rune(name)[:32])
	}
	var n int64
	DB.Model(&AiVendor{}).Where("name = ?", name).Count(&n)
	if n > 0 {
		return nil, fmt.Errorf("厂商「%s」已存在", name)
	}
	v := &AiVendor{Name: name, SortOrder: sortOrder, Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := DB.Create(v).Error; err != nil {
		return nil, err
	}
	return v, nil
}

// UpdateAiVendor 更新（改名时同步刷新模型里旧厂商名）
func UpdateAiVendor(id uint, name string, sortOrder int, enabled bool) error {
	var v AiVendor
	if err := DB.Where("id = ?", id).First(&v).Error; err != nil {
		return fmt.Errorf("厂商不存在")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("厂商名称不能为空")
	}
	err := DB.Model(&AiVendor{}).Where("id = ?", id).Updates(map[string]interface{}{
		"name": name, "sort_order": sortOrder, "enabled": enabled, "updated_at": time.Now(),
	}).Error
	if err != nil {
		return err
	}
	if name != v.Name {
		// 级联：模型表里的厂商名跟着改（map 更新，零值安全）
		DB.Model(&AiModel{}).Where("vendor = ?", v.Name).UpdateColumn("vendor", name)
	}
	return nil
}

// DeleteAiVendor 删除厂商（仍有模型引用时拒绝，避免悬空筛选）
func DeleteAiVendor(id uint) error {
	var v AiVendor
	if err := DB.Where("id = ?", id).First(&v).Error; err != nil {
		return fmt.Errorf("厂商不存在")
	}
	var n int64
	DB.Model(&AiModel{}).Where("vendor = ?", v.Name).Count(&n)
	if n > 0 {
		return fmt.Errorf("仍有 %d 个模型属于「%s」，请先改为其他厂商", n, v.Name)
	}
	return DB.Where("id = ?", id).Delete(&AiVendor{}).Error
}

// SeedAiVendors 首次启动写入常见厂商（幂等）
func SeedAiVendors() {
	seeds := []string{
		"OpenAI", "Anthropic", "Google", "Midjourney", "DeepSeek",
		"阿里云", "字节跳动", "Moonshot", "智谱", "MiniMax",
		"快手", "Runway", "Stability", "其他",
	}
	for i, name := range seeds {
		var n int64
		DB.Model(&AiVendor{}).Where("name = ?", name).Count(&n)
		if n > 0 {
			continue
		}
		DB.Create(&AiVendor{Name: name, SortOrder: i + 1, Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()})
	}
}
