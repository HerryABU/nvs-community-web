package models

// API Key 体系：基于「身份」而非「IP」的多层级流量管理体系。
//
// 四层身份 + 匿名通道：
//   - admin    站长 Key（安装/后台生成，极高配额）
//   - author   作者 Key（作者后台生成，高配额）
//   - reader   读者 Key（用户设置页生成，中配额）
//   - federated 联邦 Key（管理员为每个联邦站点单独签发，双向可见）
//   - anonymous 匿名通道（无 Key，走 IP 限流，配额最低）
//   - bot      爬虫通道（无 Key 且被识别为无头/伪装爬虫，配额比匿名更低）
//
// 配额模型：管理员设置各角色全局配额上限；用户创建自己的 Key 时，
// 可在管理员上限之内进一步自设更小配额（动态取 min(自设, 角色上限)）。

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"nvs-server/config"
)

// ==================== 用户级 API Key（admin/author/reader 统一） ====================

// APIKey 用户级 API Key。原始 Key 仅创建/重生成时展示一次，库中只存 sha256 哈希。
type APIKey struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	UserID      uint       `gorm:"not null;index" json:"user_id"`
	Name        string     `gorm:"size:64;default:''" json:"name"`         // 备注名（如「第三方阅读器」）
	Role        string     `gorm:"size:16;not null" json:"role"`           // admin / author / reader
	KeyHash     string     `gorm:"size:64;uniqueIndex;not null" json:"-"`  // sha256(hex)
	KeyPrefix   string     `gorm:"size:24;not null" json:"key_prefix"`     // 如 nvs_rdr_3f2a1b
	QuotaPerMin int        `gorm:"default:60" json:"quota_per_min"`        // 用户自设（受角色全局上限约束）
	AllowedIPs  string     `gorm:"size:512;default:''" json:"allowed_ips"` // 可选 IP/CIDR 白名单（逗号分隔）
	ExpiresAt   *time.Time `json:"expires_at"`                             // 可选过期时间
	LastUsedAt  *time.Time `json:"last_used_at"`
	TotalCalls  int64      `gorm:"default:0" json:"total_calls"` // 累计调用次数（每次带 Key 请求命中后 +1）
	IsActive    bool       `gorm:"default:true" json:"is_active"`
	// 管理员封禁：单个 Key 维度（用户整体封禁见 user_api_policies）
	IsBanned  bool       `gorm:"default:false" json:"is_banned"`
	BanReason string     `gorm:"size:256;default:''" json:"ban_reason"`
	BannedAt  *time.Time `json:"banned_at"`
	BannedBy  uint       `gorm:"default:0" json:"banned_by"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func (APIKey) TableName() string { return "api_keys" }

// ==================== 用户级 API 策略（管理员按人管控） ====================

// UserAPIPolicy 管理员对「某个具体用户」的 API 访问策略（优先级高于角色全局配置）。
//
// 语义：
//   - QuotaPerMin > 0：该用户名下所有 Key 生效的专属配额（取 min(Key 自设, 角色上限, 本值)）
//   - WindowSecs  > 0：配套时间窗口（秒）；0 = 沿用角色窗口
//   - MaxKeys     > 0：该用户最多可持有的 Key 数量；0 = 不限制
//   - KeysBanned      ：封禁该用户全部 API Key（含后续新建的），请求直接 403
//   - BanUntil != nil ：封禁到期时间（到期自动解封；nil = 永久封禁）
//
// 该表只在管理员显式设置某用户时才有记录（缺省即「跟随全局」），
// 避免为每个用户都写一行。
type UserAPIPolicy struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	UserID      uint       `gorm:"not null;uniqueIndex" json:"user_id"`
	QuotaPerMin int        `gorm:"default:0" json:"quota_per_min"`
	WindowSecs  int        `gorm:"default:0" json:"window_secs"`
	MaxKeys     int        `gorm:"default:0" json:"max_keys"`
	KeysBanned  bool       `gorm:"default:false" json:"keys_banned"`
	BanReason   string     `gorm:"size:256;default:''" json:"ban_reason"`
	BanUntil    *time.Time `json:"ban_until"`
	Note        string     `gorm:"size:256;default:''" json:"note"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	// 运行时补充（不落库）
	Username string `gorm:"-" json:"username,omitempty"`
}

func (UserAPIPolicy) TableName() string { return "user_api_policies" }

// IsBannedNow 当前是否处于封禁中（含到期自动解封判定）
func (p *UserAPIPolicy) IsBannedNow() bool {
	if p == nil || !p.KeysBanned {
		return false
	}
	if p.BanUntil != nil && p.BanUntil.Before(time.Now()) {
		return false // 已到期，视为解封
	}
	return true
}

// GetUserAPIPolicy 读取某用户的 API 策略（无记录返回 nil，表示跟随全局）
func GetUserAPIPolicy(userID uint) *UserAPIPolicy {
	if userID == 0 {
		return nil
	}
	var p UserAPIPolicy
	if err := DB.Where("user_id = ?", userID).First(&p).Error; err != nil {
		return nil
	}
	return &p
}

// SaveUserAPIPolicy 写入/更新某用户的 API 策略（零值字段表示「跟随全局」，会一并落库）。
// 用 map 显式 upsert：解封时 KeysBanned/BanUntil 需要真正写回 false/NULL，
// 不依赖 struct 的零值语义，避免将来加字段时踩 GORM 零值坑。
func SaveUserAPIPolicy(p *UserAPIPolicy) error {
	if p == nil || p.UserID == 0 {
		return fmt.Errorf("用户 ID 无效")
	}
	now := time.Now()
	cols := map[string]interface{}{
		"user_id":       p.UserID,
		"quota_per_min": p.QuotaPerMin,
		"window_secs":   p.WindowSecs,
		"max_keys":      p.MaxKeys,
		"keys_banned":   p.KeysBanned,
		"ban_reason":    p.BanReason,
		"ban_until":     p.BanUntil,
		"note":          p.Note,
		"updated_at":    now,
	}
	if existing := GetUserAPIPolicy(p.UserID); existing != nil {
		p.ID = existing.ID
		return DB.Model(&UserAPIPolicy{}).Where("user_id = ?", p.UserID).Updates(cols).Error
	}
	cols["created_at"] = now
	return DB.Model(&UserAPIPolicy{}).Create(cols).Error
}

// ListUserAPIPolicies 列出所有被管理员单独设置过的用户 API 策略（附用户名）
func ListUserAPIPolicies() ([]UserAPIPolicy, error) {
	var list []UserAPIPolicy
	if err := DB.Order("updated_at DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return list, nil
	}
	ids := make([]uint, 0, len(list))
	for _, p := range list {
		ids = append(ids, p.UserID)
	}
	var users []User
	DB.Where("id IN ?", ids).Find(&users)
	names := map[uint]string{}
	for _, u := range users {
		names[u.ID] = u.Username
	}
	for i := range list {
		list[i].Username = names[list[i].UserID]
	}
	return list, nil
}

// ==================== 联邦站点 API Key（双向可见） ====================

// FederatedPeerKey 联邦站点 API Key。
// Direction=inbound ：本站签发给对方站的 Key（对方用它访问本站 → 本站识别「谁在连、用的是哪个 Key」）
// Direction=outbound：对方站签发给本站的 Key（本站访问对方站时携带 → 对方识别本站身份）
// 双向可见性：本站能看到对方用的是本站签发的哪个 Key；对方同样能看到本站用的是它签发的哪个 Key。
type FederatedPeerKey struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	SiteID      uint       `gorm:"not null;index" json:"site_id"`          // 关联 federated_sites.id
	Direction   string     `gorm:"size:8;not null;index" json:"direction"` // inbound / outbound
	PeerName    string     `gorm:"size:128;default:''" json:"peer_name"`   // 对方站点名（本站视角）
	KeyHash     string     `gorm:"size:64;uniqueIndex;not null" json:"-"`  // sha256(hex)
	KeyPrefix   string     `gorm:"size:24;not null" json:"key_prefix"`     // nvs_fed_xxxx
	KeyCipher   string     `gorm:"size:1024;default:''" json:"-"`          // outbound 明文 Key 经 AES-256-GCM 加密后 hex 存储。长度推导：hex = 2 × (nonce 12 + 明文 N + tag 16) = 2N + 56；N=256 → 568 字符。取 1024 可容纳最长 484 字符的对端 Key，MySQL/SQLite 均安全
	IssuerName  string     `gorm:"size:128;default:''" json:"issuer_name"` // 签发方站名（被链接的站据此识别「是谁的 Key」）
	IssuerURL   string     `gorm:"size:512;default:''" json:"issuer_url"`
	QuotaPerMin int        `gorm:"default:5000" json:"quota_per_min"`
	AllowedIPs  string     `gorm:"size:512;default:''" json:"allowed_ips"` // 可选来源 IP 白名单
	ExpiresAt   *time.Time `json:"expires_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	TotalCalls  int64      `gorm:"default:0" json:"total_calls"` // 累计调用次数（联邦 Key 命中后 +1）
	IsActive    bool       `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (FederatedPeerKey) TableName() string { return "federated_peer_keys" }

// ==================== 访问日志（按身份聚合） ====================

// ApiAccessLog API 访问日志。为避免巨量写库，仅记录：
//  1. 携带 API Key 的请求（含联邦）
//  2. 触发限流（429）的请求（同 key/IP 每分钟去重）
//  3. 被识别为爬虫的请求（同 IP 每分钟去重）
type ApiAccessLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	IdentityType string    `gorm:"size:16;index" json:"identity_type"` // federated/admin/author/reader/anonymous/bot
	IdentityID   uint      `json:"identity_id"`                        // user_id 或 peer_key_id
	KeyPrefix    string    `gorm:"size:24;default:''" json:"key_prefix"`
	Method       string    `gorm:"size:8" json:"method"`
	Path         string    `gorm:"size:512" json:"path"` // 512：URL 查询串较长时避免 MySQL 严格模式 Data too long
	StatusCode   int       `json:"status_code"`
	RequestIP    string    `gorm:"size:64" json:"request_ip"`
	UserAgent    string    `gorm:"size:512" json:"user_agent"`
	BotReason    string    `gorm:"size:128;default:''" json:"bot_reason"` // 爬虫命中原因
	CreatedAt    time.Time `gorm:"index" json:"created_at"`
}

func (ApiAccessLog) TableName() string { return "api_access_logs" }

// ==================== Key 生成与哈希 ====================

// roleKeyPrefix 角色 → Key 前缀（一眼识别流量来源）
func roleKeyPrefix(role string) string {
	switch role {
	case "admin":
		return "nvs_adm_"
	case "author":
		return "nvs_aut_"
	case "federated":
		return "nvs_fed_"
	default:
		return "nvs_rdr_"
	}
}

// randomHex 生成 n 字节随机数的 hex 表示
func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// GenerateAPIKey 生成用户级 Key：前缀(角色) + 32 位随机 hex。
// 返回 (完整 Key, KeyPrefix, KeyHash)。
func GenerateAPIKey(role string) (string, string, string, error) {
	randPart, err := randomHex(16)
	if err != nil {
		return "", "", "", err
	}
	full := roleKeyPrefix(role) + randPart
	prefix := full[:len(roleKeyPrefix(role))+8] // 前缀 + 前 8 位随机
	hash := HashAPIKey(full)
	return full, prefix, hash, nil
}

// GenerateFederatedKey 生成联邦 Key（前缀 nvs_fed_）
func GenerateFederatedKey() (string, string, string, error) {
	randPart, err := randomHex(16)
	if err != nil {
		return "", "", "", err
	}
	full := roleKeyPrefix("federated") + randPart
	prefix := full[:len(roleKeyPrefix("federated"))+8]
	hash := HashAPIKey(full)
	return full, prefix, hash, nil
}

// HashAPIKey 计算 Key 的 sha256 哈希（hex 编码）
func HashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// ==================== 配额配置（platform_configs 持久化） ====================

// RoleQuota 单角色配额：窗口内最大请求数
type RoleQuota struct {
	Req    int `json:"req"`
	Window int `json:"window"` // 秒
}

// APIRateLimitConfig 全局限流配置（JSON 存 platform_configs.api_key_quota）
// 配额三档关系（无 Key 整体低配额，匿名爬虫更低，有 Key 才高配额）：
//
//	有 Key：admin/author/reader/federated 各自高配额（Key 维度）
//	无 Key：anonymous（普通访客，低配额）→ bot（识别为爬虫，更低）→ blocked（已知名头爬虫，直接 403 阻断）
type APIRateLimitConfig struct {
	Admin     RoleQuota `json:"admin"`
	Author    RoleQuota `json:"author"`
	Reader    RoleQuota `json:"reader"`
	Federated RoleQuota `json:"federated"`
	Anonymous RoleQuota `json:"anonymous"` // 无 Key 匿名（普通访客，低配额）
	Bot       RoleQuota `json:"bot"`       // 识别为爬虫（无头/伪装/超阈值），配额比匿名更低

	BotUAKeywords    []string `json:"bot_ua_keywords"`    // 自定义爬虫 UA 关键词（与内置限流名单并集 → 低配额）
	BotBlockKeywords []string `json:"bot_block_keywords"` // 已知名头爬虫阻断名单（与内置阻断名单并集 → 直接 403）
	BotSpoofCheck    bool     `json:"bot_spoof_check"`    // 伪装头检测：浏览器 UA 但缺 Sec-Fetch-* 头 → 判 bot
	BotIPThreshold   int      `json:"bot_ip_threshold"`   // 单 IP 窗口内请求数超过该值自动降级 bot（0=关闭）
	BotIPTTLMin      int      `json:"bot_ip_ttl_min"`     // IP 行为统计窗口（分钟，默认 1）
	LogRetentionDays int      `json:"log_retention_days"` // 访问日志保留天数（默认 7）
}

// DefaultAPIRateLimitConfig 默认配额。
// 核心语义：有 Key 高配额；无 Key 低配额（匿名爬虫更低）；已知名头爬虫直接阻断。
func DefaultAPIRateLimitConfig() APIRateLimitConfig {
	return APIRateLimitConfig{
		Admin:            RoleQuota{Req: 10000, Window: 60},
		Author:           RoleQuota{Req: 1000, Window: 60},
		Reader:           RoleQuota{Req: 200, Window: 60},
		Federated:        RoleQuota{Req: 5000, Window: 60},
		Anonymous:        RoleQuota{Req: 60, Window: 60}, // 无 Key 普通访客（低于有 Key）
		Bot:              RoleQuota{Req: 5, Window: 60},  // 匿名爬虫更低
		BotSpoofCheck:    true,
		BotIPThreshold:   300,
		BotIPTTLMin:      1,
		LogRetentionDays: 7,
	}
}

const apiRateLimitConfigKey = "api_key_quota"

// LoadAPIRateLimitConfig 读取全局限流配置（缺省回退默认值）
func LoadAPIRateLimitConfig() APIRateLimitConfig {
	cfg := DefaultAPIRateLimitConfig()
	val := GetPlatformConfig(apiRateLimitConfigKey)
	if val == "" {
		return cfg
	}
	if err := jsonUnmarshal([]byte(val), &cfg); err != nil {
		return DefaultAPIRateLimitConfig()
	}
	// 兜底：字段为 0 时回退默认（防配置损坏导致限流失控/全禁）
	def := DefaultAPIRateLimitConfig()
	cfg = sanitizeQuotaConfig(cfg, def)
	return cfg
}

// SaveAPIRateLimitConfig 保存全局限流配置
func SaveAPIRateLimitConfig(cfg APIRateLimitConfig) error {
	cfg = sanitizeQuotaConfig(cfg, DefaultAPIRateLimitConfig())
	b, err := jsonMarshal(cfg)
	if err != nil {
		return err
	}
	return SetPlatformConfig(apiRateLimitConfigKey, string(b))
}

// sanitizeQuotaConfig 补全零值字段并夹取合法范围
func sanitizeQuotaConfig(cfg, def APIRateLimitConfig) APIRateLimitConfig {
	roles := []struct {
		dst *RoleQuota
		src RoleQuota
	}{
		{&cfg.Admin, def.Admin}, {&cfg.Author, def.Author}, {&cfg.Reader, def.Reader},
		{&cfg.Federated, def.Federated}, {&cfg.Anonymous, def.Anonymous}, {&cfg.Bot, def.Bot},
	}
	for _, r := range roles {
		if r.dst.Req <= 0 {
			r.dst.Req = r.src.Req
		}
		if r.dst.Window <= 0 {
			r.dst.Window = r.src.Window
		}
		if r.dst.Req > 1000000 {
			r.dst.Req = 1000000
		}
		if r.dst.Window > 86400 {
			r.dst.Window = 86400
		}
	}
	if cfg.BotIPThreshold < 0 {
		cfg.BotIPThreshold = 0
	}
	if cfg.BotIPTTLMin <= 0 {
		cfg.BotIPTTLMin = 1
	}
	if cfg.LogRetentionDays <= 0 {
		cfg.LogRetentionDays = 7
	}
	if cfg.LogRetentionDays > 90 {
		cfg.LogRetentionDays = 90
	}
	return cfg
}

// RoleQuotaFor 返回某身份类型的配额（角色上限，供用户自设 Key 时做天花板）
func (c APIRateLimitConfig) RoleQuotaFor(role string) RoleQuota {
	switch role {
	case "admin":
		return c.Admin
	case "author":
		return c.Author
	case "federated":
		return c.Federated
	default:
		return c.Reader
	}
}

// ==================== Key 校验（中间件调用） ====================

// KeyIdentity 一次请求的身份识别结果
type KeyIdentity struct {
	Type        string // federated / admin / author / reader / anonymous / bot
	ID          uint   // 记录主键（api_keys.id 或 federated_peer_keys.id）
	UserID      uint   // 用户 Key 归属用户
	KeyPrefix   string
	KeyHash     string
	AllowedIPs  string // Key 的 IP 白名单（空 = 不限制）
	QuotaPerMin int
	WindowSecs  int
	PeerName    string // 联邦：对方站点名（谁在用这个 Key）
	IssuerName  string // 联邦：签发方站名
	IssuerURL   string // 联邦：签发方站点地址
	BotReason   string // bot 命中原因
	ValidKey    bool   // 是否携带且通过校验的有效 Key
	Banned      bool   // 被管理员封禁（Key 级 或 用户级），调用方应直接 403
	BanReason   string // 封禁原因（返回给调用方，便于申诉）
}

// ResolveAPIKey 校验一个原始 Key，返回身份信息（未命中返回 nil）。
// 优先级：联邦 Key → 用户 Key（因为联邦 Key 权限最高，先命中避免误归类）。
func ResolveAPIKey(rawKey string) *KeyIdentity {
	if rawKey == "" {
		return nil
	}
	hash := HashAPIKey(rawKey)

	// 1) 联邦 Key
	var fed FederatedPeerKey
	if err := DB.Where("key_hash = ? AND is_active = 1", hash).First(&fed).Error; err == nil {
		if fed.ExpiresAt != nil && fed.ExpiresAt.Before(time.Now()) {
			return nil
		}
		cfg := LoadAPIRateLimitConfig()
		quota := fed.QuotaPerMin
		if quota <= 0 || quota > cfg.Federated.Req {
			quota = cfg.Federated.Req
		}
		return &KeyIdentity{
			Type: "federated", ID: fed.ID, KeyPrefix: fed.KeyPrefix, KeyHash: hash,
			AllowedIPs: fed.AllowedIPs, QuotaPerMin: quota, WindowSecs: cfg.Federated.Window,
			PeerName: fed.PeerName, IssuerName: fed.IssuerName, IssuerURL: fed.IssuerURL, ValidKey: true,
		}
	}

	// 2) 用户 Key（admin/author/reader）
	var ak APIKey
	if err := DB.Where("key_hash = ? AND is_active = 1", hash).First(&ak).Error; err == nil {
		if ak.ExpiresAt != nil && ak.ExpiresAt.Before(time.Now()) {
			return nil
		}
		cfg := LoadAPIRateLimitConfig()
		roleQuota := cfg.RoleQuotaFor(ak.Role)
		quota := ak.QuotaPerMin
		if quota <= 0 || quota > roleQuota.Req {
			quota = roleQuota.Req // 管理员调低全局上限后，旧 Key 自动收缩
		}
		window := roleQuota.Window

		// 用户级策略（管理员按人限流 / 封禁）：优先级高于角色全局
		banned, banReason := false, ""
		if policy := GetUserAPIPolicy(ak.UserID); policy != nil {
			if policy.IsBannedNow() {
				banned, banReason = true, policy.BanReason
				if banReason == "" {
					banReason = "管理员已封禁该用户名下的全部 API Key"
				}
			}
			if policy.QuotaPerMin > 0 && (quota <= 0 || policy.QuotaPerMin < quota) {
				quota = policy.QuotaPerMin // 按人收紧：取更严格的那个
			}
			if policy.WindowSecs > 0 {
				window = policy.WindowSecs
			}
		}
		// Key 级封禁（管理员单独封禁某一把 Key）
		if ak.IsBanned {
			banned = true
			if ak.BanReason != "" {
				banReason = ak.BanReason
			} else if banReason == "" {
				banReason = "该 API Key 已被管理员封禁"
			}
		}

		return &KeyIdentity{
			Type: ak.Role, ID: ak.ID, UserID: ak.UserID, KeyPrefix: ak.KeyPrefix, KeyHash: hash,
			AllowedIPs: ak.AllowedIPs, QuotaPerMin: quota, WindowSecs: window, ValidKey: true,
			Banned: banned, BanReason: banReason,
		}
	}

	return nil
}

// MatchAllowedIP 校验 IP 是否命中白名单（逗号分隔的 IP 或 CIDR；空 = 不限制）
func MatchAllowedIP(allowed string, ip string) bool {
	allowed = strings.TrimSpace(allowed)
	if allowed == "" {
		return true
	}
	for _, part := range strings.Split(allowed, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if ipMatchesCIDR(part, ip) {
			return true
		}
	}
	return false
}

// ipMatchesCIDR 匹配单个 IP 或 CIDR 网段
func ipMatchesCIDR(pattern, ip string) bool {
	ipAddr := net.ParseIP(strings.TrimSpace(ip))
	if ipAddr == nil {
		return false
	}
	pattern = strings.TrimSpace(pattern)
	if strings.Contains(pattern, "/") {
		_, ipNet, err := net.ParseCIDR(pattern)
		if err != nil {
			return false
		}
		return ipNet.Contains(ipAddr)
	}
	return ipAddr.Equal(net.ParseIP(pattern))
}

// ==================== 联邦 Key 数据访问 ====================

// ActiveOutboundKeys 返回某站点启用的出站联邦 Key（本站访问对方站时携带）
func ActiveOutboundKeys(siteID uint) []FederatedPeerKey {
	var list []FederatedPeerKey
	DB.Where("site_id = ? AND direction = ? AND is_active = 1", siteID, "outbound").Find(&list)
	return list
}

// DecryptOutboundKey 解密出站 Key 明文（供同步请求携带）
func (k *FederatedPeerKey) DecryptOutboundKey() (string, error) {
	if k.KeyCipher == "" {
		return "", nil
	}
	return config.DecryptSecret(k.KeyCipher)
}

// ==================== 访问日志 ====================

// RecordAccessLog 写入一条访问日志（调用方已做节流控制）
func RecordAccessLog(log *ApiAccessLog) error {
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}
	return DB.Create(log).Error
}

// ListAccessLogs 分页查询访问日志
func ListAccessLogs(page, pageSize int, identityType string) ([]ApiAccessLog, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	q := DB.Model(&ApiAccessLog{})
	if identityType != "" {
		q = q.Where("identity_type = ?", identityType)
	}
	var total int64
	q.Count(&total)
	var list []ApiAccessLog
	err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}

// CleanupAccessLogs 清理超过保留天数的访问日志（启动时调用一次）
func CleanupAccessLogs(days int) {
	if days <= 0 {
		days = 7
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	DB.Where("created_at < ?", cutoff).Delete(&ApiAccessLog{})
}

// ==================== 轻量 JSON 工具 ====================

func jsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func jsonUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
