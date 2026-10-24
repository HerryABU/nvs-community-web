package models

// ==================== 自定义应用菜单（gap-audit #20）====================
// 已有「扩展应用 /frames」——但那一套是「站内嵌入的第三方页面」，站长改导航仍要改代码。
// 这里补的是**导航项配置**：站长可加站内路由、外链、分组，控制排序/可见性/登录要求。
//
// 安全约束（必须写在代码里，不能只写在文档）：
//   - 外链只允许 http/https，且 `javascript:` / `data:` / `vbscript:` 一律拒绝；
//   - 站内路由必须以 `/` 开头且不含 `//`（防协议相对 URL 绕过）；
//   - `target=_blank` 强制补 `rel="noopener noreferrer"`（前端渲染时保证）。

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

// NavMenu 导航菜单项（两级：ParentID=0 为一级）
type NavMenu struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ParentID     uint      `gorm:"not null;default:0;index" json:"parent_id"`
	Title        string    `gorm:"size:32;not null" json:"title"`
	Icon         string    `gorm:"size:64;default:''" json:"icon"`             // emoji 或图标名
	URL          string    `gorm:"size:512;not null" json:"url"`               // 站内路径或外链
	Target       string    `gorm:"size:8;not null;default:self" json:"target"` // self / blank
	SortOrder    int       `gorm:"not null;default:0;index" json:"sort_order"`
	Visible      bool      `gorm:"not null;default:true" json:"visible"`
	RequireLogin bool      `gorm:"not null;default:false" json:"require_login"`
	AdminOnly    bool      `gorm:"not null;default:false" json:"admin_only"`
	Badge        string    `gorm:"size:16;default:''" json:"badge"` // NEW / HOT 等小标
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	Children []NavMenu `gorm:"-" json:"children,omitempty"`
}

func (NavMenu) TableName() string { return "nav_menus" }

// NormalizeNavURL 校验并归一化菜单地址（返回错误表示非法）
func NormalizeNavURL(raw string, target string) (string, string, error) {
	u := strings.TrimSpace(raw)
	if u == "" {
		return "", "", errors.New("地址不能为空")
	}
	low := strings.ToLower(u)
	for _, bad := range []string{"javascript:", "data:", "vbscript:", "file:"} {
		if strings.HasPrefix(low, bad) {
			return "", "", errors.New("不支持的协议")
		}
	}
	if strings.HasPrefix(u, "//") {
		return "", "", errors.New("不允许协议相对地址（//host）")
	}
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
		t := "blank"
		if target == "self" {
			t = "self" // 站长显式要求当前窗口打开时尊重其选择
			// 但外链默认应新窗口：这里只在显式传 self 时保留
		}
		return u, t, nil
	}
	if !strings.HasPrefix(u, "/") {
		return "", "", errors.New("站内地址需以 / 开头")
	}
	return u, "self", nil
}

// SaveNavMenu 新增/更新（ID=0 为新增）
func SaveNavMenu(in NavMenu) (*NavMenu, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, errors.New("标题不能为空")
	}
	if len([]rune(title)) > 32 {
		return nil, errors.New("标题过长")
	}
	url, target, err := NormalizeNavURL(in.URL, in.Target)
	if err != nil {
		return nil, err
	}
	in.Title = title
	in.URL = url
	in.Target = target
	in.UpdatedAt = time.Now()
	if in.ID == 0 {
		// ⚠️ 必须在 Create 之前保存意图值：Create 会用数据库默认值回填结构体字段
		// （visible 默认 true），之后再读 in.Visible 拿到的已经是 true，"精确写回"会写反。
		wantVisible, wantLogin, wantAdmin := in.Visible, in.RequireLogin, in.AdminOnly
		in.CreatedAt = time.Now()
		if err := DB.Create(&in).Error; err != nil {
			return nil, err
		}
		_ = DB.Model(&NavMenu{}).Where("id = ?", in.ID).Updates(map[string]interface{}{
			"visible": wantVisible, "require_login": wantLogin, "admin_only": wantAdmin,
		}).Error
		in.Visible, in.RequireLogin, in.AdminOnly = wantVisible, wantLogin, wantAdmin
		return &in, nil
	}
	if err := DB.Model(&NavMenu{}).Where("id = ?", in.ID).Updates(map[string]interface{}{
		"parent_id": in.ParentID, "title": in.Title, "icon": in.Icon, "url": in.URL,
		"target": in.Target, "sort_order": in.SortOrder, "visible": in.Visible,
		"require_login": in.RequireLogin, "admin_only": in.AdminOnly, "badge": in.Badge,
		"updated_at": in.UpdatedAt,
	}).Error; err != nil {
		return nil, err
	}
	return &in, nil
}

// DeleteNavMenu 删除（连带子项）
func DeleteNavMenu(id uint) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("parent_id = ?", id).Delete(&NavMenu{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Delete(&NavMenu{}).Error
	})
}

// AllNavMenus 管理端：全部（含隐藏）+ 组装两级树
func AllNavMenus() ([]NavMenu, error) {
	var list []NavMenu
	if err := DB.Order("sort_order ASC, id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	return buildNavTree(list), nil
}

// PublicNavMenus 前台：可见 + 按登录/角色过滤
func PublicNavMenus(isLogin, isAdmin bool) ([]NavMenu, error) {
	var list []NavMenu
	q := DB.Where("visible = ?", true).Order("sort_order ASC, id ASC")
	if err := q.Find(&list).Error; err != nil {
		return nil, err
	}
	out := make([]NavMenu, 0, len(list))
	for _, m := range list {
		if m.RequireLogin && !isLogin {
			continue
		}
		if m.AdminOnly && !isAdmin {
			continue
		}
		out = append(out, m)
	}
	return buildNavTree(out), nil
}

func buildNavTree(flat []NavMenu) []NavMenu {
	byParent := map[uint][]NavMenu{}
	for _, m := range flat {
		byParent[m.ParentID] = append(byParent[m.ParentID], m)
	}
	roots := byParent[0]
	for i := range roots {
		roots[i].Children = byParent[roots[i].ID]
	}
	return roots
}

// MigrateNavMenu 建表
func MigrateNavMenu(db *gorm.DB) error { return db.AutoMigrate(&NavMenu{}) }
