package aiops

import (
	"fmt"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"nvs-server/models"
	ai "nvs-server/models/ai"
	"testing"
	"time"
)

// ===== (原 ai_router_test.go) =====

func setupRouterDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:ai_router_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := db.AutoMigrate(&ai.AiModel{}, &ai.AiCallLog{}, &models.PlatformConfig{}); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	old := models.DB
	models.DB = db
	t.Cleanup(func() {
		models.DB = old
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func mkModel(t *testing.T, name string, priceIn, priceOut int64, priority int, provider string) *ai.AiModel {
	t.Helper()
	m := &ai.AiModel{
		Name: name, Kind: ai.AiKindChat, ModelID: name, BaseURL: "http://localhost",
		PriceIn: priceIn, PriceOut: priceOut, Priority: priority,
		ProviderHint: provider, Enabled: true, MaxTokens: 1024,
	}
	if err := models.DB.Create(m).Error; err != nil {
		t.Fatal(err)
	}
	return m
}

// TestPickCheapest 最便宜：按 (输入价+输出价) 升序，免费优先
func TestPickCheapest(t *testing.T) {
	setupRouterDB(t)
	mkModel(t, "expensive", 10, 20, 100, "")
	mkModel(t, "free", 0, 0, 0, "")
	mkModel(t, "mid", 3, 5, 0, "")

	if err := SaveAiRouteConfig(AiRouteConfig{Strategy: RouteCheapest}); err != nil {
		t.Fatal(err)
	}
	m, err := PickChatModel(0, "ai_hub")
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "free" {
		t.Fatalf("应选免费模型，实际 %s", m.Name)
	}
}

// TestPickManualKeepsPriority 手动模式：保持既有语义（priority 大者优先）
func TestPickManualKeepsPriority(t *testing.T) {
	setupRouterDB(t)
	mkModel(t, "low", 1, 1, 1, "")
	mkModel(t, "high", 9, 9, 50, "")

	if err := SaveAiRouteConfig(AiRouteConfig{Strategy: RouteManual}); err != nil {
		t.Fatal(err)
	}
	m, _ := PickChatModel(0, "ai_hub")
	if m.Name != "high" {
		t.Fatalf("manual 应按 priority，实际 %s", m.Name)
	}
}

// TestPickFastestUsesLatency fastest：按近期实测延迟，无样本的排最后
func TestPickFastestUsesLatency(t *testing.T) {
	setupRouterDB(t)
	slow := mkModel(t, "slow", 1, 1, 0, "")
	fast := mkModel(t, "fast", 5, 5, 0, "")
	mkModel(t, "nolatency", 0, 0, 0, "")

	// 造延迟样本：slow=3000ms，fast=200ms
	for i := 0; i < 3; i++ {
		models.DB.Create(&ai.AiCallLog{UserID: 1, ModelID: slow.ID, TokensIn: 1, TokensOut: 1, LatencyMs: 3000})
		models.DB.Create(&ai.AiCallLog{UserID: 1, ModelID: fast.ID, TokensIn: 1, TokensOut: 1, LatencyMs: 200})
	}
	if err := SaveAiRouteConfig(AiRouteConfig{Strategy: RouteFastest}); err != nil {
		t.Fatal(err)
	}
	m, err := PickChatModel(0, "ai_hub")
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "fast" {
		t.Fatalf("fastest 应选延迟最低的，实际 %s", m.Name)
	}
}

// TestPreferDomestic 境内优先只影响排序
func TestPreferDomestic(t *testing.T) {
	setupRouterDB(t)
	mkModel(t, "overseas", 0, 0, 0, "overseas")
	mkModel(t, "domestic", 5, 5, 0, "domestic")

	if err := SaveAiRouteConfig(AiRouteConfig{Strategy: RouteCheapest, PreferDomestic: true}); err != nil {
		t.Fatal(err)
	}
	m, _ := PickChatModel(0, "ai_hub")
	if m.Name != "domestic" {
		t.Fatalf("境内优先未生效，实际 %s", m.Name)
	}
}

// TestFallbackCandidates 降级候选：排除已试，受开关与上限约束
func TestFallbackCandidates(t *testing.T) {
	setupRouterDB(t)
	a := mkModel(t, "a", 0, 0, 0, "")
	mkModel(t, "b", 1, 1, 0, "")
	mkModel(t, "c", 2, 2, 0, "")

	if err := SaveAiRouteConfig(AiRouteConfig{Strategy: RouteCheapest, Fallback: false, MaxFallback: 2}); err != nil {
		t.Fatal(err)
	}
	if got := FallbackChatModels([]uint{a.ID}, "ai_hub", 2); len(got) != 0 {
		t.Fatalf("关闭降级时不应有候选: %+v", got)
	}

	if err := SaveAiRouteConfig(AiRouteConfig{Strategy: RouteCheapest, Fallback: true, MaxFallback: 1}); err != nil {
		t.Fatal(err)
	}
	got := FallbackChatModels([]uint{a.ID}, "ai_hub", 3)
	if len(got) != 1 || got[0].Name != "b" {
		t.Fatalf("上限 1 且排除 a 时应只剩 b: %+v", got)
	}
}

// TestEffectiveStrategy scene 模式下按场景覆盖，未配置场景回落最便宜
func TestEffectiveStrategy(t *testing.T) {
	cfg := NormalizeAiRouteConfig(AiRouteConfig{
		Strategy:      RouteScene,
		SceneStrategy: map[string]string{"oc_turn": RouteCheapest, "ai_hub": RouteFastest, "bad": "nonsense"},
	})
	if EffectiveStrategy(cfg, "oc_turn") != RouteCheapest {
		t.Fatal("oc_turn 应用 cheapest")
	}
	if EffectiveStrategy(cfg, "ai_hub") != RouteFastest {
		t.Fatal("ai_hub 应用 fastest")
	}
	if EffectiveStrategy(cfg, "unknown_scene") != RouteCheapest {
		t.Fatal("未配置场景应回落 cheapest")
	}
	if _, ok := cfg.SceneStrategy["bad"]; ok {
		t.Fatal("非法策略应被丢弃")
	}
	// 非 scene 模式：统一策略
	cfg2 := NormalizeAiRouteConfig(AiRouteConfig{Strategy: RouteFastest})
	if EffectiveStrategy(cfg2, "oc_turn") != RouteFastest {
		t.Fatal("非 scene 模式应统一使用全局策略")
	}
}

// ===== 合并自 ai_nav_test.go =====

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
