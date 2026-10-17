package models

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"nvs-server/config"
	"nvs-server/realname"
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
		"site_name":                 "星海文学",
		"vip_enabled":               "true",
		"email_verify":              "false",
		"captcha_enabled":           "true",
		"novel_visitor_mode":        "open",
		"home_module":               "novels",                                     // 首页主模块：novels / blogs / forums / combined
		"realname_required":         "false",                                      // 发布/评论强制实名（默认关闭）
		"login_methods":             `{"email":true,"phone":true,"wechat":false}`, // 允许的登录方式
		"minor_mode":                "off",                                        // 未成年保护模式：off / soft / strict
		"national_realname_api":     `{"enabled":false,"url":"","token":""}`,      // 国家实名检测 API（不存储姓名/身份证明文）
		"categories":                `["硬科幻","奇幻","推演文学","架空历史","现实主义","悬疑推理","实验文学","同人区","政治区","讽刺文学","泛二次元区","其他"]`,
		"robots_txt":                "User-agent: *\nAllow: /\n",
		"web_publish_quota":         "3", // 每用户默认可公开发布的 web 项目（UserHTML+UserFrame 合计）数量上限
		"web_publish_quota_size_mb": "0", // 每用户默认可公开发布的 web 项目总大小上限（MB，0 = 不限）
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

// HomeModuleKey 首页主模块配置键
const HomeModuleKey = "home_module"

// 允许的首页主模块取值
var validHomeModules = map[string]bool{
	"novels":   true,
	"blogs":    true,
	"forums":   true,
	"combined": true, // 综合页：小说为主，博客/论坛作为副区块
}

// GetHomeModule 返回当前配置的首页主模块（默认 novels）。
// 仅当值合法时返回，避免异常配置导致前端渲染异常。
func GetHomeModule() string {
	v := GetPlatformConfig(HomeModuleKey)
	if validHomeModules[v] {
		return v
	}
	return "novels"
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

// ============ 短信网关配置（管理端可配，DB 优先回退 env） ============

const (
	// SMSGatewayConfigKey 网关形态：log=开发网关（验证码仅入日志）/ http=通用 HTTP 网关
	SMSGatewayConfigKey = "sms_gateway"
	// SMSHTTPURLConfigKey http 网关地址（POST JSON {phone,code,sign_name,template_code}）
	SMSHTTPURLConfigKey = "sms_http_url"
	// SMSHTTPTokenConfigKey http 网关 Bearer Token
	SMSHTTPTokenConfigKey = "sms_http_token"
	// SMSSignNameConfigKey 短信签名
	SMSSignNameConfigKey = "sms_sign_name"
	// SMSTemplateCodeConfigKey 短信模板编码
	SMSTemplateCodeConfigKey = "sms_template_code"
)

// platformConfigExists 判断配置键是否已落库（含空值）。
// 用于区分「管理端从未配置（回退 .env）」与「主动清空（以 DB 空值为准）」。
func platformConfigExists(key string) bool {
	var cnt int64
	DB.Model(&PlatformConfig{}).Where("key = ?", key).Count(&cnt)
	return cnt > 0
}

// GetSMSConfigFromDB 短信网关配置：DB（管理端保存）优先，未落库的键回退 config 包 env 值。
// fromDB 表示管理端是否显式保存过（false=仍依赖 .env 初始值）。
func GetSMSConfigFromDB() (gateway, url, token, signName, template string, fromDB bool) {
	gateway = config.SMSGateway
	url = config.SMSHTTPURL
	token = config.SMSHTTPToken
	signName = config.SMSSignName
	template = config.SMSTemplateCode
	if platformConfigExists(SMSGatewayConfigKey) {
		gateway = GetPlatformConfig(SMSGatewayConfigKey)
		fromDB = true
	}
	if platformConfigExists(SMSHTTPURLConfigKey) {
		url = GetPlatformConfig(SMSHTTPURLConfigKey)
		fromDB = true
	}
	if platformConfigExists(SMSHTTPTokenConfigKey) {
		token = GetPlatformConfig(SMSHTTPTokenConfigKey)
		fromDB = true
	}
	if platformConfigExists(SMSSignNameConfigKey) {
		signName = GetPlatformConfig(SMSSignNameConfigKey)
		fromDB = true
	}
	if platformConfigExists(SMSTemplateCodeConfigKey) {
		template = GetPlatformConfig(SMSTemplateCodeConfigKey)
		fromDB = true
	}
	if gateway != "log" && gateway != "http" {
		gateway = "log"
	}
	return
}

// ============ 微信扫码登录配置（管理端可配，DB 优先回退 env） ============

const (
	// WeChatAppIDConfigKey 微信开放平台 AppID（PC 网站应用）
	WeChatAppIDConfigKey = "wechat_app_id"
	// WeChatAppSecretConfigKey 微信开放平台 AppSecret
	WeChatAppSecretConfigKey = "wechat_app_secret"
	// WeChatCallbackConfigKey 微信授权回调地址
	WeChatCallbackConfigKey = "wechat_callback"
)

// GetWeChatConfigFromDB 微信扫码配置：DB 优先，未落库的键回退 config 包 env 值。
func GetWeChatConfigFromDB() (appID, appSecret, callback string, fromDB bool) {
	appID = config.WeChatAppID
	appSecret = config.WeChatAppSecret
	callback = config.WeChatCallback
	if platformConfigExists(WeChatAppIDConfigKey) {
		appID = GetPlatformConfig(WeChatAppIDConfigKey)
		fromDB = true
	}
	if platformConfigExists(WeChatAppSecretConfigKey) {
		appSecret = GetPlatformConfig(WeChatAppSecretConfigKey)
		fromDB = true
	}
	if platformConfigExists(WeChatCallbackConfigKey) {
		callback = GetPlatformConfig(WeChatCallbackConfigKey)
		fromDB = true
	}
	return
}

// GetWeChatLoginAvailable 微信扫码登录是否可用（AppID/Secret/Callback 三件套齐全）
func GetWeChatLoginAvailable() bool {
	appID, appSecret, callback, _ := GetWeChatConfigFromDB()
	return appID != "" && appSecret != "" && callback != ""
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

// ============ 实名制与登录方式配置（F10 / F21） ============

const (
	// RealNameRequiredKey 发布/评论是否要求实名（true/false，默认 false）
	RealNameRequiredKey = "realname_required"
	// LoginMethodsKey 允许的登录方式（JSON：{"email":true,"phone":true,"wechat":false}）
	LoginMethodsKey = "login_methods"
	// MinorModeKey 未成年保护模式：off=关闭 / soft=软模式（提醒+时长提示）/ strict=严格（未实名限流）
	MinorModeKey = "minor_mode"
	// NationalRealNameKey 国家实名检测 API 配置（JSON：{"enabled":false,"url":"","token":""}）
	// 核验时仅转发姓名+身份证号给国家 API，服务器不存储二者明文。
	NationalRealNameKey = "national_realname_api"
)

// IsRealNameRequired 发布/评论是否强制实名
func IsRealNameRequired() bool {
	return GetPlatformConfig(RealNameRequiredKey) == "true"
}

// LoginMethods 允许的登录方式
type LoginMethods struct {
	Email  bool `json:"email"`  // 邮箱+密码
	Phone  bool `json:"phone"`  // 手机号+验证码
	WeChat bool `json:"wechat"` // 微信扫码
}

// GetLoginMethods 读取登录方式配置（缺省 email+phone 开放，微信默认关闭）
func GetLoginMethods() LoginMethods {
	v := GetPlatformConfig(LoginMethodsKey)
	if v == "" {
		return LoginMethods{Email: true, Phone: true, WeChat: false}
	}
	var m LoginMethods
	if err := json.Unmarshal([]byte(v), &m); err != nil {
		return LoginMethods{Email: true, Phone: true, WeChat: false}
	}
	// 至少保留一种登录方式，避免站长误配置全员锁死
	if !m.Email && !m.Phone && !m.WeChat {
		return LoginMethods{Email: true, Phone: true, WeChat: false}
	}
	return m
}

// SetLoginMethods 持久化登录方式配置
func SetLoginMethods(m LoginMethods) error {
	if !m.Email && !m.Phone && !m.WeChat {
		m.Email = true // 兜底：不允许全部关闭
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return SetPlatformConfig(LoginMethodsKey, string(b))
}

// MinorMode 未成年保护模式
type MinorMode string

const (
	MinorModeOff    MinorMode = "off"
	MinorModeSoft   MinorMode = "soft"
	MinorModeStrict MinorMode = "strict"
)

// GetMinorMode 读取未成年保护模式（缺省 off）
func GetMinorMode() MinorMode {
	switch MinorMode(GetPlatformConfig(MinorModeKey)) {
	case MinorModeSoft, MinorModeStrict:
		return MinorMode(GetPlatformConfig(MinorModeKey))
	}
	return MinorModeOff
}

// NationalRealNameConfig 国家实名检测 API 配置
type NationalRealNameConfig struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url"`   // 国家实名检测 API 端点（HTTPS）
	Token   string `json:"token"` // 接入令牌
}

// GetNationalRealNameConfig 读取国家实名 API 配置（缺省关闭）
func GetNationalRealNameConfig() NationalRealNameConfig {
	v := GetPlatformConfig(NationalRealNameKey)
	var c NationalRealNameConfig
	if v == "" {
		return c
	}
	_ = json.Unmarshal([]byte(v), &c)
	return c
}

// ============ 实名核验提供商（多通道：国家代理/腾讯/阿里/华为/网易） ============

// RealNameProviderConfigKey 实名核验提供商总配置（JSON：provider + 各厂商凭据）
const RealNameProviderConfigKey = "realname_provider"

// GetRealNameProviderConfig 读取提供商配置。
// 兼容迁移：provider 为空但旧键 national_realname_api 已配置时，自动按 national 提供商返回。
func GetRealNameProviderConfig() realname.Config {
	var c realname.Config
	v := GetPlatformConfig(RealNameProviderConfigKey)
	if v != "" {
		_ = json.Unmarshal([]byte(v), &c)
	}
	if c.Provider == "" {
		// 旧配置迁移：national_realname_api 已开启 → national 通道
		old := GetNationalRealNameConfig()
		if old.Enabled && old.URL != "" {
			c.Provider = "national"
			c.National.URL = old.URL
			c.National.Token = old.Token
		}
	}
	return c
}

// SetRealNameProviderConfig 持久化提供商配置
func SetRealNameProviderConfig(c realname.Config) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return SetPlatformConfig(RealNameProviderConfigKey, string(b))
}
