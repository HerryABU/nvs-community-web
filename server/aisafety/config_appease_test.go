package aisafety

import "testing"

// TestGetConfig_SeedsAppease 确认默认配置正确播种绥靖模式字段（含安全默认值）。
func TestGetConfig_SeedsAppease(t *testing.T) {
	setupDB(t)
	cfg, err := GetConfig()
	if err != nil {
		t.Fatalf("读配置失败：%v", err)
	}
	if cfg.AppeaseDisplayMode != "replace" {
		t.Fatalf("默认展示模式应为 replace，实际 %q", cfg.AppeaseDisplayMode)
	}
	if cfg.AppeaseEscalateScore != 80 {
		t.Fatalf("默认升级线应为 80，实际 %d", cfg.AppeaseEscalateScore)
	}
	if cfg.AppeaseReplaceText == "" {
		t.Fatalf("默认替代回答文案不应为空")
	}
	if cfg.AppeaseSoftCardText == "" {
		t.Fatalf("默认软警示卡片文案不应为空")
	}
	if cfg.AppeaseSystemPrompt == "" {
		t.Fatalf("默认宽松系统提示词不应为空")
	}
}

// TestSaveConfig_AppeaseRoundTrip 确认绥靖字段可被保存并原样读回（后台持久化链路）。
func TestSaveConfig_AppeaseRoundTrip(t *testing.T) {
	setupDB(t)
	cfg, _ := GetConfig()
	cfg.AppeaseDisplayMode = "both"
	cfg.AppeaseEscalateScore = 90
	cfg.AppeaseReplaceText = "自定义替代文案"
	cfg.AppeaseSoftCardText = "自定义软卡片"
	cfg.AppeaseSystemPrompt = "自定义宽松准则"
	if err := SaveConfig(cfg); err != nil {
		t.Fatalf("保存失败：%v", err)
	}
	got, _ := GetConfig()
	if got.AppeaseDisplayMode != "both" {
		t.Fatalf("展示模式丢失：%q", got.AppeaseDisplayMode)
	}
	if got.AppeaseEscalateScore != 90 {
		t.Fatalf("升级线丢失：%d", got.AppeaseEscalateScore)
	}
	if got.AppeaseReplaceText != "自定义替代文案" {
		t.Fatalf("替代文案丢失：%q", got.AppeaseReplaceText)
	}
	if got.AppeaseSoftCardText != "自定义软卡片" {
		t.Fatalf("软卡片丢失：%q", got.AppeaseSoftCardText)
	}
	if got.AppeaseSystemPrompt != "自定义宽松准则" {
		t.Fatalf("宽松准则丢失：%q", got.AppeaseSystemPrompt)
	}
}

// TestNormalizeAppeaseDisplayMode 容错：未知值回退 replace（最克制一档）。
func TestNormalizeAppeaseDisplayMode(t *testing.T) {
	cases := map[string]string{
		"replace":  "replace",
		"softcard": "softcard",
		"both":     "both",
		"":         "replace",
		"garbage":  "replace",
	}
	for in, want := range cases {
		if got := NormalizeAppeaseDisplayMode(in); got != want {
			t.Fatalf("NormalizeAppeaseDisplayMode(%q)=%q, want %q", in, got, want)
		}
	}
}

// TestRelaxedSystemPrompt_EmptyFallsBack 空配置回退到内置默认宽松版。
func TestRelaxedSystemPrompt_EmptyFallsBack(t *testing.T) {
	setupDB(t)
	cfg, _ := GetConfig()
	if cfg.AppeaseSystemPrompt != "" {
		// 先把内置默认清空再测回退
		cfg.AppeaseSystemPrompt = ""
		SaveConfig(cfg)
	}
	cfg, _ = GetConfig()
	got := RelaxedSystemPrompt(cfg)
	if got == "" {
		t.Fatalf("宽松提示词不应为空")
	}
}
