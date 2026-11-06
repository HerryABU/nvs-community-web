// Package readperm 小说 / 博客统一的「作品级阅读权限」模型。
//
// 设计要点：等级全集由代码定义，但**哪些等级可用、VIP 门槛、付费最低价全部由站长
// 在后台配置**（platform_configs.read_permission_config），代码里不写死策略。
// 判定器只读配置，不依赖 gin / HTTP，便于在列表、详情、正文、导出等多处复用。
package readperm

// 内置等级全集（站长可逐一启用 / 停用，public 恒可用、不可停用）。
const (
	LevelPublic   = "public"   // 任何人可见
	LevelLogin    = "login"    // 登录用户可见
	LevelFollower = "follower" // 作者的粉丝可见
	LevelVip      = "vip"      // 达到站长设定的会员等级可见
	LevelPaid     = "paid"     // 已购买本篇 / 本部内容的用户可见
	LevelPrivate  = "private"  // 仅作者本人与平台管理员 / 警方可见
)

// AllLevels 全部内置等级（顺序即后台展示顺序）。
var AllLevels = []string{
	LevelPublic, LevelLogin, LevelFollower, LevelVip, LevelPaid, LevelPrivate,
}

// DefaultLabels 默认展示文案（站长可在后台覆盖）。
var DefaultLabels = map[string]string{
	LevelPublic:   "公开",
	LevelLogin:    "登录可见",
	LevelFollower: "粉丝可见",
	LevelVip:      "VIP 可见",
	LevelPaid:     "付费解锁",
	LevelPrivate:  "仅自己可见",
}

// IsKnownLevel 是否为内置等级。
func IsKnownLevel(s string) bool {
	for _, l := range AllLevels {
		if l == s {
			return true
		}
	}
	return false
}

// Label 取展示文案（优先站长配置，缺省回退内置）。
func Label(cfg Config, level string) string {
	if s := cfg.Labels[level]; s != "" {
		return s
	}
	return DefaultLabels[level]
}

// 拒绝原因码（前端可据此决定「去登录 / 去关注 / 去开通会员 / 去购买」）。
const (
	ReasonNone          = ""
	ReasonLoginRequired = "login_required"
	ReasonFollowerOnly  = "follower_only"
	ReasonVipRequired   = "vip_required"
	ReasonPaidRequired  = "paid_required"
	ReasonPrivate       = "private"
	ReasonLevelDisabled = "level_disabled"
)

// 内容类型（购买记录与判定共用）。
const (
	TypeNovel = "novel"
	TypeBlog  = "blog"
)
