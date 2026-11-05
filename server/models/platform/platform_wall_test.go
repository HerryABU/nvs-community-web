package platform

import (
	"reflect"
	"testing"
)

// TestDefaultWallConfigZones 板块重组后的三分区口径：
// 二次元区 / 二次元同人区 / 政治文学区，且每个分区都有对应详情。
func TestDefaultWallConfigZones(t *testing.T) {
	cfg := DefaultWallConfig()
	want := []string{"二次元区", "二次元同人区", "政治文学区"}
	if !reflect.DeepEqual(cfg.Zones, want) {
		t.Fatalf("默认分区应为 %v，实际 %v", want, cfg.Zones)
	}
	if len(cfg.ZoneDetails) != len(want) {
		t.Fatalf("分区详情应有 %d 条，实际 %d", len(want), len(cfg.ZoneDetails))
	}
	for i, name := range want {
		if cfg.ZoneDetails[i].Name != name {
			t.Errorf("第 %d 条详情分区名应为 %q，实际 %q", i, name, cfg.ZoneDetails[i].Name)
		}
		if cfg.ZoneDetails[i].Steps <= 0 {
			t.Errorf("%q 的确认步数应大于 0", name)
		}
	}
	// 二次元同人区的确认强度应高于泛二次元区，低于/等于政治文学区
	if cfg.ZoneDetails[1].Steps <= cfg.ZoneDetails[0].Steps {
		t.Errorf("二次元同人区确认强度应高于二次元区")
	}
}

// TestCanonicalZoneName 旧分区名归一，新分区名与未知分区名原样返回。
func TestCanonicalZoneName(t *testing.T) {
	cases := map[string]string{
		"同人区":   "二次元同人区",
		"同人创作区": "二次元同人区",
		"二次元区":  "二次元区",
		"政治文学区": "政治文学区",
		"未知分区":  "未知分区",
	}
	for in, want := range cases {
		if got := CanonicalZoneName(in); got != want {
			t.Errorf("CanonicalZoneName(%q) = %q，期望 %q", in, got, want)
		}
	}
}

// TestMigrateWallConfig 存量库里旧的「同人区/政治文学区」配置读时升级为三分区：
// 自定义步骤数/文案平移到二次元同人区，政治文学区不动，且幂等。
func TestMigrateWallConfig(t *testing.T) {
	legacy := WallConfig{
		Zones: []string{"同人区", "政治文学区"},
		ZoneDetails: []ZoneDetail{
			{Name: "同人区", Steps: 5, ConfirmText: "站长自定义确认语", Warnings: []string{"自定义警告"}, IntroText: "自定义说明", CrossDomainExtra: 3},
			{Name: "政治文学区", Steps: 4, ConfirmText: "我承诺承担全部法律与阅读责任"},
		},
		Enabled:            true,
		CrossDomainWarning: true,
	}

	got := migrateWallConfig(legacy)
	wantZones := []string{"二次元区", "二次元同人区", "政治文学区"}
	if !reflect.DeepEqual(got.Zones, wantZones) {
		t.Fatalf("升级后分区应为 %v，实际 %v", wantZones, got.Zones)
	}
	if !got.Enabled || !got.CrossDomainWarning {
		t.Error("升级不应改动开关位")
	}

	var doujin, politics, acg *ZoneDetail
	for i := range got.ZoneDetails {
		d := &got.ZoneDetails[i]
		switch d.Name {
		case "二次元同人区":
			doujin = d
		case "政治文学区":
			politics = d
		case "二次元区":
			acg = d
		}
	}
	if doujin == nil {
		t.Fatal("缺少二次元同人区详情")
	}
	if doujin.Steps != 5 || doujin.ConfirmText != "站长自定义确认语" || doujin.CrossDomainExtra != 3 {
		t.Errorf("站长自定义配置未平移到二次元同人区：%+v", *doujin)
	}
	if politics == nil || politics.Steps != 4 || politics.ConfirmText != "我承诺承担全部法律与阅读责任" {
		t.Errorf("政治文学区配置被意外改动：%+v", politics)
	}
	if acg == nil {
		t.Error("缺少自动补齐的二次元区默认详情")
	}

	// 幂等：再次升级不应重复插入分区
	again := migrateWallConfig(got)
	if !reflect.DeepEqual(again.Zones, wantZones) {
		t.Fatalf("重复升级不应改变分区列表，实际 %v", again.Zones)
	}
	if len(again.ZoneDetails) != len(got.ZoneDetails) {
		t.Fatalf("重复升级不应新增详情，%d -> %d", len(got.ZoneDetails), len(again.ZoneDetails))
	}
}

// TestMigrateWallConfigKeepsModernConfig 已经是新口径的配置（或站长自己删过分区）不被改写。
func TestMigrateWallConfigKeepsModernConfig(t *testing.T) {
	modern := WallConfig{
		Zones:       []string{"二次元区", "二次元同人区", "政治文学区"},
		ZoneDetails: DefaultWallConfig().ZoneDetails,
	}
	if got := migrateWallConfig(modern); !reflect.DeepEqual(got, modern) {
		t.Errorf("新口径配置被改写：%+v", got)
	}
	custom := WallConfig{Zones: []string{"政治文学区"}, ZoneDetails: []ZoneDetail{{Name: "政治文学区", Steps: 4}}}
	if got := migrateWallConfig(custom); !reflect.DeepEqual(got, custom) {
		t.Errorf("站长自定义（无旧分区）配置被改写：%+v", got)
	}
}

// TestGetZoneDetailLegacyAlias 旧分区名仍能取到对应详情，避免存量前端链接失效。
func TestGetZoneDetailLegacyAlias(t *testing.T) {
	cfg := DefaultWallConfig()
	modern := cfg.GetZoneDetail("二次元同人区")
	if modern.Name != "二次元同人区" || modern.Steps != 3 {
		t.Fatalf("新分区详情异常：%+v", modern)
	}
	legacy := cfg.GetZoneDetail("同人区")
	// 旧名归一到新分区后，应取到同一份详情而不是兜底默认值
	if legacy.Name != modern.Name || legacy.Steps != modern.Steps || legacy.ConfirmText != modern.ConfirmText {
		t.Errorf("旧分区名未归一到二次元同人区详情：%+v", legacy)
	}
}
