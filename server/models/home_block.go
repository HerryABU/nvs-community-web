// home_block.go — 自定义主页区块（F15）。
// 管理员可增删/排序/启停首页区块，并一键套用布局预设：
//   - mixed   混合一条龙（默认： hero + 公告 + 模块区）
//   - blog    博客首页风格（hero + 公告 + 博客大区块 + 小说副栏）
//   - forum   论坛首页风格（hero + 公告 + 论坛区块 + 最新帖 + 博客副栏）
//   - minimal 极简小说（hero + 小说模块）
//
// 区块类型与 config JSON：
//
//	hero     {site_title, subtitle, show_categories}
//	notice   {max_items}
//	modules  {modules: ["novels","blogs","forums"], featured_first}
//	blogs    {limit, view: list|grid}
//	forums   {limit}
//	threads  {limit}                      最新帖（自动排除加密/待审论坛）
//	custom   {html}                       自定义 HTML（服务端消毒后落库）
//	links    {items: [{title, url}]}
package models

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// HomeBlock 首页区块
type HomeBlock struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Type      string         `gorm:"size:32;not null;index" json:"type"` // hero/notice/modules/blogs/forums/threads/custom/links
	Title     string         `gorm:"size:128;default:''" json:"title"`
	Config    string         `gorm:"type:text" json:"config"` // 按 type 解析的 JSON 配置
	SortOrder int            `gorm:"default:0;index" json:"sort_order"`
	Enabled   bool           `gorm:"default:true" json:"enabled"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (HomeBlock) TableName() string { return "home_blocks" }

// 区块类型常量
const (
	HomeBlockHero    = "hero"
	HomeBlockNotice  = "notice"
	HomeBlockModules = "modules"
	HomeBlockBlogs   = "blogs"
	HomeBlockForums  = "forums"
	HomeBlockThreads = "threads"
	HomeBlockCustom  = "custom"
	HomeBlockLinks   = "links"
)

// validHomeBlockTypes 合法区块类型白名单
var validHomeBlockTypes = map[string]bool{
	HomeBlockHero: true, HomeBlockNotice: true, HomeBlockModules: true,
	HomeBlockBlogs: true, HomeBlockForums: true, HomeBlockThreads: true,
	HomeBlockCustom: true, HomeBlockLinks: true,
}

// IsValidHomeBlockType 区块类型是否合法
func IsValidHomeBlockType(t string) bool { return validHomeBlockTypes[t] }

// ─────────────────────────────────────────────────────────────
// CRUD
// ─────────────────────────────────────────────────────────────

// ListHomeBlocks 全部区块（含禁用），按 sort_order 升序
func ListHomeBlocks() ([]HomeBlock, error) {
	var list []HomeBlock
	err := DB.Order("sort_order ASC, id ASC").Find(&list).Error
	return list, err
}

// ListEnabledHomeBlocks 仅启用区块，按 sort_order 升序
func ListEnabledHomeBlocks() ([]HomeBlock, error) {
	var list []HomeBlock
	err := DB.Where("enabled = ?", true).Order("sort_order ASC, id ASC").Find(&list).Error
	return list, err
}

// GetHomeBlock 按 ID 取区块
func GetHomeBlock(id uint) (*HomeBlock, error) {
	var b HomeBlock
	if err := DB.First(&b, id).Error; err != nil {
		return nil, err
	}
	return &b, nil
}

// CountHomeBlocks 区块数量（判断是否需播种默认布局）
func CountHomeBlocks() int64 {
	var n int64
	DB.Model(&HomeBlock{}).Count(&n)
	return n
}

// CreateHomeBlock 新建区块
func CreateHomeBlock(b *HomeBlock) error {
	return DB.Create(b).Error
}

// UpdateHomeBlock 更新区块（map 精确写列，规避 GORM 零值坑）
func UpdateHomeBlock(id uint, updates map[string]interface{}) error {
	return DB.Model(&HomeBlock{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteHomeBlock 删除区块（软删除，合规留存）
func DeleteHomeBlock(id uint) error {
	return DB.Delete(&HomeBlock{}, id).Error
}

// ReorderHomeBlocks 按 ids 顺序重排 sort_order（事务内完成）
func ReorderHomeBlocks(ids []uint) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		for i, id := range ids {
			if err := tx.Model(&HomeBlock{}).Where("id = ?", id).
				Update("sort_order", i*10).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ─────────────────────────────────────────────────────────────
// 布局预设与默认播种
// ─────────────────────────────────────────────────────────────

// HomeBlockPreset 布局预设
type HomeBlockPreset struct {
	Key    string      `json:"key"`  // mixed/blog/forum/minimal
	Name   string      `json:"name"` // 展示名
	Desc   string      `json:"desc"` // 一句话描述
	Blocks []HomeBlock `json:"-"`    // 预设区块（内部使用，不直接序列化给前端）
}

func hp(t, title, config string, enabled bool) HomeBlock {
	return HomeBlock{Type: t, Title: title, Config: config, Enabled: enabled}
}

// HomeBlockPresets 全部布局预设
func HomeBlockPresets() []HomeBlockPreset {
	return []HomeBlockPreset{
		{
			Key: "mixed", Name: "混合一条龙", Desc: "小说/博客/论坛同页混排，信息最全（默认）",
			Blocks: []HomeBlock{
				hp(HomeBlockHero, "", `{"show_categories":true}`, true),
				hp(HomeBlockNotice, "", `{"max_items":3}`, true),
				hp(HomeBlockModules, "", `{"modules":["novels","blogs","forums"],"featured_first":true}`, true),
			},
		},
		{
			Key: "blog", Name: "博客首页风格", Desc: "博客列表为主角，小说/论坛作副栏，适合创作型社区",
			Blocks: []HomeBlock{
				hp(HomeBlockHero, "", `{"show_categories":true}`, true),
				hp(HomeBlockNotice, "", `{"max_items":2}`, true),
				hp(HomeBlockBlogs, "最新博客", `{"limit":8,"view":"list"}`, true),
				hp(HomeBlockModules, "", `{"modules":["novels"],"featured_first":false}`, true),
				hp(HomeBlockThreads, "论坛新帖", `{"limit":6}`, true),
			},
		},
		{
			Key: "forum", Name: "论坛首页风格", Desc: "论坛动态打头，最新帖 + 博客副栏，适合讨论型社区",
			Blocks: []HomeBlock{
				hp(HomeBlockHero, "", `{"show_categories":false}`, true),
				hp(HomeBlockNotice, "", `{"max_items":2}`, true),
				hp(HomeBlockThreads, "最新讨论", `{"limit":10}`, true),
				hp(HomeBlockForums, "版块大厅", `{"limit":8}`, true),
				hp(HomeBlockBlogs, "最新博客", `{"limit":4,"view":"list"}`, true),
			},
		},
		{
			Key: "minimal", Name: "极简小说", Desc: "只保留头图与小说模块，最轻量",
			Blocks: []HomeBlock{
				hp(HomeBlockHero, "", `{"show_categories":true}`, true),
				hp(HomeBlockModules, "", `{"modules":["novels"],"featured_first":true}`, true),
			},
		},
	}
}

// FindHomeBlockPreset 按 key 找预设
func FindHomeBlockPreset(key string) *HomeBlockPreset {
	for i := range HomeBlockPresets() {
		p := &HomeBlockPresets()[i]
		if p.Key == key {
			return p
		}
	}
	return nil
}

// SeedDefaultHomeBlocks 空库时播种「混合一条龙」默认布局（幂等）
func SeedDefaultHomeBlocks() error {
	if CountHomeBlocks() > 0 {
		return nil
	}
	preset := FindHomeBlockPreset("mixed")
	if preset == nil {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		for i := range preset.Blocks {
			b := preset.Blocks[i]
			b.SortOrder = i * 10
			if err := tx.Create(&b).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// ApplyHomeBlockPreset 套用预设：软删除旧区块 + 事务内创建新布局
func ApplyHomeBlockPreset(key string) error {
	preset := FindHomeBlockPreset(key)
	if preset == nil {
		return gorm.ErrInvalidData
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("1 = 1").Delete(&HomeBlock{}).Error; err != nil {
			return err
		}
		for i := range preset.Blocks {
			b := preset.Blocks[i]
			b.SortOrder = i * 10
			if err := tx.Create(&b).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// HomeBlockConfigJSON 校验并规范化 config（必须是 JSON 对象或空）
func HomeBlockConfigJSON(config string) (string, error) {
	if config == "" {
		return "{}", nil
	}
	var v map[string]interface{}
	if err := json.Unmarshal([]byte(config), &v); err != nil {
		return "", err
	}
	out, _ := json.Marshal(v)
	return string(out), nil
}
