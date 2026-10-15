package models

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"nvs-server/config"
)

// PlatformConfig 平台配置（站长面板）
type PlatformConfig struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Key       string    `gorm:"size:64;uniqueIndex;not null" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (PlatformConfig) TableName() string { return "platform_configs" }

// ============ 访客查看小说门禁（站长前台可配） ============
const (
	// NovelVisitorModeKey 访客查看小说模式：
	//   open            —— 完全开放，任何人可阅读/爬取；
	//   trial           —— 访客仅可试读前 30%（最多 5 章），完整需登录；
	//   login_required  —— 必须登录后才能查看任何内容。
	NovelVisitorModeKey = "novel_visitor_mode"
)

// GetNovelVisitorMode 读取访客查看小说模式，缺省 open（完全开放）。
func GetNovelVisitorMode() string {
	switch GetPlatformConfig(NovelVisitorModeKey) {
	case "trial", "login_required", "open":
		return GetPlatformConfig(NovelVisitorModeKey)
	}
	return "open"
}

// ============ 平台配置辅助函数 ============

// GetPlatformConfig 获取单个平台配置值
func GetPlatformConfig(key string) string {
	var cfg PlatformConfig
	if err := DB.Where("key = ?", key).First(&cfg).Error; err != nil {
		return ""
	}
	return cfg.Value
}

// SetPlatformConfig 设置平台配置值（自动 upsert）
func SetPlatformConfig(key, value string) error {
	var cfg PlatformConfig
	err := DB.Where("key = ?", key).First(&cfg).Error
	now := time.Now()
	if err != nil {
		cfg = PlatformConfig{Key: key, Value: value, UpdatedAt: now}
		return DB.Create(&cfg).Error
	}
	cfg.Value = value
	cfg.UpdatedAt = now
	return DB.Save(&cfg).Error
}

// GetAllPlatformConfigs 获取所有平台配置
func GetAllPlatformConfigs() map[string]string {
	var cfgs []PlatformConfig
	DB.Find(&cfgs)
	result := make(map[string]string)
	for _, c := range cfgs {
		result[c.Key] = c.Value
	}
	return result
}

// IsVipEnabled 检查 VIP 付费是否开启
func IsVipEnabled() bool {
	return GetPlatformConfig("vip_enabled") != "false"
}

// GetSiteName 获取站点名称
func GetSiteName() string {
	name := GetPlatformConfig("site_name")
	if name == "" {
		return "星海文学" // 默认站名
	}
	return name
}

// GetCategories 获取分类列表（从平台配置读取，返回字符串数组）
func GetCategories() []string {
	val := GetPlatformConfig("categories")
	if val == "" {
		return []string{"硬科幻", "奇幻", "推演文学", "架空历史", "现实主义", "悬疑推理", "实验文学", "同人区", "政治区", "讽刺文学", "泛二次元区", "其他"}
	}
	var cats []string
	if err := json.Unmarshal([]byte(val), &cats); err != nil {
		return []string{"硬科幻", "奇幻", "推演文学", "架空历史", "现实主义", "悬疑推理", "实验文学", "同人区", "政治区", "讽刺文学", "泛二次元区", "其他"}
	}
	if len(cats) == 0 {
		return []string{"硬科幻", "奇幻", "推演文学", "架空历史", "现实主义", "悬疑推理", "实验文学", "同人区", "政治区", "讽刺文学", "泛二次元区", "其他"}
	}
	return cats
}

// GetRobotsTxt 获取 robots.txt 内容，站长可自定义
func GetRobotsTxt() string {
	txt := GetPlatformConfig("robots_txt")
	if txt == "" {
		return "User-agent: *\nAllow: /\n"
	}
	return txt
}

// InitPlatformConfigs 初始化默认平台配置（仅在不存在时写入）
func InitPlatformConfigs() {
	defaults := map[string]string{
		"site_name":       "星海文学",
		"vip_enabled":     "true",
		"email_verify":    "false",
		"captcha_enabled": "false",
		"novel_visitor_mode": "open",
		"categories":      `["硬科幻","奇幻","推演文学","架空历史","现实主义","悬疑推理","实验文学","同人区","政治区","讽刺文学","泛二次元区","其他"]`,
		"robots_txt":      "User-agent: *\nAllow: /\n",
	}
	for k, v := range defaults {
		var count int64
		DB.Model(&PlatformConfig{}).Where("key = ?", k).Count(&count)
		if count == 0 {
			DB.Create(&PlatformConfig{Key: k, Value: v, UpdatedAt: time.Now()})
		}
	}
}

// IsEmailVerifyEnabled 检查邮箱验证是否开启
func IsEmailVerifyEnabled() bool {
	return GetPlatformConfig("email_verify") == "true"
}

// IsCaptchaEnabled 检查滑块验证码是否开启
func IsCaptchaEnabled() bool {
	return GetPlatformConfig("captcha_enabled") == "true"
}

// ============ 邮箱域名黑白名单 ============

// parseDomainList 解析域名列表配置（JSON 数组，或逗号/换行/分号/空格分隔的纯文本）。
func parseDomainList(val string) []string {
	if val == "" {
		return nil
	}
	var list []string
	if err := json.Unmarshal([]byte(val), &list); err == nil {
		out := make([]string, 0, len(list))
		for _, d := range list {
			d = strings.TrimSpace(strings.ToLower(d))
			if d != "" {
				out = append(out, d)
			}
		}
		return out
	}
	// 兼容纯文本分隔
	fields := strings.FieldsFunc(val, func(r rune) bool {
		return r == ',' || r == '\n' || r == ';' || r == ' ' || r == '\t' || r == '\r'
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(strings.ToLower(f))
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

// GetEmailDomainWhitelist 获取允许注册的邮箱域名白名单（空 = 不限制）。
func GetEmailDomainWhitelist() []string {
	return parseDomainList(GetPlatformConfig("email_domain_whitelist"))
}

// GetEmailDomainBlacklist 获取禁止注册的邮箱域名黑名单（空 = 不限制）。
func GetEmailDomainBlacklist() []string {
	return parseDomainList(GetPlatformConfig("email_domain_blacklist"))
}

// extractEmailDomain 提取邮箱域名（小写、去空格）。格式非法返回空串。
func extractEmailDomain(email string) string {
	email = strings.TrimSpace(strings.ToLower(email))
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return ""
	}
	return email[at+1:]
}

// CheckEmailDomainAllowed 检查邮箱域名是否允许注册。
// 规则：先判黑名单（命中即拒），再判白名单（白名单非空且未命中即拒）。
// 返回 (allowed, reason)，allowed=false 时 reason 为拒绝原因。
func CheckEmailDomainAllowed(email string) (bool, string) {
	domain := extractEmailDomain(email)
	if domain == "" {
		return false, "邮箱格式无效"
	}
	for _, d := range GetEmailDomainBlacklist() {
		if d == domain {
			return false, "该邮箱域名（@" + domain + "）已被平台禁止注册"
		}
	}
	whitelist := GetEmailDomainWhitelist()
	if len(whitelist) > 0 {
		for _, d := range whitelist {
			if d == domain {
				return true, ""
			}
		}
		return false, "该邮箱域名（@" + domain + "）不在允许注册的域名列表内"
	}
	return true, ""
}

// GetSMTPConfigFromDB 从平台配置获取 SMTP 配置
func GetSMTPConfigFromDB() (host, port, user, pass, from string) {
	host = GetPlatformConfig("smtp_host")
	port = GetPlatformConfig("smtp_port")
	user = GetPlatformConfig("smtp_user")
	pass = GetPlatformConfig("smtp_password")
	from = GetPlatformConfig("smtp_from")
	if host == "" {
		host = "smtp.qq.com"
	}
	if port == "" {
		port = "587"
	}
	if from == "" {
		from = user
	}
	return
}

// ============ 日志脱敏配置存储（公安部令第176号合规） ============
// 管理后台「系统设置 → 日志脱敏」修改后持久化到 platform_configs 表，
// 重启后由 config.RedactMgr.LoadFromDB() 恢复；.env 仅在从未保存过时作为初始默认。

const (
	RedactConfigKeyEnabled = "log_redact_enabled"
	RedactConfigKeyMask    = "log_redact_mask"
	RedactConfigKeyFields  = "log_redact_fields"
)

// LoadRedactConfig 从平台配置读取脱敏配置。
// found=false 表示后台从未保存过（调用方应回退 .env 默认）。
func LoadRedactConfig() (rc config.RedactConfig, found bool, err error) {
	enabled := GetPlatformConfig(RedactConfigKeyEnabled)
	mask := GetPlatformConfig(RedactConfigKeyMask)
	fields := GetPlatformConfig(RedactConfigKeyFields)
	if enabled == "" && mask == "" && fields == "" {
		return rc, false, nil
	}
	if enabled != "" {
		rc.Enabled = enabled == "true"
	}
	if mask != "" {
		rc.Mask = mask
	}
	if fields != "" {
		var list []string
		if jerr := json.Unmarshal([]byte(fields), &list); jerr != nil {
			// 兼容逗号分隔纯文本
			list = strings.Split(fields, ",")
		}
		rc.SensitiveKeys = list
	}
	return rc, true, nil
}

// StoreRedactConfig 持久化脱敏配置（管理后台保存时调用，先落库成功再切换内存）
func StoreRedactConfig(rc config.RedactConfig) error {
	if err := SetPlatformConfig(RedactConfigKeyEnabled, strconv.FormatBool(rc.Enabled)); err != nil {
		return err
	}
	if err := SetPlatformConfig(RedactConfigKeyMask, rc.Mask); err != nil {
		return err
	}
	b, err := json.Marshal(rc.SensitiveKeys)
	if err != nil {
		return err
	}
	return SetPlatformConfig(RedactConfigKeyFields, string(b))
}

// ============ 算法推荐配置（公安部令第176号 第七条第九项） ============

const (
	AlgoSortRule     = "algo_sort_rule"    // 首页推荐排序规则：updated_at/created_at/total_chapters
	AlgoRankPeriod   = "algo_rank_period"  // 排行榜计算周期：all/7d/30d/90d
	AlgoPersonalized = "algo_personalized" // 个性化推荐开关（true/false，默认 false）
	AlgoDescription  = "algo_description"  // 算法说明文本（面向用户的白话说明）
)

// DefaultAlgoDescription 默认算法说明（白话文，符合算法透明度要求）
const DefaultAlgoDescription = `本平台的作品展示与排序遵循以下规则：

1. 首页作品列表默认按「最近更新」排序，即最近有更新的作品排在最前面；
2. 排行榜按作品章节数与更新时间综合计算（可在管理后台配置统计周期）；
3. 分类页仅按你选择的分类展示作品，不涉及个性化；
4. 站内搜索按「标题相关性」与「更新时间」排序；
5. 平台默认不进行基于个人阅读历史的个性化推荐；即使开启，你也可以随时在个人设置中关闭。

以上规则公开透明，不涉及算法黑箱。如对推荐结果有疑问，可联系站长查询排序依据。`

// GetAlgoConfig 读取算法推荐配置（缺省回退默认值）
func GetAlgoConfig() (sortRule, rankPeriod string, personalized bool, description string) {
	sortRule = GetPlatformConfig(AlgoSortRule)
	if sortRule == "" {
		sortRule = "updated_at"
	}
	rankPeriod = GetPlatformConfig(AlgoRankPeriod)
	if rankPeriod == "" {
		rankPeriod = "all"
	}
	personalized = GetPlatformConfig(AlgoPersonalized) == "true"
	description = GetPlatformConfig(AlgoDescription)
	if description == "" {
		description = DefaultAlgoDescription
	}
	return
}

// ============ 联网备案配置（公安部令第176号 第七条第一项） ============

const (
	SiteICP     = "site_icp"      // 工信部ICP备案号，如 京ICP备2026000000号
	SitePSB     = "site_psb"      // 公安联网备案号，如 京公网安备11000000000000号
	SitePSBLink = "site_psb_link" // 公安备案跳转链接
)

// GetSiteFiling 读取备案信息（未配置返回空串）
func GetSiteFiling() (icp, psb, psbLink string) {
	return GetPlatformConfig(SiteICP), GetPlatformConfig(SitePSB), GetPlatformConfig(SitePSBLink)
}

// ============ 等保/关基识别记录（公安部令第176号 第七条第四/五项） ============

const (
	SecurityGrade     = "security_grade"      // 等保定级：1/2/3/未定级
	SecurityGradeNote = "security_grade_note" // 等保备注
)

// GetSecurityGrade 读取等保记录（缺省未定级）
func GetSecurityGrade() (grade, note string) {
	grade = GetPlatformConfig(SecurityGrade)
	if grade == "" {
		grade = "未定级"
	}
	return grade, GetPlatformConfig(SecurityGradeNote)
}
