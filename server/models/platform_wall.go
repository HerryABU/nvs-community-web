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

// DefaultWallConfig 返回默认隔离墙配置
func DefaultWallConfig() WallConfig {
	return WallConfig{
		Zones: []string{"同人区", "政治文学区"},
		ZoneDetails: []ZoneDetail{
			{
				Name:             "同人区",
				Steps:            3,
				ConfirmText:      "我承诺承担全部阅读责任",
				Warnings:         []string{"同人区内容可能涉及成人题材或争议性内容。如果您对这类内容敏感，建议立即离开。"},
				IntroText:        "您即将进入「同人区」分区。该分区内容属于同人创作，可能包含成人向或争议性内容。",
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

// GetZoneDetail 获取指定 zone 的详细配置，没有则返回默认值
func (wc WallConfig) GetZoneDetail(zoneName string) ZoneDetail {
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
	return cfg
}

// SetWallConfig 保存隔离墙配置
func SetWallConfig(cfg WallConfig) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return SetPlatformConfig("wall_config", string(data))
}
