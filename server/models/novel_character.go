package models

import (
	"fmt"
	"strings"
	"time"
)

// ==================== 作品角色表（话本气泡体角色管理） ====================
// 话本气泡体的「角色」需要跨章节复用（头像、主题色、默认左右），
// 因此角色独立成表挂在作品下，而不是塞进章节正文。
//
// 角色名与作品内唯一（同一作品里不允许两个同名角色，否则气泡归属会歧义）。
// 零值纪律：头像/颜色/简介均可为空，创建与更新一律走 map 精确写。

// NovelCharacter 作品角色
type NovelCharacter struct {
	ID      uint `gorm:"primaryKey" json:"id"`
	NovelID uint `gorm:"not null;index" json:"novel_id"`
	// Name 角色名（作品内唯一），气泡里「角色：台词」用的就是它
	Name string `gorm:"size:32;not null" json:"name"`
	// Avatar 头像图片地址（可空）。为空时前端用 Name 首字 + Color 生成文字头像
	Avatar string `gorm:"size:256;default:''" json:"avatar"`
	// Color 主题色（#rrggbb 或 CSS 颜色名），气泡描边与文字头像底色
	Color string `gorm:"size:16;default:''" json:"color"`
	// Side 默认左右：left / right。角色在气泡编辑器里的默认侧，新插入气泡沿用
	Side string `gorm:"size:8;not null;default:'left'" json:"side"`
	// Intro 角色简介（可空）
	Intro string `gorm:"size:256;default:''" json:"intro"`
	// SortOrder 排序（小在前）
	SortOrder int       `gorm:"not null;default:0" json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (NovelCharacter) TableName() string { return "novel_characters" }

// NormalizeCharacterSide 归一化左右（非法值回落 left）
func NormalizeCharacterSide(side string) string {
	switch strings.ToLower(strings.TrimSpace(side)) {
	case "right", "r", "右":
		return "right"
	default:
		return "left"
	}
}

// ListNovelCharacters 作品角色列表（按排序 → id）
func ListNovelCharacters(novelID uint) ([]NovelCharacter, error) {
	var list []NovelCharacter
	err := DB.Where("novel_id = ?", novelID).
		Order("sort_order ASC, id ASC").Find(&list).Error
	return list, err
}

// GetNovelCharacter 取单个角色
func GetNovelCharacter(id uint) (*NovelCharacter, error) {
	var c NovelCharacter
	if err := DB.Where("id = ?", id).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// CreateNovelCharacter 新建角色（作品内同名拒绝）
func CreateNovelCharacter(c *NovelCharacter) error {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return fmt.Errorf("角色名不能为空")
	}
	if len([]rune(c.Name)) > 32 {
		return fmt.Errorf("角色名过长（最多 32 字）")
	}
	var n int64
	DB.Model(&NovelCharacter{}).Where("novel_id = ? AND name = ?", c.NovelID, c.Name).Count(&n)
	if n > 0 {
		return fmt.Errorf("角色「%s」已存在", c.Name)
	}
	c.Side = NormalizeCharacterSide(c.Side)
	now := time.Now()
	c.CreatedAt = now
	c.UpdatedAt = now
	return DB.Create(c).Error
}

// UpdateNovelCharacterFields map 精确更新（零值安全）；改名需检查作品内重名
func UpdateNovelCharacterFields(id uint, fields map[string]interface{}) error {
	if name, ok := fields["name"]; ok {
		n := strings.TrimSpace(fmt.Sprint(name))
		if n == "" {
			return fmt.Errorf("角色名不能为空")
		}
		fields["name"] = n
		var self NovelCharacter
		if err := DB.Where("id = ?", id).First(&self).Error; err != nil {
			return fmt.Errorf("角色不存在")
		}
		var dup int64
		DB.Model(&NovelCharacter{}).Where("novel_id = ? AND name = ? AND id <> ?", self.NovelID, n, id).Count(&dup)
		if dup > 0 {
			return fmt.Errorf("角色「%s」已存在", n)
		}
	}
	if side, ok := fields["side"]; ok {
		fields["side"] = NormalizeCharacterSide(fmt.Sprint(side))
	}
	if len(fields) == 0 {
		return nil
	}
	fields["updated_at"] = time.Now()
	return DB.Model(&NovelCharacter{}).Where("id = ?", id).Updates(fields).Error
}

// DeleteNovelCharacter 删除角色
func DeleteNovelCharacter(id uint) error {
	return DB.Where("id = ?", id).Delete(&NovelCharacter{}).Error
}
