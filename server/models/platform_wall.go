package models

import "encoding/json"

// ZoneDetail 单个敏感分区的详细配置
type ZoneDetail struct {
	Name             string   `json:"name"`
	Steps            int      `json:"steps"`
	ConfirmText      string   `json:"confirm_text"`
	Warnings         []string `json:"warnings"`
	IntroText        string   `json:"intro_text"`
	CrossDomainExtra int      `json:"cross_domain_extra"`
}

// WallConfig 隔离墙配置
type WallConfig struct {
	Zones              []string     `json:"zones"`
	ZoneDetails        []ZoneDetail `json:"zone_details"`
	Enabled            bool         `json:"enabled"`
	CrossDomainWarning bool         `json:"cross_domain_warning"`
}

// LegacyZoneAliases 板块重组前的旧分区名 → 重组后的分区名。
// 存量库里的 wall_config、论坛 zone 绑定、历史访问日志仍可能带旧名，
// 读配置时统一归一，避免旧链接与旧配置失效；历史审计数据不改写。
var LegacyZoneAliases = map[string]string{
	"同人区":   "二次元同人区",
	"同人创作区": "二次元同人区",
}

// CanonicalZoneName 把旧分区名归一到板块重组后的分区名；新名字原样返回。
func CanonicalZoneName(zone string) string {
	if v, ok := LegacyZoneAliases[zone]; ok {
		return v
	}
	return zone
}

// DefaultWallConfig 返回默认隔离墙配置。
//
// 板块重组口径：原「同人区」按内容属性拆为两个分区——
//   - 二次元区：ACGN 泛二次元内容，确认强度较轻；
//   - 二次元同人区：同人创作，可能含成人向/争议题材，确认强度更重；
//   - 政治文学区：保持不变。
func DefaultWallConfig() WallConfig {
	return WallConfig{
		Zones: []string{"二次元区", "二次元同人区", "政治文学区"},
		ZoneDetails: []ZoneDetail{
			{
				Name:             "二次元区",
				Steps:            2,
				ConfirmText:      "我已知悉分区内容并承担阅读责任",
				Warnings:         []string{"本分区为 ACGN 泛二次元内容，可能含剧透、粉丝向讨论或小众题材。"},
				IntroText:        "您即将进入「二次元区」。该分区聚焦动画、漫画、游戏及其衍生文化。",
				CrossDomainExtra: 1,
			},
			{
				Name:             "二次元同人区",
				Steps:            3,
				ConfirmText:      "我承诺承担全部阅读责任",
				Warnings:         []string{"二次元同人创作可能涉及成人题材或争议性内容。如果您对这类内容敏感，建议立即离开。"},
				IntroText:        "您即将进入「二次元同人区」。该分区内容属于二次元同人创作，可能包含成人向或争议性内容。",
				CrossDomainExtra: 2,
			},
			{
				Name:             "政治文学区",
				Steps:            4,
				ConfirmText:      "我承诺承担全部法律与阅读责任",
				Warnings:         []string{"该分区内容可能涉及政治隐喻或社会议题。请确保您理解并尊重多元观点。"},
				IntroText:        "您即将进入「政治文学区」分区。该分区内容涉及政治隐喻或社会议题，可能引发强烈情感反应。",
				CrossDomainExtra: 2,
			},
		},
		Enabled:            true,
		CrossDomainWarning: true,
	}
}

// migrateWallConfig 读配置时把含旧「同人区」的存量配置升级为三分区。
// 站长对旧分区自定义的步骤数/文案会平移到「二次元同人区」，不改动政治文学区；
// 只在读取时归一，不自动写库，避免读接口产生写副作用。
func migrateWallConfig(cfg WallConfig) WallConfig {
	hasLegacy, hasNew := false, false
	for _, z := range cfg.Zones {
		if z == "同人区" || z == "同人创作区" {
			hasLegacy = true
		}
		if z == "二次元同人区" {
			hasNew = true
		}
	}
	if !hasLegacy || hasNew {
		return cfg
	}

	zones := make([]string, 0, len(cfg.Zones)+1)
	for _, z := range cfg.Zones {
		if z == "同人区" || z == "同人创作区" {
			zones = append(zones, "二次元区", "二次元同人区")
		} else {
			zones = append(zones, z)
		}
	}
	cfg.Zones = zones

	details := make([]ZoneDetail, 0, len(cfg.ZoneDetails)+1)
	for _, d := range cfg.ZoneDetails {
		if d.Name == "同人区" || d.Name == "同人创作区" {
			d.Name = "二次元同人区"
		}
		details = append(details, d)
	}
	found := false
	for _, d := range details {
		if d.Name == "二次元区" {
			found = true
			break
		}
	}
	if !found {
		details = append(details, DefaultWallConfig().ZoneDetails[0])
	}
	cfg.ZoneDetails = details
	return cfg
}

// GetZoneDetail 获取指定 zone 的详细配置，没有则返回默认值
func (wc WallConfig) GetZoneDetail(zoneName string) ZoneDetail {
	zoneName = CanonicalZoneName(zoneName)
	for _, d := range wc.ZoneDetails {
		if d.Name == zoneName {
			if d.Steps <= 0 {
				d.Steps = 3
			}
			if d.ConfirmText == "" {
				d.ConfirmText = "我承诺承担全部阅读责任"
			}
			if len(d.Warnings) == 0 {
				d.Warnings = []string{"该分区内容属于敏感题材，请谨慎阅读。"}
			}
			if d.IntroText == "" {
				d.IntroText = "您即将进入「" + zoneName + "」分区。该分区内容属于敏感题材。"
			}
			if d.CrossDomainExtra < 0 {
				d.CrossDomainExtra = 2
			}
			return d
		}
	}
	return ZoneDetail{
		Name:             zoneName,
		Steps:            3,
		ConfirmText:      "我承诺承担全部阅读责任",
		Warnings:         []string{"该分区内容属于敏感题材，请谨慎阅读。"},
		IntroText:        "您即将进入「" + zoneName + "」分区。",
		CrossDomainExtra: 2,
	}
}

// GetWallConfig 获取隔离墙配置
func GetWallConfig() WallConfig {
	val := GetPlatformConfig("wall_config")
	if val == "" {
		return DefaultWallConfig()
	}
	var cfg WallConfig
	if err := json.Unmarshal([]byte(val), &cfg); err != nil {
		return DefaultWallConfig()
	}
	return migrateWallConfig(cfg)
}

// SetWallConfig 保存隔离墙配置
func SetWallConfig(cfg WallConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return SetPlatformConfig("wall_config", string(data))
}
