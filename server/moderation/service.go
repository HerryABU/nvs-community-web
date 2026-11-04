package moderation

import (
	"nvs-server/models"

	"gorm.io/gorm"
)

// Migrate 建表（main 里 AutoMigrateAll 之后调用，收 *gorm.DB）。
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&ModerationRecord{}, &ModerationConfig{}, &MaliciousReporter{}, &HiddenContent{})
}

// Init 初始化：确保配置行存在并启动定时批量审核调度器。
// 调度器失败不应阻断主流程。
func Init() {
	if _, err := GetConfig(); err != nil {
		// 配置读取失败仅记录，不影响启动
		_ = err
	}
	StartScheduler()
}

// ReviewOnWrite 发布/提交时送检（小说/博客）。
// 由 chapter / blog / novel(简介) / forum(版块) 的写入 handler 在落库后调用；
// 具体闸门见 onWriteEnabled。
func ReviewOnWrite(scope string, targetID uint) {
	cfg, err := GetConfig()
	if err != nil || !cfg.Enabled {
		return
	}
	// 按 scope 选对应的开关：小说/博客/章节走「发布即送检」，
	// 版块走「论坛 AI 审核」那个开关（版块名/简介与论坛内容同属论坛维度）。
	// 早先只有 NovelBlogOnWrite 一个闸门，导致版块创建永远不可能送检。
	if !onWriteEnabled(cfg, scope) {
		return
	}
	content, ok := ResolveContent(scope, targetID)
	if !ok {
		return
	}
	res, e := ReviewText(scope, content, false, 0)
	if e != nil {
		return
	}
	_ = CreateRecord(&ModerationRecord{
		Scope: scope, TargetType: scope, TargetID: targetID, Trigger: TriggerWrite,
		ModelID: res.ModelID, ModelName: res.ModelName, Verdict: res.Verdict,
		Compliant: res.Verdict != VerdictFail, Reason: res.Reason, Categories: joinJSON(res.Categories),
	})
	// 同一次 AI 调用顺带产出的话题标注：评论区守卫要用它决定「拦还是只标注」
	persistTopic(scope, targetID, res)
	// 判定违规且站长开启了「自动处置」→ 交给注入的处置回调（软删留证）
	if res.Verdict == VerdictFail && cfg.AutoHideOnFail {
		runFailHook(scope, targetID, res.Reason)
	}
}

// onWriteEnabled 该 scope 的「写入即送检」是否开启。
func onWriteEnabled(cfg *ModerationConfig, scope string) bool {
	switch scope {
	case ScopeForum:
		// 版块属于论坛维度：沿用「论坛/评论区 AI 审核」开关
		return cfg.ForumCommentBatch
	case ScopeNovel, ScopeBlog, ScopeChapter:
		return cfg.NovelBlogOnWrite
	default:
		return cfg.NovelBlogOnWrite
	}
}

// ── 违规自动处置回调 ────────────────────────────────────────────────// 本包不依赖 handlers（避免反向 import），由 main 启动时用 SetOnFailHook 注入
// 「软删 + 留证」的实现。未注入时自动处置静默跳过（只记录，不处置）。
var failHook func(scope string, targetID uint, reason string)

// SetOnFailHook 注册违规自动处置回调（main 启动时调用一次）。
func SetOnFailHook(fn func(scope string, targetID uint, reason string)) {
	failHook = fn
}

// runFailHook 触发自动处置；无回调时安全跳过。
func runFailHook(scope string, targetID uint, reason string) {
	if failHook == nil {
		return
	}
	defer func() {
		// 处置失败不能影响审核主链路
		_ = recover()
	}()
	failHook(scope, targetID, reason)
}

// IsHidden 判断某内容是否对指定用户隐藏（恶意举报者可见性控制）。
func IsHidden(userID uint, targetType string, targetID uint) bool {
	if userID == 0 {
		return false
	}
	var n int64
	models.DB.Model(&HiddenContent{}).
		Where("user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID).
		Count(&n)
	return n > 0
}
