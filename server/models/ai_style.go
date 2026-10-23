package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ==================== 作品风格字典 ====================
// 绘画/视频发布作品时从字典选风格；广场按字典生成筛选 chips。
// 保留 AiMediaTask.Style 字符串字段，字典只做规范化与展示，零数据迁移。

// AiStyle 风格
type AiStyle struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Kind      string    `gorm:"size:16;not null;default:image;index" json:"kind"` // image / video
	Name      string    `gorm:"size:32;not null" json:"name"`
	SortOrder int       `gorm:"not null;default:0" json:"sort_order"`
	Enabled   bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (AiStyle) TableName() string { return "ai_styles" }

// ListAiStyles 风格列表（kind 为空 = 全部）
func ListAiStyles(kind string, onlyEnabled bool) ([]AiStyle, error) {
	q := DB.Order("sort_order ASC, id ASC")
	if k := strings.TrimSpace(kind); k != "" {
		q = q.Where("kind = ?", NormalizeAiKind(k))
	}
	if onlyEnabled {
		q = q.Where("enabled = ?", true)
	}
	var list []AiStyle
	err := q.Find(&list).Error
	return list, err
}

// CreateAiStyle 新增风格（同类型下重名拒绝）
func CreateAiStyle(kind, name string, sortOrder int) (*AiStyle, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("风格名称不能为空")
	}
	if len([]rune(name)) > 32 {
		name = string([]rune(name)[:32])
	}
	kind = NormalizeAiKind(kind)
	var n int64
	DB.Model(&AiStyle{}).Where("kind = ? AND name = ?", kind, name).Count(&n)
	if n > 0 {
		return nil, fmt.Errorf("该类型下已存在风格「%s」", name)
	}
	st := &AiStyle{Kind: kind, Name: name, SortOrder: sortOrder, Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := DB.Create(st).Error; err != nil {
		return nil, err
	}
	return st, nil
}

// UpdateAiStyle 更新（改名时同步刷新作品里的风格名）
func UpdateAiStyle(id uint, name string, sortOrder int, enabled bool) error {
	var st AiStyle
	if err := DB.Where("id = ?", id).First(&st).Error; err != nil {
		return fmt.Errorf("风格不存在")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("风格名称不能为空")
	}
	if err := DB.Model(&AiStyle{}).Where("id = ?", id).Updates(map[string]interface{}{
		"name": name, "sort_order": sortOrder, "enabled": enabled, "updated_at": time.Now(),
	}).Error; err != nil {
		return err
	}
	if name != st.Name {
		DB.Model(&AiMediaTask{}).Where("style = ?", st.Name).UpdateColumn("style", name)
	}
	return nil
}

// DeleteAiStyle 删除风格（不阻断，作品里的历史 style 字符串保留）
func DeleteAiStyle(id uint) error {
	res := DB.Where("id = ?", id).Delete(&AiStyle{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("风格不存在")
	}
	return nil
}

// SeedAiStyles 内置常见风格（幂等）
func SeedAiStyles() {
	seeds := map[string][]string{
		AiKindImage: {"写实摄影", "电商设计", "插画设计", "动漫游戏", "国风", "海报设计", "建筑空间", "头像写真"},
		AiKindVideo: {"空镜", "人物特写", "转场", "广告片", "动画", "分镜叙事"},
	}
	i := 0
	for kind, names := range seeds {
		for _, name := range names {
			i++
			var n int64
			DB.Model(&AiStyle{}).Where("kind = ? AND name = ?", kind, name).Count(&n)
			if n > 0 {
				continue
			}
			DB.Create(&AiStyle{Kind: kind, Name: name, SortOrder: i, Enabled: true, CreatedAt: time.Now(), UpdatedAt: time.Now()})
		}
	}
}

// ==================== 扣子默认配置 ====================

// AiCozeDefault 扣子全局默认值（智能体未填时继承）
type AiCozeDefault struct {
	BaseURL        string `json:"base_url"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

// GetAiCozeDefault 读取（默认国内域名 + 150s）
func GetAiCozeDefault() AiCozeDefault {
	out := AiCozeDefault{BaseURL: "https://api.coze.cn", TimeoutSeconds: 150}
	raw := GetPlatformConfig("ai_coze_default")
	if strings.TrimSpace(raw) == "" {
		return out
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return AiCozeDefault{BaseURL: "https://api.coze.cn", TimeoutSeconds: 150}
	}
	if strings.TrimSpace(out.BaseURL) == "" {
		out.BaseURL = "https://api.coze.cn"
	}
	if out.TimeoutSeconds < 30 || out.TimeoutSeconds > 600 {
		out.TimeoutSeconds = 150
	}
	return out
}

// SetAiCozeDefault 保存
func SetAiCozeDefault(d AiCozeDefault) error {
	if strings.TrimSpace(d.BaseURL) == "" {
		d.BaseURL = "https://api.coze.cn"
	}
	if d.TimeoutSeconds < 30 || d.TimeoutSeconds > 600 {
		d.TimeoutSeconds = 150
	}
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return SetPlatformConfig("ai_coze_default", string(b))
}
