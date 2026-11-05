package aiops

import (
	"fmt"
	"nvs-server/models"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupAiNavTestDB(t *testing.T) {
	t.Helper()
	dsn := fmt.Sprintf("file:ainav_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存测试库失败: %v", err)
	}
	if err := db.AutoMigrate(&models.PlatformConfig{}); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	old := models.DB
	models.DB = db
	t.Cleanup(func() { models.DB = old })
}

// 站内面板与外部链接都能存；前台只拿到已启用项，且顺序保持
func TestAiNavItems_PanelAndLink(t *testing.T) {
	setupAiNavTestDB(t)

	items := []AiNavItem{
		{Title: "写作助手", Emoji: "✍️", Kind: "panel", Target: "chat", Enabled: true},
		{Title: "官网", Kind: "link", Target: "https://example.com", NewTab: true, Enabled: true},
		{Title: "隐藏项", Kind: "panel", Target: "image", Enabled: false},
	}
	if err := SetAiNavItems(items); err != nil {
		t.Fatalf("保存失败: %v", err)
	}

	all := GetAllAiNavItems()
	if len(all) != 3 {
		t.Fatalf("管理端应看到 3 条（含未启用），实际 %d", len(all))
	}
	pub := GetAiNavItems()
	if len(pub) != 2 {
		t.Fatalf("前台应只看到 2 条已启用，实际 %d", len(pub))
	}
	if pub[0].Title != "写作助手" || pub[1].Title != "官网" {
		t.Errorf("顺序应保持录入顺序，实际 %s / %s", pub[0].Title, pub[1].Title)
	}
	// 空分组自动补「自定义应用」
	if pub[0].Group != "自定义应用" {
		t.Errorf("空分组应补默认分组，实际 %q", pub[0].Group)
	}
	// 空 emoji 补默认图标
	if pub[1].Emoji == "" {
		t.Error("空 emoji 应有兜底图标")
	}
}

// 伪协议必须拒绝（这不是审核，是防站内 XSS 的基本正确性）
func TestAiNavItems_RejectPseudoScheme(t *testing.T) {
	setupAiNavTestDB(t)

	bad := [][]AiNavItem{
		{{Title: "xss", Kind: "link", Target: "javascript:alert(1)", Enabled: true}},
		{{Title: "xss2", Kind: "link", Target: "JavaScript:alert(1)", Enabled: true}},
		{{Title: "data", Kind: "link", Target: "data:text/html,<script>", Enabled: true}},
	}
	for _, b := range bad {
		if err := SetAiNavItems(b); err == nil {
			t.Errorf("应拒绝伪协议：%s", b[0].Target)
		}
	}
	// 正常 https 不受影响
	if err := SetAiNavItems([]AiNavItem{{Title: "ok", Kind: "link", Target: "https://a.com", Enabled: true}}); err != nil {
		t.Errorf("正常链接不应被拒: %v", err)
	}
}

// 外部链接必须以 http(s) 开头；站内面板不能填完整 URL
func TestAiNavItems_KindConsistency(t *testing.T) {
	setupAiNavTestDB(t)

	if err := SetAiNavItems([]AiNavItem{{Title: "x", Kind: "link", Target: "example.com", Enabled: true}}); err == nil {
		t.Error("外部链接缺协议应被拒")
	}
	if err := SetAiNavItems([]AiNavItem{{Title: "y", Kind: "panel", Target: "https://example.com", Enabled: true}}); err == nil {
		t.Error("站内面板填完整 URL 应被拒")
	}
	// 站内绝对路径可以
	if err := SetAiNavItems([]AiNavItem{{Title: "论坛", Kind: "panel", Target: "/forums", Enabled: true}}); err != nil {
		t.Errorf("站内路径应放行: %v", err)
	}
}

// 名称/数量上限
func TestAiNavItems_Limits(t *testing.T) {
	setupAiNavTestDB(t)

	if err := SetAiNavItems([]AiNavItem{{Title: "", Kind: "panel", Target: "chat", Enabled: true}}); err == nil {
		t.Error("空菜单名应被拒")
	}
	long := make([]rune, 20)
	for i := range long {
		long[i] = '长'
	}
	if err := SetAiNavItems([]AiNavItem{{Title: string(long), Kind: "panel", Target: "chat", Enabled: true}}); err == nil {
		t.Error("超长菜单名应被拒")
	}
	// 超过 50 条
	many := make([]AiNavItem, 0, 51)
	for i := 0; i < 51; i++ {
		many = append(many, AiNavItem{Title: fmt.Sprintf("m%d", i), Kind: "panel", Target: "chat", Enabled: true})
	}
	if err := SetAiNavItems(many); err == nil {
		t.Error("超过 50 条应被拒")
	}
}

// 清空后前台应返回空（站长可以整体撤掉自定义菜单）
func TestAiNavItems_Clear(t *testing.T) {
	setupAiNavTestDB(t)

	_ = SetAiNavItems([]AiNavItem{{Title: "a", Kind: "panel", Target: "chat", Enabled: true}})
	if err := SetAiNavItems(nil); err != nil {
		t.Fatalf("清空失败: %v", err)
	}
	if len(GetAiNavItems()) != 0 {
		t.Error("清空后前台应为空")
	}
}
