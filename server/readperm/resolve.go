package readperm

import (
	"time"

	"nvs-server/models"
	"nvs-server/models/social"
)

// Actor 访问者快照（由 handler 从登录态构造，判定层不依赖 gin）。
type Actor struct {
	UserID   uint
	Role     string // reader / admin / police ...
	LoggedIn bool
}

// NewActor 从当前登录态构造访问者。
func NewActor(userID uint, role string) Actor {
	return Actor{UserID: userID, Role: role, LoggedIn: userID != 0}
}

// Content 内容快照（小说 / 博客统一形状）。
type Content struct {
	Type        string // TypeNovel / TypeBlog
	ID          uint
	AuthorID    uint
	Permission  string
	PaidPrice   int64
	WallEnabled bool
}

// NovelContent 从小说模型构造内容快照。
func NovelContent(n *models.Novel) Content {
	if n == nil {
		return Content{Type: TypeNovel}
	}
	return Content{
		Type: TypeNovel, ID: n.ID, AuthorID: n.AuthorID,
		Permission: n.ReadPermission, PaidPrice: n.PaidPrice, WallEnabled: n.WallEnabled,
	}
}

// BlogContent 从博客字段构造内容快照（handler 侧调用，避免 readperm 反向依赖 blog 子包）。
func BlogContent(id, authorID uint, permission string, paidPrice int64) Content {
	return Content{
		Type: TypeBlog, ID: id, AuthorID: authorID,
		Permission: permission, PaidPrice: paidPrice,
	}
}

// Decision 判定结果。Reason 为机器码，前端据此决定引导动作；Message 可直接展示。
type Decision struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
	Level   string `json:"level"` // 实际生效的等级（含老数据兼容映射）
}

func allow(level string) Decision { return Decision{Allowed: true, Level: level} }

func deny(level, reason, msg string) Decision {
	return Decision{Allowed: false, Reason: reason, Message: msg, Level: level}
}

// VipLevelOf 返回用户当前会员等级（0 = 非会员）。
//
// 当前会员模型只有 MemberUntil（有效期），有效即视为 1 级；#66 VIP 卡密 /
// 多等级会员落地后可覆盖此钩子，判定层与配置里的 VipMinLevel 均无需改动。
var VipLevelOf = func(userID uint) int {
	if userID == 0 || models.DB == nil {
		return 0
	}
	var u models.User
	if err := models.DB.Select("id", "member_until").Where("id = ?", userID).First(&u).Error; err != nil {
		return 0
	}
	if u.MemberUntil != nil && u.MemberUntil.After(time.Now()) {
		return 1
	}
	return 0
}

// Check 统一的作品级阅读权限判定。
//
// 顺序：public 直通 → 平台角色合规放行 → 作者本人放行 → 未登录拦截 →
// 按等级校验（关注 / VIP / 已购 / 仅自己）。
// 若作者选用了后来被站长停用的等级，保守降级为「登录可见」，
// 既不把内容意外全公开，也不至于彻底锁死。
func Check(a Actor, ct Content) Decision {
	cfg := Load()
	level := EffectivePermission(ct.Permission, ct.WallEnabled)
	if !cfg.IsEnabled(level) {
		level = LevelLogin
	}
	if level == LevelPublic {
		return allow(level)
	}
	// 平台角色（admin / police）合规审查放行；与协作权限矩阵不同，
	// 这里只涉及「读取」，不赋予任何创作能力。
	if social.IsElevated(a.Role) || a.Role == "police" {
		return allow(level)
	}
	if a.UserID != 0 && a.UserID == ct.AuthorID {
		return allow(level)
	}
	if !a.LoggedIn || a.UserID == 0 {
		return deny(level, ReasonLoginRequired, "该内容需登录后查看")
	}

	switch level {
	case LevelLogin:
		return allow(level)
	case LevelFollower:
		if social.IsFollowing(a.UserID, ct.AuthorID) {
			return allow(level)
		}
		return deny(level, ReasonFollowerOnly, "该内容仅作者粉丝可见，请先关注作者")
	case LevelVip:
		if cfg.VipMinLevel <= 0 || VipLevelOf(a.UserID) >= cfg.VipMinLevel {
			return allow(level)
		}
		return deny(level, ReasonVipRequired, "该内容需要达到站长设定的会员等级")
	case LevelPaid:
		if HasPurchased(a.UserID, ct.Type, ct.ID) {
			return allow(level)
		}
		return deny(level, ReasonPaidRequired, "该内容为付费内容，购买后即可阅读")
	case LevelPrivate:
		return deny(level, ReasonPrivate, "该内容仅作者本人可见")
	default:
		return allow(level)
	}
}
