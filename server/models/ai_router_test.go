package models

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupRouterDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:ai_router_test?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	if err := db.AutoMigrate(&AiModel{}, &AiCallLog{}, &PlatformConfig{}); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	old := DB
	DB = db
	t.Cleanup(func() {
		DB = old
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func mkModel(t *testing.T, name string, priceIn, priceOut int64, priority int, provider string) *AiModel {
	t.Helper()
	m := &AiModel{
		Name: name, Kind: AiKindChat, ModelID: name, BaseURL: "http://localhost",
		PriceIn: priceIn, PriceOut: priceOut, Priority: priority,
		ProviderHint: provider, Enabled: true, MaxTokens: 1024,
	}
	if err := DB.Create(m).Error; err != nil {
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
		DB.Create(&AiCallLog{UserID: 1, ModelID: slow.ID, TokensIn: 1, TokensOut: 1, LatencyMs: 3000})
		DB.Create(&AiCallLog{UserID: 1, ModelID: fast.ID, TokensIn: 1, TokensOut: 1, LatencyMs: 200})
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
