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
	ID   uint   `gorm:"primaryKey" json:"id"`
	Name string `gorm:"size:32;not null;uniqueIndex" json:"name"`
	// Prefix 厂商前缀（模型 ID 约定：前缀-版本-亚版本，如 deepseek-v3.1-turbo 的 deepseek）。
	// 管理员可维护；导入模型时按此前缀匹配厂商，便于归类与「主名字」展示。
	Prefix    string    `gorm:"size:64;default:'';index" json:"prefix"`
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

// CreateAiVendor 新增厂商（名称去重）。prefix 可空：缺省由厂商名推导（小写、去非字母数字）。
func CreateAiVendor(name string, sortOrder int, prefix ...string) (*AiVendor, error) {
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
	pfx := ""
	if len(prefix) > 0 {
		pfx = strings.TrimSpace(prefix[0])
	}
	if pfx == "" {
		// 默认前缀：厂商名转小写、去除非字母数字字符
		pfx = derivePrefix(name)
	}
	v := &AiVendor{Name: name, Prefix: pfx, SortOrder: sortOrder, Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := DB.Create(v).Error; err != nil {
		return nil, err
	}
	return v, nil
}

// UpdateAiVendor 更新（改名时同步刷新模型里旧厂商名）。prefix 可空：不传则不改动前缀。
func UpdateAiVendor(id uint, name string, sortOrder int, enabled bool, prefix ...string) error {
	var v AiVendor
	if err := DB.Where("id = ?", id).First(&v).Error; err != nil {
		return fmt.Errorf("厂商不存在")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("厂商名称不能为空")
	}
	fields := map[string]interface{}{
		"name": name, "sort_order": sortOrder, "enabled": enabled, "updated_at": time.Now(),
	}
	if len(prefix) > 0 {
		fields["prefix"] = strings.TrimSpace(prefix[0])
	}
	err := DB.Model(&AiVendor{}).Where("id = ?", id).Updates(fields).Error
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

// derivePrefix 由厂商名推导前缀（小写、去非字母数字）。
// 用于「新厂商未填前缀」与「老库升级后回填」（老版本 ai_vendors 没有 prefix 列）。
//
// ⚠️ 必须先 ToLower 再过滤：反过来写会把大写字母当非法字符丢掉
// （"OpenAI" → "pen"、"My Vendor" → "yendor"），前缀从此永远匹配不上。
func derivePrefix(name string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		default:
			return -1
		}
	}, strings.ToLower(strings.TrimSpace(name)))
}

// SeedAiVendors 首次启动写入常见厂商（幂等），并回填老数据缺失的前缀。
//
// ⚠️ 回填很重要：老版本的 ai_vendors 表没有 prefix 列，AutoMigrate 只加列、
// 已有行一律是空串。不做回填的话，「按模型 ID 前缀匹配厂商」在老站升级后
// 会静默失效（前缀永远匹配不上），而且界面上看起来一切正常。
func SeedAiVendors() {
	seeds := []struct {
		name   string
		prefix string
	}{
		{"OpenAI", "openai"}, {"Anthropic", "claude"}, {"Google", "gemini"},
		{"Midjourney", "midjourney"}, {"DeepSeek", "deepseek"}, {"阿里云", "qwen"},
		{"字节跳动", "doubao"}, {"Moonshot", "moonshot"}, {"智谱", "glm"},
		{"MiniMax", "minimax"}, {"快手", "kling"}, {"Runway", "runway"},
		{"Stability", "stable-diffusion"}, {"其他", ""},
	}
	now := time.Now()
	for i, s := range seeds {
		pfx := s.prefix
		if pfx == "" {
			pfx = derivePrefix(s.name)
		}
		var v AiVendor
		if err := DB.Where("name = ?", s.name).First(&v).Error; err == nil {
			// 已存在：仅当前缀为空时回填，不覆盖管理员手工改过的值
			if strings.TrimSpace(v.Prefix) == "" && pfx != "" {
				DB.Model(&AiVendor{}).Where("id = ?", v.ID).
					Updates(map[string]interface{}{"prefix": pfx, "updated_at": now})
			}
			continue
		}
		DB.Create(&AiVendor{Name: s.name, Prefix: pfx, SortOrder: i + 1, Enabled: true, CreatedAt: now, UpdatedAt: now})
	}
	// 非预置厂商（管理员自建 / 老数据）：同样按名字推导回填空前缀
	var blank []AiVendor
	DB.Where("prefix IS NULL OR prefix = ''").Find(&blank)
	for _, v := range blank {
		p := derivePrefix(v.Name)
		if p == "" {
			continue
		}
		DB.Model(&AiVendor{}).Where("id = ?", v.ID).
			Updates(map[string]interface{}{"prefix": p, "updated_at": now})
	}
}

// MatchVendorByPrefix 按模型 ID 前缀匹配厂商（大小写不敏感）。
// 先精确比对 prefix 字段，再比对厂商名；命中返回厂商指针，否则 nil。
// 用于导入模型时把 raw_id 的「前缀-版本-亚版本」归类到对应厂商。
func MatchVendorByPrefix(prefix string) *AiVendor {
	p := strings.ToLower(strings.TrimSpace(prefix))
	if p == "" {
		return nil
	}
	var list []AiVendor
	if err := DB.Find(&list).Error; err != nil || len(list) == 0 {
		return nil
	}
	for i := range list {
		if strings.ToLower(list[i].Prefix) == p {
			return &list[i]
		}
	}
	for i := range list {
		if strings.ToLower(list[i].Name) == p {
			return &list[i]
		}
	}
	return nil
}

// MatchVendorName 按厂商名精确匹配（兼容旧数据 vendor 存的是厂商名）。
func MatchVendorName(name string) *AiVendor {
	n := strings.TrimSpace(name)
	if n == "" {
		return nil
	}
	var v AiVendor
	if err := DB.Where("name = ?", n).First(&v).Error; err != nil {
		return nil
	}
	return &v
}
